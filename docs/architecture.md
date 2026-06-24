# KubeSentinel AI — 아키텍처 설계 (범용 클러스터판)

> 본 문서는 개념 노트 `Kubesentinel AI.md`(GPT-5.5 응답)를 **CSP 중립적인 일반 Kubernetes 클러스터**에 신규 배포하는 것을 전제로 구체화한 설계 문서다.
> 특정 클라우드(OKE/EKS/GKE/AKS) 또는 사전 구성된 환경에 의존하지 않으며, **깨끗한 클러스터에 KubeSentinel과 그 의존 스택을 처음부터 설치**하는 경로를 함께 정의한다.
>
> - 대상: 임의의 CNCF-conformant Kubernetes 클러스터 (managed / on-prem / kind·k3s 포함)
> - 문서 상태: Draft v0.2 (범용판)
> - 전제: Kubernetes ≥ 1.27, 클러스터 관리자 권한, 아웃바운드 또는 사내 git/registry 접근

---

## 1. 목적과 범위

### 1.1 한 문장 정의

> Prometheus/Alertmanager·Loki에서 장애 신호를 수집하고, OpenAI 호환 LLM으로 원인을 분석한 뒤, 정책으로 허용된 범위 안에서 GitOps PR을 생성하고 알림 채널로 승인 요청을 보내는 Kubernetes remediation 시스템.

### 1.2 설계 원칙 (개념 9장 계승)

1. **AI는 판단자가 아니라 제안자다.** 허용 범위·위험도·승인 여부·적용 방식은 *시스템*이 결정한다.
2. **GitOps가 1순위 조치 경로다.** runtime patch는 예외(긴급 완화)이며, 사후 Git 반영을 강제한다.
3. **근거 없는 조치 금지.** evidence 없으면 write 없음, 신뢰도 낮으면 제안만, production write는 승인 필수.
4. **변경 범위는 명시적으로 제한된다.** repo / branch / path / kind / namespace / action / risk 단위로 화이트리스트.
5. **자기 자신과 관측·제어 평면은 건드리지 않는다.** remediation 대상에서 관측 스택·GitOps·정책 엔진·KubeSentinel 자신을 default-deny 한다.

### 1.3 비목표 (Non-goals)

- 특정 CSP 매니지드 기능(예: 클라우드 LB·노드풀 오토스케일러)에 대한 직접 제어는 범위 밖이다. KubeSentinel은 **클러스터 내부 리소스와 git 매니페스트**만 다룬다.
- 멀티 클러스터 fleet 관리는 v1 범위 밖(단일 클러스터 우선, 아키텍처는 확장 가능하게 둔다).

---

## 2. 플랫폼 의존성 (Required Platform Components)

KubeSentinel은 **스스로 인프라를 만들지 않고, 표준 오픈소스 스택을 전제**한다. 깨끗한 클러스터라면 아래를 먼저(또는 KubeSentinel과 함께) 설치한다. 모든 항목은 CNCF/오픈소스이며 특정 CSP에 묶이지 않는다.

| 영역 | 권장 컴포넌트 | 역할 | 필수/선택 | 비고 |
|---|---|---|---|---|
| Metrics | **kube-prometheus-stack** (Prometheus Operator) | metric 수집, alert 규칙 | 필수 | Helm `prometheus-community/kube-prometheus-stack` |
| Alerts | **Alertmanager** (위 스택 포함) | alert 라우팅 → KubeSentinel webhook | 필수 | |
| Logs | **Loki** + 수집기(Promtail/Alloy/Fluent Bit) | 로그 집계·쿼리 | 필수 | Helm `grafana/loki` |
| Dashboard | **Grafana** | 시각화·딥링크 | 선택(권장) | 알림 메시지 딥링크용 |
| GitOps | **Argo CD** (또는 Flux) | git desired state → 클러스터 반영 | 필수 | KubeSentinel은 reconciler가 아니라 PR 생성자 |
| Policy(admission) | **Kyverno** (또는 OPA/Gatekeeper) | KubeSentinel 권한의 클러스터 강제 | 선택(권장) | 없으면 in-app policy만으로 시작 가능 |
| LLM | OpenAI 호환 엔드포인트 | RCA 추론 | 필수 | 내부(Ollama/vLLM) 또는 frontier API |
| Notification | Discord/Slack/Teams webhook | 알림·승인 | 필수 | 채널 1개 이상 |
| Git provider | GitHub/GitLab/Bitbucket | PR 대상 repo | 필수 | 매니페스트 repo 1개 |

