# KubeSentinel AI — Helm chart (로컬 minikube 테스트)

로컬 minikube에서 차트를 배포해 개발·검증하는 방법입니다.
CSP/OKE 배포와 프로젝트 전반은 [루트 README](../../README.md)를 참고하세요.

배포되는 워크로드는 백엔드 · 프론트엔드 · Postgres 3개입니다.

---

## 사전 준비

```bash
minikube start
kubectl config current-context     # → minikube
```

필요한 클러스터 기능 두 가지를 확인합니다. `minikube addons list`가 `disabled`로
보여도 **파드가 Running이면 동작**하므로 아래 결과를 신뢰하세요.

```bash
kubectl get storageclass                        # default 없으면 Postgres PVC가 Pending
kubectl -n metallb-system get pods              # 없으면 port-forward로 접속
```

없을 때: `minikube addons enable default-storageclass storage-provisioner metallb`

---

## 설치

`values/metallb.yaml`이 minikube용 오버레이입니다. LLM을 어디에 붙일지에 따라 둘 중 하나를 고릅니다.

### A. mock LLM (외부 의존 없음)

mock LLM과 알림 sink를 먼저 배포합니다. 이 둘은 차트에 없는 **별도 매니페스트**입니다.

```bash
cd "$(git rev-parse --show-toplevel)"

kubectl create namespace kubesentinel --dry-run=client -o yaml | kubectl apply -f -
kubectl -n kubesentinel create configmap mock-llm-script \
  --from-file=mock-llm.py=deploy/local/mock-llm.py --dry-run=client -o yaml | kubectl apply -f -
kubectl -n kubesentinel create configmap notify-sink-script \
  --from-file=notify-sink.py=deploy/local/notify-sink.py --dry-run=client -o yaml | kubectl apply -f -
kubectl -n kubesentinel apply -f deploy/local/minikube-mock-stack.yaml

helm upgrade --install kubesentinel helm/kubesentinel-ai \
  -n kubesentinel --create-namespace \
  -f helm/kubesentinel-ai/values.yaml -f helm/kubesentinel-ai/values/metallb.yaml \
  --set ai.endpoint=http://mock-llm:8080/v1 --set ai.model=mock-model \
  --wait --timeout 5m

kubectl -n kubesentinel logs -l app=notify-sink -f   # 알림 확인
```

### B. 실제 LLM (Mac의 LM Studio 등)

```bash
cd "$(git rev-parse --show-toplevel)"

helm upgrade --install kubesentinel helm/kubesentinel-ai \
  -n kubesentinel --create-namespace \
  -f helm/kubesentinel-ai/values.yaml -f helm/kubesentinel-ai/values/metallb.yaml \
  --set ai.endpoint=http://host.minikube.internal:1234/v1 \
  --set secret.notifierWebhook= \
  --wait --timeout 5m
```

- `ai.endpoint`는 **비울 수 없습니다.** 미설정 시 config 검증에 실패해 파드가 기동되지 않습니다.
  파드 안에서 `localhost`는 파드 자신이므로 호스트를 가리키는 `host.minikube.internal`을 씁니다.
- `secret.notifierWebhook=`(빈 값)은 오버레이 기본값인 `notify-sink`(모드 A 전용)를 지워
  알림 전송 실패 경고를 없앱니다.
- 모델명은 대시보드 Settings에서 지정하세요. `ai.model`은 기동 검증용 placeholder이고
  DB에 저장된 값이 우선합니다. 저장 후 파드를 재생성해야 반영됩니다.

---

## 대시보드 접속

```bash
kubectl -n kubesentinel get svc kubesentinel-kubesentinel-ai-frontend \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

노드 대역이 호스트에서 라우팅되면(`ping $(minikube ip)` 응답) 위 IP로 바로 접속됩니다.
안 되면 `minikube tunnel` 또는:

```bash
kubectl -n kubesentinel port-forward svc/kubesentinel-kubesentinel-ai-frontend 8081:80
```

---

## 스모크 테스트

alert를 주입해 수집 → RCA → 알림 경로를 확인합니다. **현재 컨텍스트로 가므로 대상 클러스터를
먼저 확인하세요.**

```bash
kubectl config current-context                  # → minikube

