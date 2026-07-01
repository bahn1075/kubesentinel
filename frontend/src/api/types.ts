// 백엔드 도메인 모델 미러 (internal/models + architecture.md §5 상태머신/§9 정책).
// 백엔드 API가 생기면 이 타입에 맞춰 client.ts의 fetch를 채운다.

// architecture.md §5 상태 머신
export type IncidentState =
  | "IncidentDetected"
  | "EvidenceCollected"
  | "DiagnosisCompleted"
  | "PlanGenerated"
  | "PolicyEvaluated"
  | "ApprovalPending"
  | "GitPRCreated"
  | "GitMerged"
  | "Synced"
  | "Verified"
  | "Closed"
  // 실패 상태
  | "PolicyDenied"
  | "ApprovalRejected"
  | "ValidationFailed"
  | "GitPatchFailed"
  | "SyncFailed"
  | "VerificationFailed"
  | "SuppressedByCooldown";

export type Risk = "low" | "medium" | "high";
export type Severity = "info" | "warning" | "critical";

export interface ProposedAction {
  type: string; // suggestion | git_pr | rollback | runtime_patch ...
  description: string;
  target: string;
  risk: Risk;
}

export interface DiagnosisResult {
  rootCause: string;
  summary: string;
  confidence: number; // 0..1
  proposedActions: ProposedAction[];
  evidenceQuality?: string; // none | partial | rich (코드 계산)
}

export interface RelatedAlert {
  alertname: string;
  namespace: string;
  severity: string;
  summary: string;
}

export interface EvidenceBundle {
  metrics: { name: string; query: string; samples: unknown[] }[];
  logs: string[];
  events: string[];
  resourceStatus?: Record<string, unknown>;
  gitContext?: { repo: string; path: string; lastCommit: string };
  relatedAlerts?: RelatedAlert[];
}

export interface Incident {
  incidentId: string;
  alert: string;
  namespace: string;
  workload: string;
  pod?: string;
  severity: Severity;
  state: IncidentState;
  createdAt: string; // ISO
  diagnosis?: DiagnosisResult;
  evidence?: EvidenceBundle;
  rule?: { category: string; rationale?: string; signals?: string[] };
  prUrl?: string;
}

// architecture.md §9 RemediationPolicy (요약 뷰)
export interface RemediationPolicy {
  name: string;
  mode: string; // pull-request | auto-merge ...
  allowedPaths: string[];
  deniedPaths: string[];
  allowedActions: string[];
  approvalRequiredFor: string[];
  minConfidenceForPR: number;
}

// architecture.md §4.2 AIProvider / §4.7 Notifier / §4.5 GitOps (설정 뷰)
export interface ProviderSettings {
  // 비민감 설정만 여기에. 민감정보(AI api key, git token)는 /api/secrets(write-only)로 분리.
  ai: {
    kind: string;          // frontier | local
    provider: string;      // (frontier) openai | anthropic | azure-openai | google | custom
    type: string;          // API 형식 (openai-compatible)
    endpoint: string;
    model: string;
    authMethod: string;    // (frontier) api-key | oauth | machine
    allowExternal: boolean;
    redactSecrets: boolean;
  };
  collector: { prometheusUrl: string; lokiUrl: string; alertmanagerUrl: string; grafanaUrl: string };
  notifier: { type: string };
  git: { provider: string; authMethod: string; repository: string; baseBranch: string };
}