> **설계 원칙:** 위 컴포넌트의 **서비스 주소·릴리스 이름은 절대 코드에 하드코딩하지 않는다.** 전부 Helm values / 환경변수 / `AIProvider`·`RemediationPolicy` 리소스로 주입한다. 동일 산출물이 EKS·GKE·on-prem·kind에서 값만 바꿔 동작해야 한다.

### 2.1 Bootstrap (깨끗한 클러스터 → 동작까지)

```
0. 클러스터 준비 (managed/on-prem/kind, ≥1.27)
1. 관측 스택 설치   helm install kube-prometheus-stack / loki / (grafana)
2. GitOps 설치      helm/manifest install argo-cd  →  매니페스트 repo 연결
3. (선택) Kyverno   helm install kyverno
4. LLM 준비         내부(ollama/vllm) 배포 또는 frontier API 키 확보
5. KubeSentinel     helm install kubesentinel  (values로 위 엔드포인트 주입)
6. 배선             Alertmanager에 KubeSentinel webhook receiver 추가
7. 정책             RemediationPolicy / AIProvider 적용
```

> 1~3은 이미 갖춰진 클러스터라면 건너뛰고 5~7만 수행한다(브라운필드 호환).

---

## 3. 목표 아키텍처

```
┌────────────────────────────────────────────────────────────────┐
│ Observability  (플랫폼 의존성, 값으로 주입)                     │
│  Prometheus  /  Alertmanager  /  Loki  /  Grafana               │
└───────────────────────────────┬────────────────────────────────┘
                                 │ Alertmanager webhook receiver
                                 ▼
┌────────────────────────────────────────────────────────────────┐
│ KubeSentinel AI  (전용 namespace)                               │
│                                                                 │
│  ① Signal Collector   alert + Prom query + Loki query + events  │
│         │             + 대상 워크로드 manifest snapshot         │
│         ▼                                                       │
│  ② Diagnosis Engine   Rule Analyzer → Correlation → LLM RCA     │
│         │             (+ Runbook RAG, 추후)                     │
│         ▼                                                       │
│  ③ Remediation Planner  FixPlan / git diff / risk / confidence  │
│         │                                                       │
│    ┌────┴─────┐                                                 │
│    ▼          ▼                                                 │
│  ④ Policy   ⑤ Approval                                          │
│  (in-app +   (Discord/Slack/                                    │
│   Kyverno)    Teams webhook)                                    │
│    └────┬─────┘                                                 │
│         ▼                                                       │
│  ⑥ GitOps Executor → <manifest repo> (branch + PR)              │
└───────────────────────────────┬────────────────────────────────┘
                                 │ git push (branch + PR)
                                 ▼
┌────────────────────────────────────────────────────────────────┐
│ Argo CD / Flux  (플랫폼 의존성)                                 │
│   git desired state  →  cluster live state                      │
└───────────────────────────────┬────────────────────────────────┘
                                 ▼
┌────────────────────────────────────────────────────────────────┐
│ Verification (KubeSentinel 로직, 기존 데이터 활용)              │
│  alert resolved? · restart count · metric 회복 · Argo Synced?   │
└────────────────────────────────────────────────────────────────┘
```

핵심: **KubeSentinel은 GitOps 컨트롤러(Argo/Flux)를 executor가 아니라 "반영 담당자"로 둔다.** KubeSentinel은 git 변경안과 승인 흐름만 관리하고, 실제 클러스터 반영은 GitOps 컨트롤러가 한다. → 모든 조치가 git history + PR + sync 로그로 감사된다.

---

## 4. 컴포넌트 설계

### 4.1 Signal Collector

**진입점:** Alertmanager webhook receiver. 깨끗한 클러스터에서는 Alertmanager 설정에 KubeSentinel용 receiver와 매칭 룰을 추가하는 것이 배선의 핵심이다.