lsof -nP -iTCP:8080 -sTCP:LISTEN                # 점유 중이면 정리하거나 포트 변경
LOCAL_PORT=8080
kubectl -n kubesentinel port-forward svc/kubesentinel-kubesentinel-ai $LOCAL_PORT:8080 &

curl -X POST localhost:$LOCAL_PORT/v1/alerts \
  -H 'Content-Type: application/json' \
  -d '{"receiver":"kubesentinel","status":"firing","alerts":[{"status":"firing",
       "labels":{"alertname":"OOMKilled","namespace":"production","pod":"demo-7d9f-abcde",
       "deployment":"demo","severity":"critical"},
       "annotations":{"summary":"container in pod demo-7d9f-abcde was OOMKilled"}}]}'

kubectl -n kubesentinel logs -l app.kubernetes.io/name=kubesentinel-ai -f
lsof -ti:$LOCAL_PORT -sTCP:LISTEN | xargs -r kill   # 끝나면 정리
```

`🔍 Analyzing Incident` → `✅ Analysis Complete`가 보이면 정상입니다.

### 케이스별 테스트

서로 다른 룰 분류 경로를 타는 alert 9종을 **하나씩 골라** 주입합니다.

```bash
cd "$(git rev-parse --show-toplevel)"
./deploy/local/smoke-alerts.sh        # 목록에서 번호 선택
./deploy/local/smoke-alerts.sh 3      # 3번만
./deploy/local/smoke-alerts.sh OOMKilled
./deploy/local/smoke-alerts.sh -l     # 목록만
```

> **한 번에 하나씩 주입하세요.** 여러 건을 연달아 넣으면 LLM 호출이 동시에 쌓여
> 로컬 모델(LM Studio 등)이 처리하지 못합니다. 앞 건이 `✅ Analysis Complete`로
> 끝난 것을 로그에서 확인한 뒤 다음 건을 넣으세요.

| alertname | 분류 | 심층 프로브 |
|---|---|---|
| `OOMKilled` | OOMKilled | O |
| `KubePodCrashLooping` | CrashLoopBackOff | O |
| `ImagePullBackOff` | ImagePullBackOff | O |
| `FailedScheduling` | Unschedulable | O |
| `CreateContainerConfigError` | ConfigError | - |
| `KubeJobFailed` | JobFailed | - |
| `TargetDown` | TargetDown | - |
| `KubeNodeNotReady` | NodeIssue | - |
| `KubePersistentVolumeFillingUp` | Unknown | - |

마지막 케이스는 룰에 걸리지 않는 경로(분류 `Unknown`)를 확인용으로 넣었습니다.
이때 상세 화면은 AI 진단 대신 **권장 조치 + AI 재분석 버튼**을 표시합니다.

단건으로 저장소 샘플 파일을 쓰려면 경로가 **루트 기준**입니다.

```bash
cd "$(git rev-parse --show-toplevel)"
curl -X POST localhost:$LOCAL_PORT/v1/alerts -H 'Content-Type: application/json' \
  --data @deploy/local/sample-alert.json
```

---

## 코드 수정분 테스트

오버레이는 Docker Hub 배포본을 당겨오므로, 작업 중인 코드는 이미지를 직접 밀어넣습니다.

> containerd 런타임에서는 `eval $(minikube docker-env)`가 동작하지 않습니다. `image load`를 쓰세요.

```bash
cd "$(git rev-parse --show-toplevel)"
ARCH=$(kubectl get node -o jsonpath='{.items[0].status.nodeInfo.architecture}')

docker build --platform linux/$ARCH -t kubesentinel-ai:dev .
docker build --platform linux/$ARCH -t kubesentinel-ai-front:dev ./frontend
minikube image load kubesentinel-ai:dev
minikube image load kubesentinel-ai-front:dev

