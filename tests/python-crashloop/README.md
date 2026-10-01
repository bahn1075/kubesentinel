# 실제 Python Pod CrashLoopBackOff 테스트

공개 이미지 `python:3.12-slim`으로 Python 구문 오류를 발생시켜 실제 Pod 재시작을 재현한다. Prometheus → Alertmanager → KubeSentinel 경로를 통해 자동 진단이 생성되고, 로그에 나온 오류와 수정 방법을 정확히 안내하는지 확인한다. 이미지 빌드와 가짜 알림 API 전송은 필요하지 않다.

## 준비 조건

- `kubectl`이 설치돼 있고 minikube가 실행 중이다.
- KubeSentinel 백엔드와 프런트엔드가 `kubesentinel` 네임스페이스에 실행 중이다.
- kube-prometheus-stack(Prometheus, Alertmanager, kube-state-metrics)과 Loki/Alloy가 실행 중이다.
- KubeSentinel Settings에 관측 도구와 LLM 연결이 설정돼 있다.

이 가이드의 서비스 주소는 앞서 검증한 로컬 설치 기준이다. 설치 이름이 다르면 Settings에서 해당 클러스터의 주소를 사용한다.

| Settings 항목 | 검증한 주소 |
| --- | --- |
| Prometheus | `http://prometheus-operated.monitoring.svc:9090` |
| Alertmanager | `http://alertmanager-operated.monitoring.svc:9093` |
| Loki | `http://loki-gateway.monitoring.svc:80` |
| 로컬 LLM | `http://host.minikube.internal:1234/v1` |

LLM은 모델이 실제 로드된 상태여야 한다. Loki/Alloy는 테스트 네임스페이스의 로그를 `namespace`, `pod` 라벨로 수집해야 한다. 백엔드의 시작 로그에서 `Alertmanager polling enabled`를 확인한다. 현재 구현의 기본 폴링 간격은 30초다.

## 파일 구성

| 파일 | 역할 |
| --- | --- |
| `namespace.yaml` | 테스트 전용 네임스페이스 |
| `deployment-broken.yaml` | 함수 정의의 콜론을 누락해 SyntaxError를 발생시키는 Deployment |
| `prometheus-rule-fast.yaml` | 선택 사항: 재시작 3회 이상 상태를 1분 관측해 알림을 발생시키는 테스트 규칙 |
| `deployment-fixed.yaml` | 콜론을 추가하고 프로세스를 유지하는 복구용 Deployment |

## 1. 컨텍스트와 기존 리소스 확인

저장소 루트에서 실행한다. 아래의 모든 클러스터 명령은 `--context=minikube`로 대상을 명시한다.

```bash
cd /Users/cozy/app/kubesentinel
kubectl config current-context
kubectl --context=minikube get nodes
kubectl --context=minikube -n kubesentinel get pods
kubectl --context=minikube -n monitoring get pods
kubectl --context=minikube get namespace kubesentinel-crashloop-test --ignore-not-found
```

이전 실행의 테스트 네임스페이스가 남아 있으면 먼저 7단계의 정리를 수행한다. 네임스페이스가 완전히 삭제된 뒤 새로 시작하면 재시작 횟수와 Pod 이름을 명확하게 구분할 수 있다.

## 2. Python 오류 배포

```bash
kubectl --context=minikube apply -f tests/python-crashloop/namespace.yaml
kubectl --context=minikube apply -f tests/python-crashloop/deployment-broken.yaml
kubectl --context=minikube -n kubesentinel-crashloop-test get pods -w
```

이미지 다운로드가 끝나면 컨테이너가 종료 코드 1로 반복 종료한다. `Error`와 `CrashLoopBackOff`가 번갈아 나타날 수 있다. 재시작 횟수가 증가하고 `BackOff` 이벤트가 발생하는지 확인한다. `Ctrl+C`는 관찰만 종료하며 테스트 Pod는 계속 실행된다.

## 3. 직전 로그와 종료 상태 확인

```bash
TEST_POD=$(kubectl --context=minikube -n kubesentinel-crashloop-test get pods \
  -l app=python-syntax-crash -o jsonpath='{.items[0].metadata.name}')

kubectl --context=minikube -n kubesentinel-crashloop-test logs "$TEST_POD" \
  -c python --previous --tail=20
kubectl --context=minikube -n kubesentinel-crashloop-test describe pod "$TEST_POD"
```