```yaml
# Alertmanager 설정 (kube-prometheus-stack values 경유)
receivers:
  - name: kubesentinel
    webhook_configs:
      - url: http://<kubesentinel-svc>.<ns>.svc:8080/v1/alerts
        send_resolved: true
route:
  routes:
    - matchers: [ "severity =~ warning|critical" ]
      receiver: kubesentinel
      continue: true   # 기존 라우팅 보존
```

수집 입력(엔드포인트는 전부 설정 주입):

- Alertmanager webhook payload (alert 본문)
- Prometheus 쿼리 API (예: 메모리 사용량 p95 vs limit)
- Loki 쿼리 API (대상 pod 로그)
- Kubernetes Events + 대상 워크로드 manifest (in-cluster client-go)
- GitOps application 상태 + 최근 git commit

산출물 = **EvidenceBundle** (개념 3.1 JSON 스키마 그대로 사용).

```json
{
  "incident_id": "inc-20260617-001",
  "source": "alertmanager",
  "alert": "KubePodCrashLooping",
  "namespace": "<ns>", "workload": "<workload>", "pod": "<pod>",
  "metrics": [], "logs": [], "events": [], "resource_yaml": {},
  "git_context": { "repo": "<repo>", "path": "<path>", "last_commit": "..." }
}
```

### 4.2 AI Gateway

OpenAI 호환 API로 추상화한다. **모델 provider를 코어 로직에 박지 않는다.** 내부 LLM과 frontier API를 동일 인터페이스로 교체 가능하게 둔다.

```yaml
apiVersion: remedy.ai/v1alpha1
kind: AIProvider
metadata: { name: default-provider }
spec:
  type: openai-compatible
  endpoint: <https://llm-endpoint/v1>   # ollama / vLLM / LocalAI / frontier gateway
  model: <model-name>
  secretRef: { name: ai-provider-secret, key: apiKey }
  fallback:                               # 선택: 가용성 확보
    type: openai-compatible
    endpoint: <fallback-endpoint/v1>
  policy:
    allowExternalModel: false             # 외부 전송 허용 여부
    redactSecrets: true
    maxInputTokens: 120000
    temperature: 0.1
```

지원 백엔드(개념 3.2): frontier API(OpenAI/Azure/Anthropic·Gemini adapter), 로컬 LLM(Ollama / vLLM / LocalAI / 사내 gateway). **선택은 배포 환경의 정책에 맡기고, system prompt·tool schema·evidence schema는 백엔드와 무관하게 안정 유지한다.**

### 4.3 Diagnosis Engine (다층 분석)

1. **Rule Analyzer** — CrashLoopBackOff / OOMKilled / ImagePullBackOff / Pending(자원부족) / Probe failure / Service selector mismatch / PVC pending
2. **Correlation Analyzer** — 최근 rollout 여부, 특정 commit 이후 발생, node/zone 편중, metric spike ↔ log error 시간 상관
3. **LLM Analyzer** — evidence 요약 → root cause 후보 → 수정안 후보 → 영향 설명 → 검증 계획 (structured JSON 출력)
4. **Runbook RAG** — (MVP 이후) 매니페스트 repo 내 markdown runbook + metadata. CNCF HolmesGPT 사례처럼 모델보다 runbook 품질이 조사 결과를 좌우한다.

### 4.4 Remediation Planner

개념 3.4의 `RemediationPlan` 스키마 사용(target / diagnosis / proposedChanges / risk / execution). 조치 유형별 기본 정책:

| 유형 | 설명 | 기본 정책 |
|---|---|---|
| Suggestion | 조치 후보만 생성 | 모든 환경 허용 |
| Git PR | 소스/매니페스트 변경 PR | **기본 조치 방식** |
| Git Commit | branch 직접 commit | 제한적 허용 |
| Runtime Patch | 클러스터 직접 patch | dev/test 한정 |
| Runtime Delete | Pod delete 등 | 낮은 위험 조건만 |
| Rollback | 이전 revision 복구 | 승인 필요 |
| Source Code Patch | app 소스 수정 | 매우 제한적 |

### 4.5 GitOps Executor ⭐ (최대 차별점, 최대 리스크)

