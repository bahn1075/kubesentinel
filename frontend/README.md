# KubeSentinel AI — Frontend

Operator 대시보드 (초기 스캐폴드). React + Vite + TypeScript.

> 화면 구성은 향후 기능(인시던트 RCA·제안 조치·승인·정책·설정)을 반영한다.
> **백엔드 조회 API가 아직 없어 기본은 mock 모드**로 동작한다.

## 개발

```bash
npm install
npm run dev        # http://localhost:5173 (/api 는 localhost:8080 백엔드로 프록시)
npm run build      # tsc + vite build → dist/
```

## 환경 변수 (빌드 타임)

| 변수 | 기본값 | 설명 |
|---|---|---|
| `VITE_USE_MOCK` | `true` | `false`로 두면 mock 대신 실제 `/api` 호출 |
| `VITE_API_BASE_URL` | `/api` | 백엔드 조회 API base 경로 |

## 빌드/배포

- 이미지: `frontend/Dockerfile` (node 빌드 → nginx-unprivileged, 8080 리슨). **백엔드와 별도 빌드.**
  ```bash
  make frontend-docker-push REGISTRY=ghcr.io/your-org TAG=v0.1.0
  ```
- 배포: 루트 Helm 차트에 포함(`frontend.enabled=true`). UI/`/api` 라우팅은 `ingress.enabled=true`.

## 구조

```
src/
  api/        types.ts(도메인 모델) · mock.ts(예시 데이터) · client.ts(fetch, mock 분기)
  lib/        format.ts(상태/뱃지/시간) · useAsync.ts(로딩 훅)
  components/  Layout.tsx(사이드바)
  pages/       Dashboard · Incidents · IncidentDetail · Approvals · Policies · Settings
```

백엔드 API 연동 시: `client.ts`의 `getJSON` 분기를 채우고 `VITE_USE_MOCK=false`.
응답 스키마는 `src/api/types.ts` ↔ 백엔드 `internal/models` 와 맞춘다.
