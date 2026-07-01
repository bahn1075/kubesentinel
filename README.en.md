# KubeSentinel AI

[한국어](README.md) · **English**

> A **Kubernetes auto-diagnosis & remediation system** that collects failure signals from Prometheus/Alertmanager·Loki,
> analyzes root cause with an OpenAI-compatible LLM, then — within policy-allowed scope — opens a GitOps PR and
> requests approval through a notification channel.

![status](https://img.shields.io/badge/status-MVP-yellow) ![go](https://img.shields.io/badge/Go-1.26-00ADD8) ![react](https://img.shields.io/badge/React-19-61DAFB) ![helm](https://img.shields.io/badge/Helm-chart-0F1689) ![license](https://img.shields.io/badge/license-TBD-lightgrey)

---

## Table of Contents
- [Design Principles](#design-principles)
- [How It Works](#how-it-works)
- [Architecture](#architecture)
- [Components](#components)
- [Feature Status](#feature-status)
- [Installation](#installation)
  - [1. Local (docker compose)](#1-local-docker-compose)
  - [2. minikube (metallb)](#2-minikube-metallb)
  - [3. CSP / OKE (Helm + ArgoCD)](#3-csp--oke-helm--argocd)
- [Settings](#settings)
- [Exposure Modes](#exposure-modes)
- [Repository Layout](#repository-layout)
- [Benefits](#benefits)
- [Roadmap](#roadmap)

---

## Design Principles

1. **AI is a proposer, not a decider.** Scope, risk, approval, and apply-mode are decided by the *system*, not the model.
2. **GitOps is the primary remediation path.** Runtime patches are the exception and must be reflected back to Git.
3. **No action without evidence.** No evidence → no write; low confidence → suggestion only; production write → approval required.
4. **Change scope is explicitly bounded** by repo/branch/path/kind/namespace/action/risk whitelists.
5. **Never touch itself or the control/observability plane.** Observability stack, GitOps, policy engine, and KubeSentinel itself are default-deny.
6. **CSP-neutral.** Service addresses, model names, and repos are never hardcoded — all injected via Settings/Helm values.

---

## How It Works

```
Alertmanager ──(webhook /v1/alerts)──▶ ① Signal Collector
                                          │  parse alert + enrich with Prometheus/Loki (best-effort)
                                          ▼
                                       ② Diagnosis Engine
                                          │  EvidenceBundle → OpenAI-compatible LLM → structured RCA
                                          │  (root cause / summary / confidence / proposed actions)
                                          ▼
                                       ③ Persist + Notify
                                          │  store Incident in PostgreSQL
                                          ▼
                                       ④ Notifier → Discord / Slack / Teams
                                          (root cause · proposed actions · deep links)
                                          ▼
                                       ⑤ (MVP-1) Policy → GitOps PR → approval → Argo CD sync → verify
```

1. **Collect** — On receiving an alert, KubeSentinel identifies the target workload/namespace and enriches an `EvidenceBundle` with Prometheus metrics and Loki logs (skipped if not configured). Two intake modes: **push** (Alertmanager posts to `/v1/alerts`) or **pull** (polls `GET /api/v2/alerts` using the Alertmanager URL from Settings — **no Prometheus/Alertmanager config changes**).
2. **Diagnose (deep analysis)** — The EvidenceBundle is sent to an OpenAI-compatible LLM for a **structured RCA** (root cause, summary, confidence, proposed actions). Not a single guess: it applies **correlation of co-firing alerts + confidence gating when evidence is thin (L1)**, **client-go collection of Events/resource/node status (L2)**, and **an agentic loop where the LLM requests read-only tools → re-analyzes + a verification pass (L3)**. (A tolerant parser handles local-model output drift.)
3. **Persist** — The incident is stored in PostgreSQL for dashboard retrieval.
4. **Notify** — The result is pushed to a notification channel (proposed actions are labeled "suggestions only — apply after policy/approval").
5. **Remediate (planned)** — Within policy scope, open a GitOps PR; after approval Argo CD reconciles it and the outcome is verified via metrics/logs.

> **Settings are managed in the DB, not in manifests.** Values entered in the dashboard are stored in PostgreSQL and loaded by the backend on startup. Sensitive values (LLM API key, git token) are stored **write-only** — never returned by the API.

---

## Architecture

```
┌────────────────────────────────────────────────────────────────┐
│ Observability (injected)   Prometheus · Alertmanager · Loki     │
└───────────────────────────────┬────────────────────────────────┘
                                 │ Alertmanager webhook
                                 ▼
┌────────────────────────────────────────────────────────────────┐
│ KubeSentinel AI                                                 │
│   Backend(Go): Collector → Diagnosis(LLM) → Store(PG) → Notifier│
│   Frontend(React): Dashboard · Incidents · Settings             │
│   settings/secrets/incidents ← PostgreSQL                       │
└───────────────────────────────┬────────────────────────────────┘
                                 │ (MVP-1) git PR
                                 ▼
                       Argo CD / Flux → cluster
```

Design details: [`docs/architecture.md`](docs/architecture.md) · Implementation status: [`docs/implementation-status.md`](docs/implementation-status.md)

---

## Components

| Area | Stack | Description |
|---|---|---|
| Backend | Go 1.26 (stdlib net/http, pgx, goose) | webhook intake, evidence collection, LLM diagnosis, persistence, notifications, settings/secrets API |
| Frontend | React 19 · Vite · TypeScript | operator dashboard (Dashboard/Incidents/Settings). nginx proxies `/api` to the backend |
| DB | PostgreSQL | persists settings, secrets, incidents. Schema managed via embedded goose migrations |
| LLM | OpenAI-compatible endpoint | LM Studio · Ollama · vLLM · OpenAI · Anthropic, etc. (local/frontier selectable) |
| Notifier | Discord / Slack / Teams webhook | diagnosis result notifications |
| Deploy | Docker (multi-arch) · Helm · ArgoCD | single chart, per-environment exposure modes |

---

## Feature Status

| Feature | Status |
|---|---|
| Alertmanager webhook (push) → RCA → notify (MVP-0) | ✅ |
| Alertmanager API polling (pull) — no Prometheus config changes | ✅ |
| Prometheus/Loki evidence enrichment (best-effort) | ✅ |
| OpenAI-compatible LLM diagnosis (local/frontier) + model discovery | ✅ |
| Deep analysis: correlation & confidence gating (L1) · client-go evidence (L2) · agentic tool loop + verify (L3) | ✅ |
| Incident persistence in PostgreSQL + dashboard | ✅ |
| DB-backed settings (frontend → DB → load on startup) | ✅ |
| Write-only secrets | ✅ |
| Per-env exposure (ingress-nginx/metallb/tailscale) | ✅ |
| Helm + ArgoCD deployment | ✅ |
| Automated GitOps PR (MVP-1) | ⏳ planned |
| Approval-gated apply / policy engine (MVP-2) | ⏳ planned |
| Kubernetes Events·manifest collection (client-go) | ⏳ planned |

---

## Installation

### Prerequisites
- Kubernetes ≥ 1.27 (or Docker / `docker compose` for local)
- Observability stack (Prometheus/Loki/Alertmanager) and an OpenAI-compatible LLM are **optional** to connect (the app boots without them)

### 1. Local (docker compose)
Validates the full flow with no external dependencies — includes `mock-llm` (fixed RCA) + `notify-sink` (logs notifications) + `postgres`.
```bash
docker compose up --build
# in another terminal, inject an alert
curl -X POST localhost:8080/v1/alerts -H 'Content-Type: application/json' \
  --data @deploy/local/sample-alert.json
docker compose logs -f backend        # diagnosis result
docker compose logs -f notify-sink    # delivered notification
# dashboard: http://localhost:8081
```

### 2. minikube (metallb)
```bash
helm install kubesentinel helm/kubesentinel-ai -n kubesentinel --create-namespace \
  -f helm/kubesentinel-ai/values.yaml \
  -f helm/kubesentinel-ai/values/metallb.yaml
# dashboard
kubectl -n kubesentinel port-forward svc/kubesentinel-kubesentinel-ai-frontend 8080:80
#  → http://localhost:8080
```

### 3. CSP / OKE (Helm + ArgoCD)
GitOps deploy via an ArgoCD Application (tailscale exposure example):
```bash
kubectl apply -n argocd -f deploy/argocd/application-oke-tailscale.yaml
```
To use an external PostgreSQL, inject the DSN via a Secret:
```bash
kubectl -n kubesentinel create secret generic kubesentinel-db-url \
  --from-literal=url='postgres://user:pass@postgresql.postgres.svc:5432/kubesentinel?sslmode=disable'
# values/tailscale.yaml: postgres.enabled=false, database.existingSecret=kubesentinel-db-url
```

Build & push images (multi-arch):
```bash
./scripts/docker-build-push.sh <dockerhub-id>                                  # backend
DOCKERFILE=frontend/Dockerfile CONTEXT=frontend REPO=kubesentinel-ai-front \
  ./scripts/docker-build-push.sh <dockerhub-id>                                # frontend
```

---

## Settings

Enter in the dashboard **Settings** page → stored in DB → loaded on backend startup. (secrets are write-only)

| Section | Fields |
|---|---|
| **AI Provider** | kind (local/frontier), provider, Endpoint, Model (discover via **Check Status**), auth method, API Key (write-only) |
| **Collector** | Prometheus / Loki / Alertmanager / Grafana URL |
| **Notifier** | channel type (slack/discord/teams) |
| **Git** | provider (github/gitlab/gitea) · auth method · repo · branch · token (write-only) |

> Settings take effect on backend restart (merged once at startup). Runtime hot-reload is a planned follow-up.

Minimum boot-time env vars: `KUBESENTINEL_AI_ENDPOINT`, `KUBESENTINEL_AI_DATABASE_URL`. Other non-sensitive values are overridden by Settings (DB).

---

## Exposure Modes

A single `expose.mode` switches how the app is exposed (target = frontend; nginx proxies `/api` to the backend).

| mode | environment | behavior |
|---|---|---|
| `ingress-nginx` | general CSP k8s | Ingress (class=nginx) + CSP LoadBalancer |
| `metallb` | minikube/on-prem | Service `type=LoadBalancer` (metallb assigns IP) |
| `tailscale` | OKE (tailscale operator) | Ingress (class=tailscale) → `<name>.<tailnet>.ts.net` HTTPS |

Per-env overlays: `helm/kubesentinel-ai/values/{ingress,metallb,tailscale}.yaml`

---

## Repository Layout

```
kubesentinel/
├── cmd/kubesentinel-ai/        # entrypoint
├── internal/
│   ├── config/                 # config load (env + DB merge)
│   ├── collector/              # webhook intake · prometheus/loki enrich · settings/secrets/incidents API
│   ├── diagnosis/              # LLM RCA engine
│   ├── provider/               # OpenAI-compatible AI Gateway
│   ├── notifier/               # discord/slack/teams
│   ├── store/                  # PostgreSQL + goose migrations
│   └── models/                 # domain models
├── frontend/                   # React+Vite dashboard (separate image)
├── helm/kubesentinel-ai/     # Helm chart (+ values/ per-env overlays)
├── deploy/                     # argocd Application · local mock stack
├── scripts/                    # docker build/push (multi-arch)
├── docker-compose.yml          # local integration test
└── docs/                       # architecture.md · implementation-status.md
```

---

## Benefits

- **Faster MTTR** — the LLM proposes evidence-based root cause and remediation candidates the moment an alert fires, cutting first-triage time.
- **Safe automation** — AI only *proposes*; application goes through policy/approval/GitOps, so **every action is auditable via git history + PR + sync logs**.
- **Portability** — the same artifact runs on kind·minikube·on-prem·OKE/EKS/GKE by changing values only (CSP-neutral).
- **Unified configuration** — operational settings live in the DB and are edited from the dashboard, not scattered across manifests.
- **Reuse of observability assets** — leverages your existing Prometheus/Loki/Alertmanager as signal sources.

---

## Roadmap

- **MVP-0** Read-only RCA + notify — ✅
- **MVP-1** GitOps PR creation (policy whitelist + provider abstraction) — ⏳
- **MVP-2** Approval-gated apply + sync/verify — ⏳
- **MVP-3** Limited auto-remediation (dev/test, low risk, cooldown/rate-limit) — ⏳
- Others: Kubernetes Events·manifest collection (client-go), settings hot-reload, OAuth auth flows, Runbook RAG

---

> ⚠️ This project is at MVP stage. Before production use, review security (secret encryption, least-privilege RBAC) and policy guards.
