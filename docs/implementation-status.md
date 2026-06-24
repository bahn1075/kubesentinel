# KubeSentinel AI — 구현 현황 & 연속 작업 가이드

> 이 문서는 **현재 코드의 실제 상태**와 **다음 작업자(사람 또는 AI 에이전트)가 바로 이어받기 위한 정보**를 담는다.
> 목표 설계는 [`architecture.md`](architecture.md)에 있고, 이 문서는 "지금 어디까지 됐고, 무엇을, 어떻게 이어서 할지"를 다룬다.
> 설계와 구현이 충돌하면 **architecture.md가 의도의 기준**이며, 구현은 그쪽으로 수렴시킨다.

- 문서 상태: Living doc — 코드 변경 시 함께 갱신한다.
- 기준 시점: MVP-0 핵심 흐름(수집 → RCA → 알림) 동작 가능.
- 모듈: `kubesentinel-ai` / Go 1.26 / 외부 의존성 없음(표준 라이브러리만).

---

## 1. 현황 한눈에

| 컴포넌트 | 패키지 | 상태 | 비고 |
|---|---|---|---|
| 진입점 / 컴포넌트 조립 | `cmd/kubesentinel-ai` | ✅ 동작 | env 기반 설정 로드 → 컴포넌트 주입 → webhook 서버 기동 |
| 설정 | `internal/config` | ✅ 동작 (부분) | env 오버라이드 + 기본값. **YAML 파일 로딩은 태그만 있고 미구현** |
| Signal Collector — 진입 | `internal/collector` (webhook.go) | ✅ 동작 | `/v1/alerts` Alertmanager webhook 수신 → 비동기 처리 |
| Signal Collector — 보강 | `internal/collector` (prometheus/loki/enrich) | ✅ 동작 | Prom instant 쿼리 + Loki 로그. **best-effort(실패해도 흐름 진행)** |
| Signal Collector — K8s | — | ❌ 미구현 | Events·워크로드 manifest 스냅샷. client-go 의존성 필요 |
| 도메인 모델 | `internal/models` | ✅ 동작 | EvidenceBundle / DiagnosisResult / AlertmanagerPayload / AIClient |
| AI Gateway | `internal/provider` | ✅ 동작 | OpenAI 호환 `/chat/completions`. fallback·토큰제한·redact 미구현 |
| Diagnosis Engine | `internal/diagnosis` | ✅ 동작 (LLM only) | LLM RCA + JSON 추출. **Rule/Correlation/RAG 분석기 미구현** |
| Notifier | `internal/notifier` | ✅ 동작 | Discord/Slack/Teams webhook. 단방향 알림만(승인 액션 없음) |
| Remediation Planner | — | ❌ 미구현 | architecture §4.4 |
| Policy & Safety | `internal/policy` (빈 디렉토리) | ❌ 미구현 | architecture §4.6, §9 |
| GitOps Executor | `internal/gitops` (빈 디렉토리) | ❌ 미구현 | architecture §4.5 — MVP-1 핵심 |
| Audit | `internal/audit` (빈 디렉토리) | ❌ 미구현 | |
| Helm chart / Dockerfile | — | ❌ 미구현 | architecture §12 |
| 테스트 | `*_test.go` | ✅ 부분 | models / provider / notifier 단위 테스트 존재. collector·engine 미커버 |
| Dockerfile / Helm / ArgoCD | `Dockerfile`, `charts/`, `deploy/argocd/` | ✅ 동작 | multi-arch 이미지 + Helm 차트 + ArgoCD Application. `helm lint`·`docker build` 검증됨 |
| Frontend (dashboard) | `frontend/` | ✅ 동작 | React+Vite+TS. **Settings·Incidents 백엔드 API(DB) 연동**, policies/approvals는 아직 mock. nginx가 `/api`→백엔드 프록시. Helm `frontend.enabled` |
| 설정 영속화 (Settings) | `internal/store`, `/api/settings` | ✅ 동작 | Postgres + goose 임베드 마이그레이션(v2). 비민감 설정만 저장(민감정보는 Secret/env). 기동 시 cfg에 병합되어 파이프라인이 소비(§3.6). 재시작·Pod 재생성 후 영속 검증됨 |
| 인시던트 영속화 (Incidents) | `internal/store`, `/api/incidents` | ✅ 동작 | webhook 처리 시 `incidents` 테이블 저장 → `GET /api/incidents[/{id}]`로 대시보드 조회. camelCase 뷰가 프론트 타입과 정합 |
| Postgres | `charts/.../postgres.yaml`, compose | ✅ 동작 | 차트 `postgres.enabled`(로컬/테스트) 또는 `database.url`(외부 DB). PVC 영속 |

