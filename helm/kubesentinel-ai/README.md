# KubeSentinel AI — Helm chart

로컬 **minikube**에서 차트를 배포해 개발·검증하는 방법을 정리한 문서입니다.
CSP/OKE 배포와 프로젝트 전반은 [루트 README](../../README.md)를 참고하세요.

> 이 차트는 백엔드 · 프론트엔드 · Postgres 3개 워크로드를 배포합니다.
> 모든 서비스 주소·모델명·repo는 코드가 아닌 values로 주입합니다 (architecture.md §2).

---

## 목차

- [사전 준비](#사전-준비)
- [두 가지 테스트 모드](#두-가지-테스트-모드)
- [모드 A: mock 스택 (오프라인)](#모드-a-mock-스택-오프라인)
- [모드 B: 실제 LLM 연결](#모드-b-실제-llm-연결)
- [대시보드 접속](#대시보드-접속)
- [스모크 테스트](#스모크-테스트)
- [로컬 코드로 이미지 교체](#로컬-코드로-이미지-교체)
- [values 오버레이](#values-오버레이)
- [트러블슈팅](#트러블슈팅)
- [정리](#정리)

---

## 사전 준비

```bash
minikube start                   # 이미 떠 있으면 생략
kubectl config current-context   # → minikube 확인
```

> **다중 노드(`--nodes 2` 이상)로 기동한 경우**
> `minikube-hostpath` 프로비저너는 컨트롤플레인의 static pod라 그 노드에만 올바른 권한의
> PV 디렉토리를 만듭니다. postgres가 워커로 스케줄되면 PGDATA 생성이 실패하므로
> `values/metallb.yaml`이 postgres를 컨트롤플레인에 고정합니다(`postgres.nodeSelector`).
> 이 오버레이를 쓰지 않는다면 직접 지정하세요.
> 자세한 내용은 [트러블슈팅](#트러블슈팅)의 `Permission denied` 항목을 참고하세요.

차트가 요구하는 클러스터 기능은 두 가지입니다.

| 필요 기능 | 확인 명령 | 없을 때 |
|---|---|---|
| default StorageClass (Postgres PVC) | `kubectl get storageclass` | `minikube addons enable storage-provisioner default-storageclass` |
| LoadBalancer (`expose.mode=metallb`) | `kubectl -n metallb-system get pods` | `minikube addons enable metallb` <br> 또는 port-forward로 우회 |

`minikube addons list`가 `disabled`로 보여도 파드가 Running이면 실제로는 동작합니다.
애드온 메타데이터와 실제 상태가 어긋나는 경우가 있으니 **위 확인 명령의 결과를 신뢰**하세요.

metallb를 새로 켰다면 할당 대역을 확인해 둡니다. minikube 노드와 같은 서브넷이어야 합니다.

```bash
minikube ip                                                    # 예: 192.168.105.6
kubectl -n metallb-system get cm config -o jsonpath='{.data.config}'
```

---

## 두 가지 테스트 모드

| | 모드 A (mock) | 모드 B (실제 LLM) |
|---|---|---|
| LLM | 클러스터 내 `mock-llm` (고정 RCA JSON) | LM Studio/Ollama 등 Mac 호스트 |
| 알림 | 클러스터 내 `notify-sink` (로그 출력) | 비활성(noop) 또는 실제 webhook |
| 외부 의존 | **없음** | Mac에서 LLM 서버 기동 필요 |
| 용도 | 파이프라인 배선 검증, CI | 실제 진단 품질 확인 |

`values/metallb.yaml` 오버레이는 **모드 A를 전제**로 작성돼 있습니다
(`notifierWebhook: http://notify-sink:8080/`). 모드 B에서는 아래처럼 값을 덮어씁니다.

---

## 모드 A: mock 스택 (오프라인)

mock LLM과 알림 sink를 먼저 배포합니다. 이 둘은 차트에 포함되지 않은 **별도 매니페스트**입니다.

```bash
cd /path/to/kubesentinel

kubectl create namespace kubesentinel --dry-run=client -o yaml | kubectl apply -f -

# python 스크립트를 ConfigMap으로 주입
kubectl -n kubesentinel create configmap mock-llm-script \
  --from-file=mock-llm.py=deploy/local/mock-llm.py --dry-run=client -o yaml | kubectl apply -f -
kubectl -n kubesentinel create configmap notify-sink-script \
  --from-file=notify-sink.py=deploy/local/notify-sink.py --dry-run=client -o yaml | kubectl apply -f -

kubectl -n kubesentinel apply -f deploy/local/minikube-mock-stack.yaml
```

차트를 배포합니다.

```bash
helm upgrade --install kubesentinel helm/kubesentinel-ai \
  --namespace kubesentinel --create-namespace \
  -f helm/kubesentinel-ai/values.yaml \
  -f helm/kubesentinel-ai/values/metallb.yaml \
  --set ai.endpoint=http://mock-llm:8080/v1 \
  --set ai.model=mock-model \
  --wait --timeout 5m
```

알림이 도착하는지 확인합니다.

```bash
kubectl -n kubesentinel logs -l app=notify-sink -f
```

---

## 모드 B: 실제 LLM 연결

Mac에서 LM Studio(기본 1234 포트)를 띄운 뒤, 파드에서 호스트를 가리키게 합니다.

```bash
helm upgrade --install kubesentinel helm/kubesentinel-ai \
  --namespace kubesentinel --create-namespace \
  -f helm/kubesentinel-ai/values.yaml \
  -f helm/kubesentinel-ai/values/metallb.yaml \
  --set ai.endpoint=http://host.minikube.internal:1234/v1 \
  --set secret.notifierWebhook= \
  --wait --timeout 5m
```

**`--set` 두 개가 필요한 이유:**

1. **`ai.endpoint`** — 오버레이 기본값은 `http://localhost:1234/v1`인데, 파드 안에서 `localhost`는
   *파드 자신*이라 호스트의 LLM에 닿지 않습니다. minikube가 CoreDNS에 등록해 둔
   `host.minikube.internal`이 Mac을 가리킵니다.

   ```bash
   kubectl -n kube-system get cm coredns -o jsonpath='{.data.Corefile}' | grep -A2 'hosts {'
   ```

   이 값은 **비워둘 수 없습니다.** openai-compatible provider는 endpoint가 필수라
   미설정 시 config 검증에 실패해 파드가 기동되지 않습니다
   ([config.go](../../internal/config/config.go) `Validate`).

2. **`secret.notifierWebhook`** — 빈 값으로 덮지 않으면 존재하지 않는 `notify-sink`로
   전송을 시도해 로그에 경고가 쌓입니다. 알림은 best-effort라 RCA 흐름 자체는 진행됩니다.

모델명은 UI(Settings)에서 지정하는 것을 권합니다. `ai.model`은 기동 검증용 placeholder이고,
실제 값은 DB에 저장된 설정이 우선합니다.

---

## 대시보드 접속

`expose.mode=metallb`이면 프론트엔드 Service가 LoadBalancer로 뜹니다.

```bash
kubectl -n kubesentinel get svc kubesentinel-kubesentinel-ai-frontend \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

minikube 노드 대역이 호스트에서 라우팅되는 드라이버(krunkit/vfkit 등)라면
`minikube tunnel` 없이 위 IP로 바로 접속됩니다. 먼저 확인해 보세요.

```bash
ping -c1 $(minikube ip)     # 응답하면 tunnel 불필요
```

`<pending>`에서 멈추거나 IP에 닿지 않으면 두 가지 우회로가 있습니다.

```bash
# 1) tunnel (별도 터미널 유지, sudo 요구)
minikube tunnel

# 2) port-forward — 가장 확실
kubectl -n kubesentinel port-forward svc/kubesentinel-kubesentinel-ai-frontend 8081:80
#  → http://localhost:8081
```

---

## 스모크 테스트

alert를 직접 주입해 수집 → RCA → 알림 전체 경로를 확인합니다.

```bash
kubectl -n kubesentinel port-forward svc/kubesentinel-kubesentinel-ai 8080:8080 &

curl -X POST localhost:8080/v1/alerts \
  -H 'Content-Type: application/json' \
  --data @deploy/local/sample-alert.json

kubectl -n kubesentinel logs -l app.kubernetes.io/name=kubesentinel-ai -f
```

`🔍 Analyzing Incident` → `✅ Analysis Complete`가 보이면 정상입니다.
대시보드 Incidents 화면에도 새 인시던트가 나타납니다.

Alertmanager를 실제로 연결하려면 receiver 대상은 다음 주소입니다.

```
http://kubesentinel-kubesentinel-ai.kubesentinel.svc:8080/v1/alerts
```

관측 스택이 클러스터에 있으면 수집 소스도 주입할 수 있습니다(미설정 시 자동 skip).

```bash
  --set collector.prometheusUrl=http://prometheus-operated.monitoring.svc:9090 \
  --set collector.lokiUrl=http://loki-gateway.monitoring.svc:80
```

`collector.alertmanagerUrl`을 설정하면 webhook 대신 **폴링(pull) 모드**가 켜져
Alertmanager 설정을 건드리지 않고도 alert를 가져옵니다.

---

## 로컬 코드로 이미지 교체

오버레이는 Docker Hub의 배포본을 당겨오므로, 작업 중인 코드를 테스트하려면
이미지를 minikube에 직접 밀어넣습니다.

> **containerd 런타임에서는 `eval $(minikube docker-env)`가 동작하지 않습니다.**
> `minikube image load`를 사용하세요. 런타임 확인:
> `kubectl get nodes -o custom-columns=NAME:.metadata.name,RUNTIME:.status.nodeInfo.containerRuntimeVersion`

노드 아키텍처에 맞춰 빌드해야 합니다(Apple Silicon이면 `arm64`).

```bash
ARCH=$(kubectl get node -o jsonpath='{.items[0].status.nodeInfo.architecture}')

docker build --platform linux/$ARCH -t kubesentinel-ai:dev .
docker build --platform linux/$ARCH -t kubesentinel-ai-front:dev ./frontend

minikube image load kubesentinel-ai:dev
minikube image load kubesentinel-ai-front:dev
```

```bash
helm upgrade --install kubesentinel helm/kubesentinel-ai \
  --namespace kubesentinel --create-namespace \
  -f helm/kubesentinel-ai/values.yaml \
  -f helm/kubesentinel-ai/values/metallb.yaml \
  --set ai.endpoint=http://host.minikube.internal:1234/v1 \
  --set secret.notifierWebhook= \
  --set image.repository=kubesentinel-ai \
  --set image.tag=dev \
  --set image.pullPolicy=Never \
  --set frontend.image.repository=kubesentinel-ai-front \
  --set frontend.image.tag=dev \
  --set frontend.image.pullPolicy=Never \
  --wait --timeout 5m
```

**`pullPolicy=Never`가 핵심입니다.** 오버레이 기본값이 `Always`라 그대로 두면
로컬 이미지를 무시하고 레지스트리를 조회해 `ErrImagePull`이 납니다.

같은 태그로 다시 빌드·로드한 경우에는 파드를 재생성해야 반영됩니다.

```bash
kubectl -n kubesentinel rollout restart deploy/kubesentinel-kubesentinel-ai
kubectl -n kubesentinel rollout restart deploy/kubesentinel-kubesentinel-ai-frontend
```

---

## values 오버레이

| 파일 | 환경 | 노출 방식 |
|---|---|---|
| `values.yaml` | 기본값 (단독 사용 금지) | - |
| `values/metallb.yaml` | **minikube** | Service type=LoadBalancer |
| `values/ingress.yaml` | 일반 CSP Kubernetes | ingress-nginx + host 기반 |
| `values/tailscale.yaml` | oci-oke | Ingress(class=tailscale) → tailnet HTTPS |

`values.yaml`을 항상 먼저 주고 그 위에 환경 오버레이를 얹습니다.

```bash
-f helm/kubesentinel-ai/values.yaml -f helm/kubesentinel-ai/values/metallb.yaml
```

배포 전 렌더 결과를 확인하려면:

```bash
helm template kubesentinel helm/kubesentinel-ai \
  -f helm/kubesentinel-ai/values.yaml -f helm/kubesentinel-ai/values/metallb.yaml

# API 서버 검증까지 포함 (클러스터 필요)
helm upgrade --install kubesentinel helm/kubesentinel-ai -n kubesentinel --create-namespace \
  -f helm/kubesentinel-ai/values.yaml -f helm/kubesentinel-ai/values/metallb.yaml \
  --set ai.endpoint=http://mock-llm:8080/v1 --dry-run=server
```

> `values/tailscale.yaml`은 ArgoCD가 참조하는 GitOps 대상입니다.
> 로컬 테스트 때문에 이 파일의 이미지 태그를 수정하지 마세요.

---

## 트러블슈팅

| 증상 | 원인 | 해결 |
|---|---|---|
| 백엔드 파드 `CrashLoopBackOff`, 로그에 `ai.endpoint must be specified` | endpoint 미설정 | `--set ai.endpoint=...` 지정 |
| 백엔드가 `waiting for database...` 반복 | Postgres 미기동 | `kubectl -n kubesentinel get pvc` → `Pending`이면 StorageClass 확인 |
| PVC가 계속 `Pending` | default StorageClass 없음 | `minikube addons enable default-storageclass storage-provisioner` |
| postgres `CrashLoopBackOff` + `mkdir: ... /pgdata: Permission denied` | **다중 노드 + hostPath PV** (아래 설명) | `--set postgres.nodeSelector."node-role\.kubernetes\.io/control-plane"=""` |
| `ErrImagePull` (로컬 이미지) | `pullPolicy=Always` | `--set image.pullPolicy=Never` + `minikube image load` |
| `exec format error` | 아키텍처 불일치 | `docker build --platform linux/arm64` 로 재빌드 |
| LoadBalancer IP가 `<pending>` | metallb 미설치 | 애드온 활성화 또는 port-forward |
| EXTERNAL-IP는 나오나 접속 불가 | 호스트에서 대역 라우팅 안 됨 | `minikube tunnel` 또는 port-forward |
| LLM 호출 타임아웃 | `localhost`로 설정됨 | `host.minikube.internal`로 변경 |
| 알림 전송 실패 경고 | `notify-sink` 미배포 | mock 스택 배포 또는 `--set secret.notifierWebhook=` |
| Settings의 "Pod 재시작" 버튼이 503 | `rbac.selfRestart` 기본 off | `--set rbac.selfRestart.enabled=true` |

### 다중 노드에서 postgres가 기동하지 못하는 이유

hostPath 계열 StorageClass(`minikube-hostpath`, `local-path` 등)의 PV는 **노드 로컬**이고
`nodeAffinity`가 비어 있습니다. 그래서 이런 일이 벌어집니다.

1. 프로비저너(컨트롤플레인 static pod)가 자기 노드에 `0777` 디렉토리를 생성
2. 스케줄러는 PV에 노드 제약이 없으니 postgres를 **워커 노드**에 배치
3. 워커에는 그 경로가 없어 kubelet이 **`root:root 0755`로 새로 생성**
4. postgres는 `runAsUser: 999`라 그 안에 `pgdata`를 만들 수 없음 → `Permission denied`

`postgres.yaml`에 `fsGroup: 999`가 있지만 **hostPath 볼륨은 fsGroup 소유권 변경을
지원하지 않아** 무력화됩니다. 따라서 해법은 **프로비저너가 있는 노드로 고정**하는 것입니다.

```yaml
postgres:
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
```

`values/metallb.yaml`에 이미 들어 있습니다. 단일 노드에서도 무해하며,
네트워크 스토리지(CSI)를 쓰면 비워두면 됩니다.

> 노드를 옮기면 이전 노드의 데이터에 접근할 수 없어 **DB가 새로 초기화**됩니다.
> 백엔드는 기동 시에만 마이그레이션을 적용하므로, 파드를 재생성해 스키마를 복원하세요.
> `kubectl -n kubesentinel delete pod -l app.kubernetes.io/name=kubesentinel-ai`

상태 확인 일괄 명령:

```bash
kubectl -n kubesentinel get pods,svc,pvc
kubectl -n kubesentinel describe pod -l app.kubernetes.io/name=kubesentinel-ai | tail -30
kubectl -n kubesentinel logs -l app.kubernetes.io/name=kubesentinel-ai --tail=50
```

---

## 정리

```bash
helm uninstall kubesentinel -n kubesentinel

# Postgres 데이터(설정·인시던트)까지 비울 때
kubectl -n kubesentinel delete pvc --all

# mock 스택까지 제거
kubectl -n kubesentinel delete -f deploy/local/minikube-mock-stack.yaml
kubectl delete namespace kubesentinel
```

PVC를 남겨두면 재배포 시 이전 설정과 인시던트가 그대로 복원됩니다.
클린 상태로 테스트하려면 PVC를 지우세요.