- **대상 repo:** 매니페스트 repo(GitHub/GitLab/Bitbucket). 자격증명은 **단일 repo로 스코프 한정된 fine-grained 토큰**.
- **동작 모드:**
  - Mode 1 (기본) **PR-only** — branch 생성 → manifest/Helm values patch → PR 생성 → 알림 → 사람이 merge
  - Mode 2 **policy 내 auto-merge** — dev/test 한정, 낮은 위험만 (MVP-3)
  - Mode 3 **긴급 runtime patch** — 장애 완화 후 git 반영 강제 (후순위)
- **provider 추상화:** `GitProvider` 인터페이스(createBranch / commit / openPR / merge)를 두고 GitHub부터 구현, GitLab·Bitbucket 확장.
- **allowedPaths / deniedPaths** 화이트리스트(§9)로 변경 범위 강제. GitOps 컨트롤러의 root(app-of-apps) 매니페스트와 플랫폼 컴포넌트 경로는 deny.

### 4.6 Policy & Safety Engine

2계층:

1. **In-app policy (1차, 빠름)** — `RemediationPolicy`: allowedPaths/deniedPaths, action allow·deny, risk, approval, cooldown·rate limit, minConfidence. **Kyverno 없이도 단독 동작**한다(깨끗한 클러스터의 최소 구성).
2. **Admission policy (2차, 클러스터 강제, 선택)** — Kyverno 또는 OPA/Gatekeeper로 KubeSentinel service account 권한을 admission 단에서 한 번 더 제한.

안전 가드(개념 9.4):
```
No evidence                  → no write
Low confidence               → suggestion only
Production write             → approval required
Secret/RBAC change           → denied by default
Self / 관측·GitOps·정책 평면  → denied by default
```

### 4.7 알림 / 승인

- **채널 추상화:** `Notifier` 인터페이스 — Discord / Slack / Teams webhook 중 1개 이상.
- **2단계 메시지(개념 3.7):** Notification(발생/분석완료/PR생성/승인대기/적용완료/검증실패) + Action(approve / reject / request-more-evidence / dry-run / escalate).
- 메시지에 PR 링크 + Grafana/Loki 딥링크 포함.

---

## 5. 상태 머신

```
IncidentDetected → EvidenceCollected → DiagnosisCompleted → PlanGenerated
  → PolicyEvaluated → ApprovalPending → GitPRCreated → GitMerged
  → Synced → Verified → Closed
```
실패 상태: `PolicyDenied · ApprovalRejected · ValidationFailed · GitPatchFailed · SyncFailed · VerificationFailed · SuppressedByCooldown`

---

## 6. 데이터 모델 — 단계적 도입

> 개념 6장은 처음부터 CRD 9종 + Go controller를 제안하지만, 초기에 그 보일러플레이트에 묶이면 핵심 로직(수집·진단·PR) 검증이 늦어진다.
>
> - **Phase A (MVP-0~1):** 단일 서비스 + 경량 상태(ConfigMap/내장 DB). CRD 없음.
> - **Phase B (안정화 후):** 검증된 도메인 모델을 CRD(`Incident`, `EvidenceBundle`, `Diagnosis`, `RemediationPlan`, `RemediationPolicy`, `AIProvider`, `RemediationRun`, `ApprovalRequest`, `RunbookSource`)로 승격하고 controller-runtime 도입. 이때 KubeSentinel 자체가 GitOps로 배포되는 Kubernetes-native 운영 모델로 전환.

---

## 7. MVP 로드맵

### MVP-0 — Read-only RCA + 알림 (write 전혀 없음) ⭐ 착수 지점
- [ ] Alertmanager에 `kubesentinel` webhook receiver 추가 *(배포 작업, 코드 외)*
- [~] 수신 서비스: alert 파싱 → Prom/Loki/event 수집 → EvidenceBundle 생성
  *(alert 파싱·Prom·Loki 구현됨 / Kubernetes Events·manifest 스냅샷은 미구현 — client-go 필요)*
- [x] AI Gateway: OpenAI 호환 LLM으로 RCA (structured JSON)
- [x] 알림 채널로 진단 결과 + 근거 + 딥링크 전송
- **가치/위험 비 최고. 클러스터에 쓰기 행위 0.**