범례: ✅ 동작 · [~]/부분 · ❌ 미구현

---

## 2. 실제 코드 구조

```
kubesentinel-ai/
  cmd/kubesentinel-ai/main.go      # 조립 + 기동
  internal/
    config/      config.go         # Config 구조체 + env 로딩 + Validate
    models/      alert.go          # AlertmanagerPayload, Alert, Labels
                 evidence.go       # EvidenceBundle, GitContext, NewEvidenceBundle()
                 diagnosis.go      # DiagnosisResult, ProposedAction
                 ai.go             # AIClient 인터페이스, ChatResponse  ← 공유 인터페이스 위치
    collector/   webhook.go        # HTTP 수신 → enrich → analyze → notify
                 prometheus.go     # PrometheusClient.QueryInstant
                 loki.go           # LokiClient.QueryRecent
                 enrich.go         # Enricher: bundle에 metric/log 보강
    provider/    ai_gateway.go     # AIGateway (models.AIClient 구현)
    diagnosis/   engine.go         # Engine.Analyze: 프롬프트 → LLM → JSON 파싱
    notifier/    notifier.go       # Notifier 인터페이스, noopNotifier
                 webhook.go        # webhookNotifier (discord/slack/teams)
    audit/ gitops/ policy/         # 빈 디렉토리 (플레이스홀더)
  Dockerfile / .dockerignore       # multi-arch(amd64+arm64) distroless 이미지
  Makefile                         # build/test/docker/helm 헬퍼
  charts/kubesentinel-ai/          # Helm 차트 (백엔드 + frontend 함께 배포)
  deploy/argocd/application.yaml   # ArgoCD Application (GitOps 배포)
  frontend/                        # operator 대시보드 (React+Vite+TS, 별도 이미지)
    src/{api,pages,components,lib}  #   api/=타입·mock·client / pages/=화면
    Dockerfile nginx.conf          #   node 빌드 → nginx-unprivileged 서빙
  docs/          architecture.md   # 목표 설계
                 implementation-status.md  # 이 문서
```

### 의존 방향 (import cycle 주의)
```
models  ← (leaf, 아무것도 import 안 함)
config  ← (leaf)
provider  → config, models
diagnosis → models
notifier  → config, models
collector → diagnosis, models, notifier, config
cmd       → 전부
```
> **`AIClient` 인터페이스는 `models`에 둔다.** provider가 구현하고 diagnosis가 소비하는데, 인터페이스를 provider나 diagnosis에 두면 순환참조가 난다. 같은 이유로 새 공유 타입은 `models`에 배치한다.

---

## 3. 런타임 흐름

```
Alertmanager
   │  POST /v1/alerts  (AlertmanagerPayload JSON)
   ▼
collector.handleAlertmanagerWebhook
   │  models.NewEvidenceBundle(payload)   # 첫 alert → EvidenceBundle (워크로드 추정)
   │  즉시 200 OK 응답, 이후는 goroutine에서 비동기 처리
   ▼
Enricher.Enrich(bundle)                   # best-effort
   │  PrometheusClient.QueryInstant(...)  # 재시작 횟수 / 메모리
   │  LokiClient.QueryRecent(...)         # 대상 pod 최근 로그
   ▼
diagnosis.Engine.Analyze(bundle)
   │  bundle → JSON 직렬화 → 프롬프트
   │  AIClient.Chat(prompt, context)      # OpenAI 호환
   │  응답 문자열에서 {...} 추출 → DiagnosisResult
   ▼
notifier.NotifyDiagnosis(bundle, result)  # Discord/Slack/Teams
```