기대 로그:

```text
  File "<string>", line 1
    def calculate_total(price)
                              ^
SyntaxError: expected ':'
```

`--previous`는 직전에 종료한 컨테이너의 로그를 조회한다. 첫 종료 직후에는 아직 이전 컨테이너가 없어 조회가 실패할 수 있으므로 재시작 후 다시 실행한다. `describe`에서 `Exit Code: 1`, `Reason: Error`, `Back-off restarting failed container`를 확인한다.

## 4. 실제 모니터링 알림 발생

다음 두 방법 중 하나를 선택한다.

### 기본 규칙으로 기다리기

기존 `KubePodCrashLooping` 규칙을 사용한다. 검증 당시 기본 규칙은 `CrashLoopBackOff` waiting reason을 최근 5분 동안 관측하고, 조건이 15분 지속돼야 firing 상태가 됐다. 설치된 규칙과 `for` 값은 다음 명령으로 확인한다.

```bash
kubectl --context=minikube -n monitoring get prometheusrule \
  monitoring-stack-kube-prom-kubernetes-apps -o yaml
```

기본 대기 시간, 메트릭 스크레이프, 규칙 평가, Alertmanager 전달 및 KubeSentinel 폴링 때문에 Pod 생성 즉시 진단이 나오지는 않는다.

### 테스트 규칙으로 빠르게 확인하기

```bash
kubectl --context=minikube apply -f tests/python-crashloop/prometheus-rule-fast.yaml
```

이 규칙은 실제 kube-state-metrics의 재시작 횟수를 사용한다. Pod 상태가 잠시 `Error`로 표시되는 동안에도 재시작을 감지할 수 있다. 재시작 3회 이상이 1분 지속되면 알림을 발생시킨다. 규칙 반영과 전달 시간이 추가로 필요하다.

`release: monitoring-stack` 라벨은 검증 환경의 Prometheus ruleSelector에 맞춘 값이다. 규칙이 로드되지 않으면 아래 selector를 확인하고 테스트 규칙의 라벨을 맞춘다.

```bash
kubectl --context=minikube -n monitoring get prometheus \
  monitoring-stack-kube-prom-prometheus \
  -o jsonpath='{.spec.ruleSelector}{"\n"}'
```

테스트 규칙은 재시작 누적 횟수를 사용하므로 복구를 판단하는 용도로 쓰지 않는다. 진단 확인 후 반드시 제거한다. 기본 규칙이 나중에 발화하면 같은 이름의 CrashLoop 알림이 추가 분석될 수 있다.

## 5. KubeSentinel 진단 확인

```bash
kubectl --context=minikube -n kubesentinel logs \
  deployment/kubesentinel-kubesentinel-ai --tail=100 -f
```

`Analyzing Incident: ...KubePodCrashLooping`, 필요 시 `tool: k8s_logs`, 마지막으로 `Analysis Complete! Root Cause:`를 확인한다. 로컬 LLM은 도구 조사와 최종 검증을 위해 여러 요청을 수행하므로 수 분 이상 걸릴 수 있다.

프런트엔드를 로컬로 연결하려면 다른 터미널에서 실행한다.

```bash
kubectl --context=minikube -n kubesentinel port-forward \
  svc/kubesentinel-kubesentinel-ai-frontend 18080:80
```

브라우저에서 <http://127.0.0.1:18080>의 Incidents를 열고, 네임스페이스 `kubesentinel-crashloop-test`와 현재 Pod 이름으로 해당 인시던트를 확인한다. 날짜가 포함된 인시던트 ID를 고정해서 사용하지 않는다.

알림을 감지하면 `IncidentDetected`, 근거 수집 후에는 `EvidenceCollected` 상태로 먼저 표시된다. AI 대기열에 있거나 분석 중이어도 목록에서 확인할 수 있다. 목록과 상세 화면은 5초마다 자동 갱신되며, 진단이 완료되면 `DiagnosisCompleted`로 변경된다. 이 동작은 2026-10-01 수정 이후 이미지에 포함된다.