> **구현 현황·연속 작업 지침은 [`implementation-status.md`](implementation-status.md) 참조.**
> 이 문서(architecture.md)는 *목표 설계*이고, implementation-status.md는 *현재 코드의 실제 상태*다.

### MVP-1 — GitOps PR 생성
- [ ] 매니페스트 repo write 토큰(단일 repo 스코프) 설정
- [ ] GitProvider 추상화(GitHub부터) + patch 생성(Helm values/Kustomize/manifest)
- [ ] allowedPaths/deniedPaths 강제 + PR body(evidence/diagnosis/verification plan)

### MVP-2 — 승인 기반 적용
- [ ] 알림 채널 approve/reject 액션 webhook
- [ ] merge는 사람 또는 전용 SA로 제한, GitOps sync 상태 추적
- [ ] 적용 후 metric/log/restart 검증

### MVP-3 — 제한적 자동 조치
- [ ] dev/test namespace 한정, Service selector mismatch 같은 낮은 위험만
- [ ] cooldown / rate limit / dry-run 필수 / audit log 필수

---

## 8. 장애 유형별 처리 전략 (개념 5장)

| 장애 유형 | 1차 분석 | 권장 조치 | 자동화 |
|---|---|---|---|
| CrashLoopBackOff | previous logs, events, 최근 deploy | 원인별 PR 또는 rollback | 조건부 |
| OOMKilled | memory metric vs limit | Helm values/resource limit PR | 승인 필요 |
| ImagePullBackOff | image tag, registry auth, pull secret | image tag PR 또는 secret 점검 알림 | PR 중심 |
| Probe failure | readiness/liveness 설정, app logs | probe threshold/path PR | 승인 필요 |
| Pending | scheduler event, quota, node capacity | request 조정 또는 capacity 알림 | 대부분 승인 |
| Service selector mismatch | selector vs pod label | selector/label PR | 낮은 위험 자동 가능 |
| ConfigMap/Secret missing | manifest reference | manifest 수정 PR | 승인 필요 |
| GitOps OutOfSync | live diff | git source/sync policy 분석 | 알림/PR |
| Error rate spike | Prometheus + Loki 상관분석 | 최근 release rollback PR | 승인 필요 |

---

## 9. 변경 범위 제한 — 샘플 RemediationPolicy

```yaml
apiVersion: remedy.ai/v1alpha1
kind: RemediationPolicy
metadata: { name: default-safe-policy }
spec:
  selector:
    matchLabels: { remedy.ai/enabled: "true" }   # 명시적 opt-in 워크로드만
  ai:
    providerRef: default-provider
    allowExternalModel: false
    redactSecrets: true
  gitops:
    provider: github                 # github | gitlab | bitbucket
    repository: <org>/<manifest-repo>
    baseBranch: main
    workingBranchPrefix: remedy/
    mode: pull-request
    allowedPaths:
      - <apps>/<app>/manifests/
      - <apps>/<app>/values*.yaml
    deniedPaths:
      - "**/argocd/**"               # GitOps 컨트롤러 자신
      - "**/kube-prometheus-stack/**"
      - "**/loki/**"
      - "**/grafana/**"
      - "**/kyverno/**"
      - "**/secrets/**"
      - "**/rbac/**"
      - "**/*app-of-apps*"           # 루트 앱 (연쇄 영향 방지)
  actions:
    allowed: [ collect_evidence, analyze, create_pull_request, notify ]
    conditional:
      - { action: merge_pull_request, environments: [ dev ], maxRisk: low }
    denied:
      - delete_namespace
      - patch_secret
      - patch_clusterrole
      - modify_network_policy
      - touch_platform_components    # 관측/GitOps/정책 스택
  approval:
    requiredFor: [ production, resource_limit_change, image_change, rollback, source_code_change ]
    channels:
      - { type: discord, targetRef: kubesentinel-alerts }   # 또는 slack/teams
  safety:
    cooldownSeconds: 900
    maxActionsPerWorkloadPerDay: 3
    requireEvidence: true
    minConfidenceForPR: 0.75
    minConfidenceForAutoMerge: 0.9
```

---