핵심 특성:
- **수신은 즉시 200**, 분석은 goroutine. (Alertmanager 재전송 방지) — 단, 현재 **동시성 제한·중복 억제(cooldown) 없음** → §6 백로그.
- **보강 실패는 무시**하고 진단을 진행한다(엔드포인트 미설정/장애 내성).

---

## 4. 설정 레퍼런스

설정은 현재 **환경변수 + 코드 기본값**으로만 주입된다(`config.LoadConfig`). YAML 파일 로딩은 미구현.

| 환경변수 | 매핑 | 기본값 | 필수 |
|---|---|---|---|
| `KUBESENTINEL_AI_ENDPOINT` | AI.Endpoint | — | ✅ (openai-compatible) |
| `KUBESENTINEL_AI_MODEL` | AI.Model | — | LLM에 따라 |
| `KUBESENTINEL_AI_API_KEY` | AI.APIKey | — | 엔드포인트에 따라 |
| `KUBESENTINEL_AI_PROMETHEUS_URL` | Collector.PrometheusURL | — | 미설정 시 metric 보강 skip |
| `KUBESENTINEL_AI_LOKI_URL` | Collector.LokiURL | — | 미설정 시 log 보강 skip |
| `KUBESENTINEL_AI_GRAFANA_URL` | Collector.GrafanaURL | — | 알림 딥링크용(선택) |
| `KUBESENTINEL_AI_NOTIFIER_TYPE` | Notifier.Type | `slack`로 해석 | `discord`/`slack`/`teams` |
| `KUBESENTINEL_AI_NOTIFIER_WEBHOOK` | Notifier.Webhook | — | 미설정 시 noop(알림 안 감) |
| `KUBESENTINEL_AI_GIT_TOKEN` | GitOps.Token | — | MVP-1부터 |
| `KUBESENTINEL_AI_DATABASE_URL` | Database.URL | — | 설정 영속화용 Postgres DSN. 미설정 시 `/api/settings` 503 |

> `Validate()`는 현재 `ai.provider_type`와 openai-compatible의 `endpoint`만 검사한다.
> Port·LogLevel은 코드 기본값(8080/info)이며 env 노출 안 됨 → 필요 시 추가.

---

## 5. 빌드 / 테스트 / 실행

```bash
# 빌드 & 정적검사
go build ./...
go vet ./...
gofmt -l internal/ cmd/      # 출력 없으면 clean

# 테스트
go test ./...

# 로컬 실행 (예: Ollama를 LLM으로)
export KUBESENTINEL_AI_ENDPOINT=http://localhost:11434/v1
export KUBESENTINEL_AI_MODEL=llama3
export KUBESENTINEL_AI_NOTIFIER_TYPE=discord
export KUBESENTINEL_AI_NOTIFIER_WEBHOOK=https://discord.com/api/webhooks/...
go run ./cmd/kubesentinel-ai

# 알림 수신 스모크 테스트 (Alertmanager 페이로드 흉내)
curl -X POST localhost:8080/v1/alerts -H 'Content-Type: application/json' -d '{
  "receiver":"kubesentinel","status":"firing",
  "alerts":[{"status":"firing","labels":{
    "alertname":"KubePodCrashLooping","namespace":"production",
    "pod":"api-server-abc","deployment":"api-server","severity":"critical"}}]
}'
```

### 로컬 통합 테스트 (docker compose)