합격 기준:

- 상태가 `DiagnosisCompleted`이고 대상 Pod가 테스트 Pod다.
- 수집 근거에 Python 구문 오류 로그, 재시작 횟수, 종료 코드 1 또는 BackOff 이벤트가 있다.
- Root Cause가 `SyntaxError: expected ':'`와 `def calculate_total(price)`의 콜론 누락을 지목한다.
- 제안 조치가 `def calculate_total(price):`로 수정하라고 안내한다.
- 이미지 pull 실패나 OOM을 근본 원인으로 잘못 지목하지 않는다.

2026-09-30 실제 검증에서는 위 조건을 충족했고 신뢰도 0.97의 진단이 저장됐다. 신뢰도와 자연어 문구는 모델과 실행마다 달라질 수 있다.

## 6. 복구 확인 (선택)

빠른 테스트 규칙을 사용했다면 먼저 제거한다.

```bash
kubectl --context=minikube delete -f tests/python-crashloop/prometheus-rule-fast.yaml \
  --ignore-not-found
kubectl --context=minikube apply -f tests/python-crashloop/deployment-fixed.yaml
kubectl --context=minikube -n kubesentinel-crashloop-test rollout status \
  deployment/python-syntax-crash --timeout=120s
kubectl --context=minikube -n kubesentinel-crashloop-test get pods
kubectl --context=minikube -n kubesentinel-crashloop-test logs \
  deployment/python-syntax-crash -c python --tail=20
```

새 Pod가 `1/1 Running`이 되고 로그에 `90.0`이 출력되면 복구된 것이다. 수정 파일은 정상 계산 뒤 `Event().wait()`로 프로세스를 유지한다. 계산 후 바로 종료하면 `restartPolicy: Always`인 Deployment에서 다시 재시작되기 때문이다.

기본 알림의 시간 범위 때문에 복구 후에도 알림이 잠시 남을 수 있다. 이미 저장된 KubeSentinel 진단은 복구 뒤에도 과거 기록으로 남는다.

## 7. 테스트 종료 및 정리

```bash
kubectl --context=minikube delete -f tests/python-crashloop/prometheus-rule-fast.yaml \
  --ignore-not-found
kubectl --context=minikube delete -f tests/python-crashloop/namespace.yaml \
  --ignore-not-found --wait=true
```

열어 둔 로그 관찰과 port-forward 터미널도 `Ctrl+C`로 종료한다. 위 명령은 이 시나리오의 테스트 규칙과 테스트 네임스페이스를 삭제한다.

실패 Pod를 남겨 두면 `KubePodNotReady`, `KubeDeploymentReplicasMismatch` 같은 연관 알림과 재발화 알림이 추가 분석될 수 있다. 그래서 첫 진단이 표시된 뒤에도 LM Studio 추론이 이어질 수 있다. 테스트를 마치면 정리하고, 이미 대기하거나 실행 중인 분석은 별도로 완료될 수 있음을 고려한다.

## 결과가 나오지 않을 때

| 증상 | 확인할 사항 |
| --- | --- |
| `ImagePullBackOff` | 공개 이미지 접근, DNS, 레지스트리 제한 및 노드 네트워크 |
| Pod가 `Error`로 보임 | 재시작 횟수와 BackOff 이벤트 확인; waiting reason은 일시적으로 보이지 않을 수 있음 |
| Prometheus 알림이 없음 | kube-state-metrics 수집, ruleSelector 라벨, 기본 규칙의 15분 대기 |
| KubeSentinel 분석 시작 로그가 없음 | Settings의 Alertmanager URL, 백엔드 시작 로그의 polling 활성화, 무시 규칙에 테스트 네임스페이스가 포함됐는지 확인 |
| 수집 로그가 비어 있음 | Loki URL, Alloy의 네임스페이스 수집 범위, `namespace`/`pod` 라벨, backend의 Pod 로그 조회 권한 |
| `ValidationFailed` 또는 긴 대기 | LM Studio의 모델 로드 상태와 API 접근, 다른 분석의 대기열; 현재 HTTP 요청 제한 시간은 5분이고 인시던트 하나가 여러 요청을 수행할 수 있음 |
