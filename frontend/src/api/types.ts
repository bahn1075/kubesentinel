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
}

export interface EvidenceBundle {
  metrics: { name: string; query: string; samples: unknown[] }[];
  logs: string[];
  events: string[];
  gitContext?: { repo: string; path: string; lastCommit: string };
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
  // 주의: 비민감 설정만 DB에 저장된다. 민감정보(AI apiKey, notifier webhook, git token)는
  // k8s Secret/env로 관리하며 이 타입/Settings API에 포함되지 않는다. (architecture.md R8)
  ai: { type: string; endpoint: string; model: string; allowExternal: boolean; redactSecrets: boolean };
  collector: { prometheusUrl: string; lokiUrl: string; grafanaUrl: string };
  notifier: { type: string };
  gitops: { provider: string; repository: string; baseBranch: string };
}