외부 의존성 없이 **alert → 수집 → RCA → 알림** 전체 흐름을 검증한다.
구성: backend + frontend + mock-llm(OpenAI 호환 고정 RCA) + notify-sink(알림 로그 출력).

```bash
docker compose up --build            # 4개 서비스 기동
# 다른 터미널에서 alert 주입:
curl -X POST localhost:8080/v1/alerts -H 'Content-Type: application/json' \
  --data @deploy/local/sample-alert.json
docker compose logs -f backend       # 진단 결과
docker compose logs -f notify-sink   # 전송된 알림 내용
# 대시보드: http://localhost:8081  (mock 모드)
docker compose down
```

> ⚠️ 호스트 8080 포트가 다른 프로세스에 점유돼 있으면 curl이 컨테이너 대신 그쪽으로 간다.
> `lsof -nP -iTCP:8080 -sTCP:LISTEN` 로 확인하라.

### 배포 (Docker → Helm → ArgoCD)

최종 배포 형태는 **Helm 차트를 ArgoCD가 GitOps로 관리**하는 것이다.

```bash
# 1) 이미지 빌드/푸시 (multi-arch, architecture.md R2)
make docker-push REGISTRY=ghcr.io/your-org TAG=v0.1.0

# 2) Helm 직접 설치 (개발/검증용)
helm install kubesentinel charts/kubesentinel-ai -n kubesentinel --create-namespace \
  --set image.repository=ghcr.io/your-org/kubesentinel-ai --set image.tag=v0.1.0 \
  --set ai.endpoint=http://ollama.llm.svc:11434/v1 --set ai.model=llama3 \
  --set collector.prometheusUrl=http://prometheus-operated.monitoring.svc:9090 \
  --set collector.lokiUrl=http://loki-gateway.monitoring.svc:80 \
  --set secret.notifierWebhook=<slack-webhook>

# 3) 운영: ArgoCD Application으로 관리 (민감정보는 existingSecret 참조)
kubectl apply -n argocd -f deploy/argocd/application.yaml
```