helm upgrade --install kubesentinel helm/kubesentinel-ai \
  -n kubesentinel --create-namespace \
  -f helm/kubesentinel-ai/values.yaml -f helm/kubesentinel-ai/values/metallb.yaml \
  --set ai.endpoint=http://host.minikube.internal:1234/v1 --set secret.notifierWebhook= \
  --set image.repository=kubesentinel-ai --set image.tag=dev --set image.pullPolicy=Never \
  --set frontend.image.repository=kubesentinel-ai-front --set frontend.image.tag=dev \
  --set frontend.image.pullPolicy=Never \
  --wait --timeout 5m
```

`pullPolicy=Never`가 핵심입니다. 오버레이 기본값이 `Always`라 그대로 두면 로컬 이미지를
무시하고 레지스트리를 조회해 `ErrImagePull`이 납니다.

**같은 태그로 다시 빌드·로드했다면** 파드를 재생성해야 반영됩니다.

```bash
# Postgres는 건드리지 않도록 앱 파드만 재생성한다(DB 재시작 방지).
kubectl -n kubesentinel delete pod \
  -l 'app.kubernetes.io/name in (kubesentinel-ai,kubesentinel-ai-frontend)'
```

---

## 트러블슈팅

| 증상 | 원인 | 해결 |
|---|---|---|
| 백엔드 `CrashLoopBackOff`, `ai.endpoint must be specified` | endpoint 미설정 | `--set ai.endpoint=...` |
| 백엔드가 `waiting for database...` 반복 | Postgres 미기동 | `kubectl -n kubesentinel get pvc` → `Pending`이면 StorageClass 확인 |
| PVC가 계속 `Pending` | default StorageClass 없음 | `minikube addons enable default-storageclass storage-provisioner` |
| Postgres `mkdir: ... /pgdata: Permission denied` | 다중 노드 + hostPath PV (프로비저너 노드에만 권한 있는 디렉토리 생성) | `values/metallb.yaml`이 컨트롤플레인에 고정해 해결됨. 이 오버레이를 안 쓰면 `postgres.nodeSelector` 직접 지정 |
| `ErrImagePull` (로컬 이미지) | `pullPolicy=Always` | `--set image.pullPolicy=Never` + `minikube image load` |
| `exec format error` | 아키텍처 불일치 | `--platform linux/$ARCH`로 재빌드 |
| LoadBalancer IP가 `<pending>` | metallb 미설치 | 애드온 활성화 또는 port-forward |
| LLM 호출 타임아웃 | endpoint가 `localhost` | `host.minikube.internal`로 변경 |
| 알림 전송 실패 경고 | `notify-sink` 미배포 | 모드 A 스택 배포 또는 `--set secret.notifierWebhook=` |
| Settings의 "Pod 재시작"이 503 | `rbac.selfRestart` 기본 off | `--set rbac.selfRestart.enabled=true` |
| port-forward `bind: address already in use` | 로컬 포트 점유 | `lsof -ti:8080 -sTCP:LISTEN \| xargs -r kill` 또는 포트 변경 |
| `curl: Failed to open deploy/local/...` | 샘플 경로가 루트 기준 | `cd "$(git rev-parse --show-toplevel)"` |
| 엉뚱한 클러스터에 적용됨 | kubectl 컨텍스트 | `kubectl config current-context` 확인 후 전환 |
| Grafana에서 Loki가 `Unable to connect with Loki` | Loki 2.9 미만은 Grafana의 health check 쿼리(`vector()`)를 파싱 못 함 | 현행 `grafana/loki` 차트(Loki 3.x)로 재설치. deprecated `loki-stack`은 2.6.1을 설치하니 사용 금지 |

```bash
kubectl -n kubesentinel get pods,svc,pvc
kubectl -n kubesentinel logs -l app.kubernetes.io/name=kubesentinel-ai --tail=50
```

---

## 정리

```bash
helm uninstall kubesentinel -n kubesentinel
kubectl -n kubesentinel delete pvc --all              # 설정·인시던트까지 초기화
kubectl -n kubesentinel delete -f deploy/local/minikube-mock-stack.yaml   # 모드 A 사용 시
kubectl delete namespace kubesentinel
```

PVC를 남기면 재배포 시 이전 설정·인시던트가 복원됩니다.