## 10. 이식성·운영 리스크와 결정사항

| # | 항목 | 영향 | 결정/완화 |
|---|---|---|---|
| R1 | **엔드포인트 하드코딩** | 환경 종속 | 모든 서비스 주소·repo·모델명을 values/CR로 주입. kind부터 production까지 동일 산출물 |
| R2 | **노드 아키텍처 다양성**(amd64/arm64) | 이미지 호환 | 컨테이너를 **multi-arch(amd64+arm64)** 빌드·게시 |
| R3 | **자가 치유가 플랫폼 평면을 건드릴 위험** | 두뇌가 자기 눈을 멀게 함 | 관측/GitOps/정책 스택 대상 조치 **default-deny** (§9 deniedPaths, `touch_platform_components`) |
| R4 | **git write = 사실상 클러스터 제어권** | 광범위 영향 | 토큰을 단일 repo로 스코프 한정, allowedPaths 화이트리스트가 PR 생성보다 먼저 구현 |
| R5 | **알림 폐기 위험** | 신호 미수신 | 배포 후 Alertmanager → KubeSentinel route 연결을 smoke test로 검증 |
| R6 | **LLM 가용성** | 두뇌 단일 장애점 | AIProvider에 fallback 엔드포인트. 내부 LLM 사용 시 헬스체크 + 외부 fallback |
| R7 | **GitOps 컨트롤러 선택**(Argo vs Flux) | 통합 분기 | Argo CD 우선 구현, Flux는 어댑터로 선택 지원. 상태 조회는 추상화 |
| R8 | **데이터 민감도 / 외부 전송** | 보안·규정 | `redactSecrets` 기본 on, `allowExternalModel` 기본 off. 사내 규정에 따라 내부 LLM 강제 가능 |

---

## 11. 착수 전 확정 사항 (배포 파라미터)

깨끗한 클러스터에 설치하기 전, 아래를 values로 확정한다:

1. **KubeSentinel 전용 namespace** + 관측 스택에 대한 **read-only RBAC** 범위
2. **AIProvider** — endpoint/model/secret, 내부 LLM vs frontier, fallback 여부
3. **매니페스트 repo + 토큰** — provider, repo, baseBranch, allowedPaths
4. **알림 채널** — Discord/Slack/Teams webhook URL
5. **GitOps 컨트롤러** — Argo CD / Flux 중 택1
6. **런타임 언어** — Phase A 단일 서비스. 권장 **Go**(in-cluster client·multi-arch·후일 controller-runtime 승격 용이); LLM 호출은 HTTP라 언어 무관

---

## 12. Repo 구조 제안 (개념 6장 계승)

```
kubesentinel-ai/
  cmd/            collector / ai-gateway / webhook-gateway
  internal/
    collector/    prometheus.go loki.go kubernetes.go gitops.go git.go
    diagnosis/    rule_based.go correlation.go llm_rca.go
    policy/       evaluator.go risk.go scope.go
    gitops/       provider.go github.go gitlab.go patch.go pullrequest.go
    notifier/     discord.go slack.go teams.go webhook.go
    audit/        recorder.go
  ai/             prompt_templates/ schemas/ redactor
  charts/kubesentinel-ai/      # Helm 배포 (values로 전 환경 대응)
  deploy/crds/ deploy/samples/ # Phase B
  docs/           architecture.md threat-model.md policy.md runbook-format.md
  examples/scenarios/          crashloop/ oomkilled/ imagepullbackoff/ ...
  tests/          kind/ e2e/   # kind 기반 — CSP 독립 검증
```

---

## 13. 다음 단계

본 문서 승인 후 **MVP-0 골격**부터 구현:
1. KubeSentinel Helm chart 스캐폴드 (values로 엔드포인트 주입)
2. Alertmanager webhook receiver 연동 + smoke test
3. evidence-collector 서비스 (alert 수신 → Prom/Loki/event 수집)
4. 첫 RCA 프롬프트 + OpenAI 호환 LLM 호출
5. 알림 채널 연동

> CRD 전체 초안·controller reconciliation flow는 Phase B에서 작성.
> kind 기반 로컬 e2e로 CSP에 무관한 재현 환경을 함께 구축한다.
