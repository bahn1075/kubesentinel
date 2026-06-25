# KubeSentinel AI

**한국어** · [English](README.en.md)

> Prometheus/Alertmanager·Loki에서 장애 신호를 수집하고, OpenAI 호환 LLM으로 원인을 분석한 뒤,
> 정책으로 허용된 범위 안에서 GitOps PR을 만들고 알림 채널로 승인 요청을 보내는 **Kubernetes 자동 진단·조치(remediation) 시스템**.

![status](https://img.shields.io/badge/status-MVP-yellow) ![go](https://img.shields.io/badge/Go-1.26-00ADD8) ![react](https://img.shields.io/badge/React-19-61DAFB) ![helm](https://img.shields.io/badge/Helm-chart-0F1689) ![license](https://img.shields.io/badge/license-TBD-lightgrey)

---

## 목차
- [핵심 설계 원칙](#핵심-설계-원칙)
- [작동 기전 (How it works)](#작동-기전-how-it-works)
- [아키텍처](#아키텍처)
- [구성 요소](#구성-요소)
- [기능 현황](#기능-현황)
- [설치](#설치)
  - [1. 로컬 (docker compose)](#1-로컬-docker-compose)
  - [2. minikube (metallb)](#2-minikube-metallb)
  - [3. CSP / OKE (Helm + ArgoCD)](#3-csp--oke-helm--argocd)
- [설정 (Settings)](#설정-settings)
- [환경별 노출 모드](#환경별-노출-모드)
- [리포지터리 구조](#리포지터리-구조)
- [효과](#효과)
- [로드맵](#로드맵)

---

## 핵심 설계 원칙

1. **AI는 판단자가 아니라 제안자다.** 허용 범위·위험도·승인·적용 방식은 *시스템*이 결정한다.
2. **GitOps가 1순위 조치 경로다.** runtime patch는 예외이며 사후 Git 반영을 강제한다.
3. **근거 없는 조치 금지.** evidence 없으면 write 없음, 신뢰도 낮으면 제안만, production write는 승인 필수.
4. **변경 범위는 명시적으로 제한된다.** repo/branch/path/kind/namespace/action/risk 단위 화이트리스트.
5. **자기 자신과 관측·제어 평면은 건드리지 않는다.** 관측 스택·GitOps·정책 엔진·KubeSentinel 자신은 default-deny.
6. **CSP 중립.** 서비스 주소·모델명·repo는 코드에 하드코딩하지 않고 전부 값(Settings/Helm)으로 주입한다.

---

## 작동 기전 (How it works)

```
Alertmanager ──(webhook /v1/alerts)──▶ ① Signal Collector
                                          │  alert 파싱 + Prometheus/Loki 보강(best-effort)
                                          ▼
                                       ② Diagnosis Engine
                                          │  EvidenceBundle → OpenAI 호환 LLM → 구조화 RCA
                                          │  (root cause / summary / confidence / proposed actions)
                                          ▼
                                       ③ 영속화 + 알림
                                          │  Incident를 PostgreSQL에 저장
                                          ▼
                                       ④ Notifier → Discord / Slack / Teams
                                          (root cause · 제안 조치 · 딥링크)
                                          ▼
                                       ⑤ (MVP-1) Policy → GitOps PR → 승인 → Argo CD sync → 검증
```

1. **수집** — Alertmanager가 webhook(`/v1/alerts`)으로 alert를 보내면, 대상 워크로드/네임스페이스를 식별하고 Prometheus 메트릭·Loki 로그로 근거(EvidenceBundle)를 보강한다. (관측 소스 미설정 시 자동 skip)
2. **진단** — EvidenceBundle을 OpenAI 호환 LLM에 보내 **구조화된 RCA**(근본 원인·요약·신뢰도·제안 조치 목록)를 얻는다. 응답 형식이 흔들려도 견디는 관대한 파서를 사용한다.
3. **영속화** — 인시던트를 PostgreSQL에 저장해 대시보드에서 조회한다.
4. **알림** — 진단 결과를 알림 채널로 전송(제안 조치는 "제안일 뿐, 적용은 정책·승인 후").
5. **조치(예정)** — 정책 범위 안에서 GitOps PR을 생성하고 승인 후 Argo CD가 반영, metric/log로 검증.

> **설정은 매니페스트가 아니라 DB로 관리한다.** 프론트엔드(대시보드)에서 입력한 값을 PostgreSQL에 저장하고, 백엔드는 기동 시 이를 로드해 동작한다. 민감정보(LLM API key·git token)는 write-only로 저장되어 값이 다시 노출되지 않는다.

---

## 아키텍처

```
┌────────────────────────────────────────────────────────────────┐
│ Observability (값으로 주입)  Prometheus · Alertmanager · Loki   │
└───────────────────────────────┬────────────────────────────────┘
                                 │ Alertmanager webhook
                                 ▼
┌────────────────────────────────────────────────────────────────┐
│ KubeSentinel AI                                                 │
│   Backend(Go): Collector → Diagnosis(LLM) → Store(PG) → Notifier│
│   Frontend(React): Dashboard · Incidents · Settings             │
│   설정/시크릿/인시던트 ← PostgreSQL                              │
└───────────────────────────────┬────────────────────────────────┘
                                 │ (MVP-1) git PR
                                 ▼
                       Argo CD / Flux → cluster
```

자세한 설계: [`docs/architecture.md`](docs/architecture.md) · 구현 현황: [`docs/implementation-status.md`](docs/implementation-status.md)

---

## 구성 요소

| 영역 | 기술 | 설명 |
|---|---|---|
| Backend | Go 1.26 (표준 net/http, pgx, goose) | webhook 수신, 근거 수집, LLM 진단, 영속화, 알림, 설정/시크릿 API |
| Frontend | React 19 · Vite · TypeScript | 운영 대시보드 (Dashboard/Incidents/Settings). nginx가 `/api`를 백엔드로 프록시 |
| DB | PostgreSQL | 설정·시크릿·인시던트 영속화. 스키마는 goose 임베드 마이그레이션으로 관리 |
| LLM | OpenAI 호환 엔드포인트 | LM Studio · Ollama · vLLM · OpenAI · Anthropic 등 (로컬/프론티어 선택) |
| Notifier | Discord / Slack / Teams webhook | 진단 결과 알림 |
| 배포 | Docker(멀티아치) · Helm · ArgoCD | 단일 차트로 환경별(노출 모드) 배포 |

---

## 기능 현황

| 기능 | 상태 |
|---|---|
| Alertmanager webhook 수신 → RCA → 알림 (MVP-0) | ✅ |
| Prometheus/Loki 근거 보강 (best-effort) | ✅ |
| OpenAI 호환 LLM 진단 (로컬/프론티어) + 모델 조회 | ✅ |
| 인시던트 PostgreSQL 영속화 + 대시보드 조회 | ✅ |
| 설정 DB화 (프론트 입력 → DB → 기동 시 로드) | ✅ |
| 민감정보 write-only 시크릿 | ✅ |
| 환경별 노출(ingress-nginx/metallb/tailscale) | ✅ |
| Helm + ArgoCD 배포 | ✅ |
| GitOps PR 자동 생성 (MVP-1) | ⏳ 예정 |
| 승인 기반 적용 / 정책 엔진 (MVP-2) | ⏳ 예정 |
| Kubernetes Events·manifest 수집 (client-go) | ⏳ 예정 |

---

## 설치

### 사전 요구
- Kubernetes ≥ 1.27 (또는 로컬은 Docker/`docker compose`)
- 관측 스택(Prometheus/Loki/Alertmanager)과 OpenAI 호환 LLM은 **선택적으로** 연결 (없어도 기동됨)

### 1. 로컬 (docker compose)
외부 의존성 없이 전체 흐름을 검증한다. `mock-llm`(고정 RCA) + `notify-sink`(알림 로그) + `postgres` 포함.
```bash
docker compose up --build
# 다른 터미널에서 alert 주입
curl -X POST localhost:8080/v1/alerts -H 'Content-Type: application/json' \
  --data @deploy/local/sample-alert.json
docker compose logs -f backend        # 진단 결과
docker compose logs -f notify-sink    # 전송된 알림
# 대시보드: http://localhost:8081
```

### 2. minikube (metallb)
```bash
helm install kubesentinel charts/kubesentinel-ai -n kubesentinel --create-namespace \
  -f charts/kubesentinel-ai/values.yaml \
  -f charts/kubesentinel-ai/values/metallb.yaml
# 대시보드
kubectl -n kubesentinel port-forward svc/kubesentinel-kubesentinel-ai-frontend 8080:80
#  → http://localhost:8080
```

### 3. CSP / OKE (Helm + ArgoCD)
ArgoCD Application으로 GitOps 배포 (tailscale 노출 예시):
```bash
kubectl apply -n argocd -f deploy/argocd/application-oke-tailscale.yaml
```
외부 PostgreSQL을 쓰는 경우 DSN을 Secret으로 주입:
```bash
kubectl -n kubesentinel create secret generic kubesentinel-db-url \
  --from-literal=url='postgres://user:pass@postgresql.postgres.svc:5432/kubesentinel?sslmode=disable'
# values/tailscale.yaml: postgres.enabled=false, database.existingSecret=kubesentinel-db-url
```

이미지 빌드/푸시(멀티아치):
```bash
./scripts/docker-build-push.sh <dockerhub-id>                                  # backend
DOCKERFILE=frontend/Dockerfile CONTEXT=frontend REPO=kubesentinel-ai-front \
  ./scripts/docker-build-push.sh <dockerhub-id>                                # frontend
```

---

## 설정 (Settings)

대시보드 **Settings** 화면에서 입력 → DB 저장 → 백엔드 기동 시 로드. (비밀은 write-only)

| 섹션 | 항목 |
|---|---|
| **AI Provider** | 종류(로컬/프론티어), 제공자, Endpoint, Model(**상태 확인**으로 조회 후 선택), 인증 방식, API Key(write-only) |
| **Collector** | Prometheus / Loki / Alertmanager / Grafana URL |
| **Notifier** | 채널 종류 (slack/discord/teams) |
| **Git** | 제공자(github/gitlab/gitea) · 인증 방식 · repo · branch · token(write-only) |

> 설정 변경은 백엔드 재시작 시 반영된다(기동 시 1회 병합). 런타임 hot-reload는 후속 과제.

주요 환경변수(부팅 시 필수 최소값): `KUBESENTINEL_AI_ENDPOINT`, `KUBESENTINEL_AI_DATABASE_URL`. 그 외 비민감 값은 Settings(DB)가 우선한다.

---

## 환경별 노출 모드

`expose.mode` 한 값으로 노출 방식을 전환한다 (노출 대상 = frontend, nginx가 `/api`를 백엔드로 프록시).

| mode | 환경 | 동작 |
|---|---|---|
| `ingress-nginx` | 일반 CSP k8s | Ingress(class=nginx) + CSP LoadBalancer |
| `metallb` | minikube/온프렘 | Service `type=LoadBalancer` (metallb가 IP 할당) |
| `tailscale` | OKE(tailscale operator) | Ingress(class=tailscale) → `<name>.<tailnet>.ts.net` HTTPS |

환경별 오버레이: `charts/kubesentinel-ai/values/{ingress,metallb,tailscale}.yaml`

---

## 리포지터리 구조

```
kubesentinel/
├── cmd/kubesentinel-ai/        # 진입점
├── internal/
│   ├── config/                 # 설정 로드 (env + DB 병합)
│   ├── collector/              # webhook 수신 · prometheus/loki 보강 · settings/secrets/incidents API
│   ├── diagnosis/              # LLM RCA 엔진
│   ├── provider/               # OpenAI 호환 AI Gateway
│   ├── notifier/               # discord/slack/teams
│   ├── store/                  # PostgreSQL + goose 마이그레이션
│   └── models/                 # 도메인 모델
├── frontend/                   # React+Vite 대시보드 (별도 이미지)
├── charts/kubesentinel-ai/     # Helm 차트 (+ values/ 환경별 오버레이)
├── deploy/                     # argocd Application · 로컬 mock 스택
├── scripts/                    # docker build/push (멀티아치)
├── docker-compose.yml          # 로컬 통합 테스트
└── docs/                       # architecture.md · implementation-status.md
```

---

## 효과

- **MTTR 단축** — alert 발생 즉시 LLM이 근거 기반 원인 분석과 조치 후보를 제시해 1차 분류 시간을 줄인다.
- **안전한 자동화** — AI는 "제안자"일 뿐, 적용은 정책·승인·GitOps를 거쳐 **모든 조치가 git history + PR + sync 로그로 감사**된다.
- **환경 이식성** — 동일 산출물이 kind·minikube·온프렘·OKE/EKS/GKE에서 값만 바꿔 동작(CSP 중립).
- **설정 일원화** — 운영 설정을 매니페스트 산재가 아닌 DB로 관리, 대시보드에서 변경.
- **관측 자산 재사용** — 이미 갖춘 Prometheus/Loki/Alertmanager를 그대로 신호원으로 활용.

---

## 로드맵

- **MVP-0** Read-only RCA + 알림 — ✅
- **MVP-1** GitOps PR 생성 (정책 화이트리스트 + provider 추상화) — ⏳
- **MVP-2** 승인 기반 적용 + sync/검증 — ⏳
- **MVP-3** 제한적 자동 조치 (dev/test, 낮은 위험, cooldown/rate-limit) — ⏳
- 그 외: Kubernetes Events·manifest 수집(client-go), 설정 hot-reload, OAuth 인증 플로우, Runbook RAG

---

> ⚠️ 본 프로젝트는 MVP 단계다. 프로덕션 적용 전 보안(시크릿 암호화·RBAC 최소화)과 정책 가드를 반드시 검토하라.
