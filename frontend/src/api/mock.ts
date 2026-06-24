import type { Incident, RemediationPolicy, ProviderSettings } from "./types";

// 백엔드 API가 없는 초기 단계용 mock 데이터.
// 미래 기능(RCA·제안·승인·정책)을 화면에서 확인할 수 있도록 다양한 상태를 포함한다.

export const mockIncidents: Incident[] = [
  {
    incidentId: "inc-20260623-KubePodCrashLooping",
    alert: "KubePodCrashLooping",
    namespace: "production",
    workload: "api-server",
    pod: "api-server-7d9f-abcde",
    severity: "critical",
    state: "ApprovalPending",
    createdAt: "2026-06-23T01:12:00Z",
    diagnosis: {
      rootCause: "최근 배포(v1.8.2)에서 환경변수 DB_HOST 누락으로 기동 직후 패닉.",
      summary:
        "deploy 직후 CrashLoopBackOff. previous 로그에 'missing DB_HOST' 패닉. 이전 revision은 정상.",
      confidence: 0.88,
      proposedActions: [
        { type: "git_pr", description: "values.yaml에 DB_HOST 환경변수 복구", target: "apps/api-server/values.yaml", risk: "medium" },
        { type: "rollback", description: "직전 정상 revision으로 롤백", target: "deployment/api-server", risk: "low" },
      ],
    },
    evidence: {
      metrics: [{ name: "restarts", query: 'kube_pod_container_status_restarts_total{...}', samples: [] }],
      logs: ["panic: missing DB_HOST", "goroutine 1 [running]:", "main.main()"],
      events: ["BackOff restarting failed container", "Liveness probe failed"],
      gitContext: { repo: "your-org/manifests", path: "apps/api-server", lastCommit: "a1b2c3d" },
    },
  },
  {
    incidentId: "inc-20260623-OOMKilled",
    alert: "OOMKilled",
    namespace: "production",
    workload: "search-indexer",
    pod: "search-indexer-0",
    severity: "critical",
    state: "GitPRCreated",
    createdAt: "2026-06-23T00:40:00Z",
    prUrl: "https://github.com/your-org/manifests/pull/142",
    diagnosis: {
      rootCause: "memory limit(512Mi)이 워킹셋(peak 730Mi) 대비 부족.",
      summary: "OOMKill 3회. p95 working set이 limit 초과. limit 상향 PR 생성됨.",
      confidence: 0.81,
      proposedActions: [
        { type: "git_pr", description: "memory limit 512Mi → 1Gi 상향", target: "apps/search-indexer/values.yaml", risk: "medium" },
      ],
    },
  },
  {
    incidentId: "inc-20260622-ServiceSelectorMismatch",
    alert: "ServiceSelectorMismatch",
    namespace: "staging",
    workload: "checkout",
    severity: "warning",
    state: "Verified",
    createdAt: "2026-06-22T18:05:00Z",
    diagnosis: {
      rootCause: "Service selector(app=checkout-v2)와 Pod label(app=checkout) 불일치.",
      summary: "낮은 위험 자동 조치로 selector 정정. sync 후 endpoint 회복 확인.",
      confidence: 0.95,
      proposedActions: [
        { type: "git_pr", description: "Service selector를 app=checkout로 정정", target: "apps/checkout/service.yaml", risk: "low" },
      ],
    },
  },
  {
    incidentId: "inc-20260622-ImagePullBackOff",
    alert: "ImagePullBackOff",
    namespace: "production",
    workload: "notify-worker",
    severity: "warning",
    state: "DiagnosisCompleted",
    createdAt: "2026-06-22T16:20:00Z",
    diagnosis: {
      rootCause: "이미지 태그 오타(v2.3.1 → v2.31)로 registry에 매니페스트 없음.",
      summary: "존재하지 않는 태그. 태그 정정 PR 제안. 신뢰도 임계값 충족 대기.",
      confidence: 0.72,
      proposedActions: [
        { type: "git_pr", description: "image tag v2.31 → v2.3.1 정정", target: "apps/notify-worker/values.yaml", risk: "low" },
      ],
    },
  },
];

export const mockPolicies: RemediationPolicy[] = [
  {
    name: "default-safe-policy",
    mode: "pull-request",
    allowedPaths: ["apps/<app>/manifests/", "apps/<app>/values*.yaml"],
    deniedPaths: ["**/argocd/**", "**/kube-prometheus-stack/**", "**/secrets/**", "**/rbac/**", "**/*app-of-apps*"],
    allowedActions: ["collect_evidence", "analyze", "create_pull_request", "notify"],
    approvalRequiredFor: ["production", "resource_limit_change", "image_change", "rollback"],
    minConfidenceForPR: 0.75,
  },
];

export const mockSettings: ProviderSettings = {
  ai: { type: "openai-compatible", endpoint: "", model: "", allowExternal: false, redactSecrets: true },
  collector: { prometheusUrl: "", lokiUrl: "", grafanaUrl: "" },
  notifier: { type: "slack" },
  gitops: { provider: "github", repository: "your-org/manifests", baseBranch: "main" },
};