배포 산출물 요약:
- **Dockerfile**: `--platform=$BUILDPLATFORM` 교차컴파일 → distroless static(nonroot). CGO off.
- **charts/kubesentinel-ai/**: deployment·service·serviceaccount·rbac(read-only ClusterRole)·secret·NOTES. 모든 §4 엔드포인트를 values로 주입. 헬스 프로브는 `/healthz` 부재로 **tcpSocket** 사용(백로그 #6에서 httpGet 전환).
- **deploy/argocd/application.yaml**: automated sync(prune+selfHeal), CreateNamespace, ServerSideApply.

> **주의 — 아직 env로 주입되지 않는 values**: `ai.allowExternal`·`ai.redactSecrets`·`ai.providerType`·`gitops.*`·`collector.logLines`·`logLevel`은 values에 노출돼 있으나 `config.LoadConfig`가 읽지 않는다. config 파일 로딩 또는 env 매핑 확장이 필요(백로그 #3 및 config 항목).

---

## 6. 다음 작업 백로그 (우선순위)

> 작업 단위로 적었다. 각 항목은 architecture.md의 해당 절을 근거로 한다.

### MVP-0 마무리
1. **Kubernetes Events + manifest 스냅샷 수집** — `internal/collector/kubernetes.go`. client-go(in-cluster config) 추가, `EvidenceBundle.Events`/`ResourceYAML` 채우기. *의존성 추가가 필요한 첫 작업.*
2. **Diagnosis: Rule Analyzer** — LLM 호출 전에 CrashLoop/OOM/ImagePull 등 룰 기반 1차 분류(`internal/diagnosis/rule_based.go`). LLM 비용·환각 감소.
3. **redactSecrets / maxInputTokens 적용** — provider에서 evidence 전송 전 시크릿 마스킹·토큰 절단(현재 config 값만 있고 미사용).

### Frontend 연동 (대시보드를 mock에서 실데이터로)
3.5. **백엔드 조회 API** — ✅ Settings + Incidents 완료. `GET/PUT /api/settings`, `GET /api/incidents`, `GET /api/incidents/{id}` (모두 Postgres). 인시던트는 webhook 처리 시 `incidents` 테이블에 영속화(state=DiagnosisCompleted/ValidationFailed). 남은 것: `/api/policies`(정책 관리 미구현 → 아직 mock), Approvals(MVP-2).
3.6. **DB 설정을 파이프라인이 소비** — ✅ 기동 시 `store.GetSettings()`를 cfg에 병합(`applyDBSettings`) 후 컴포넌트 구성. UI에서 저장한 AI endpoint/model·collector·notifier가 실제 수집·진단·알림에 반영됨. 민감정보(키/토큰/webhook)는 계속 Secret/env.
  - ⚠️ **설정 변경은 백엔드 재시작 시 반영**(기동 시 1회 병합). 런타임 hot-reload(인시던트마다 DB 조회 또는 watch)는 후속 과제.

### 견고성 (MVP-0와 병행 권장)
4. **동시성 제한 + 중복 억제** — webhook goroutine에 worker pool + incident 키별 cooldown(architecture §9 `cooldownSeconds`). 현재 무제한 goroutine.
5. **구조화 로깅** — `fmt.Printf` → `log/slog`. LogLevel 설정 연결.
6. **graceful shutdown / healthz** — `select{}` 대신 signal 처리, `/healthz`·`/readyz` 추가(K8s probe·smoke test용, architecture R5).
7. **collector·engine 테스트** — handler end-to-end(httptest mock LLM+notifier), 보강 실패 내성.

### MVP-1 — GitOps PR (architecture §4.5, §9)
8. **`internal/policy`** — `RemediationPolicy` 평가기: allowedPaths/deniedPaths, action allow/deny, minConfidence, risk. **PR 생성보다 먼저 구현**(R4).
9. **`internal/gitops`** — `GitProvider` 인터페이스(createBranch/commit/openPR) + GitHub 구현. 단일 repo 스코프 토큰.
10. **Remediation Planner** — DiagnosisResult → 매니페스트 patch(diff) 생성.

### 운영 (architecture §12)
11. ✅ **Dockerfile (multi-arch) + Helm chart + ArgoCD Application** — 완료. 남은 후속:
    - config가 안 읽는 values를 실제 동작하게 연결(위 §5 주의 참고)
    - `/healthz` 구현 후 프로브를 httpGet으로 전환(백로그 #6)
    - Alertmanager receiver를 차트에 옵션으로 포함(`AlertmanagerConfig` CR 또는 values 가이드)
12. **Phase B**: 검증된 도메인 모델을 CRD로 승격 + controller-runtime (architecture §6).

---

## 7. AI 에이전트를 위한 작업 지침

이 코드베이스를 이어서 작업하는 에이전트는 다음을 지킨다.

1. **변경 후 항상** `go build ./... && go vet ./... && go test ./...` 으로 검증하고, `gofmt -w` 로 포맷한다.
2. **새 공유 타입/인터페이스는 `models`에** 둔다(§2 import 방향). provider↔diagnosis 순환참조를 만들지 말 것.
3. **엔드포인트·repo·모델명·토큰을 코드에 하드코딩하지 않는다**(architecture §2, R1). 전부 config(env)로 주입.
4. **write 경로(MVP-1+)를 추가할 땐 정책 가드를 먼저** 단다(architecture §4.6 안전 가드, R4). evidence 없으면 write 없음, production write는 승인 필수, 플랫폼/관측/GitOps 평면은 default-deny.
5. **외부 전송 주의**: LLM·webhook으로 evidence를 보내기 전 시크릿 redact를 고려(R8). `allowExternalModel` 기본 off 의도를 깨지 말 것.
6. **새 기능은 단위 테스트와 함께**, 그리고 **이 문서의 §1 현황 표·§6 백로그를 갱신**한다.
