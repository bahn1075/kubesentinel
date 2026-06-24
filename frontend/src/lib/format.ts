import type { IncidentState, Severity, Risk } from "../api/types";

// architecture.md §5 정상 진행 경로 (실패 상태는 별도 표시)
export const STATE_FLOW: IncidentState[] = [
  "IncidentDetected",
  "EvidenceCollected",
  "DiagnosisCompleted",
  "PlanGenerated",
  "PolicyEvaluated",
  "ApprovalPending",
  "GitPRCreated",
  "GitMerged",
  "Synced",
  "Verified",
  "Closed",
];

const FAILURE_STATES = new Set<IncidentState>([
  "PolicyDenied", "ApprovalRejected", "ValidationFailed",
  "GitPatchFailed", "SyncFailed", "VerificationFailed", "SuppressedByCooldown",
]);

export function isFailureState(s: IncidentState): boolean {
  return FAILURE_STATES.has(s);
}

export function severityClass(s: Severity): string {
  return s === "critical" ? "crit" : s === "warning" ? "warn" : "info";
}

export function riskClass(r: Risk): string {
  return r === "high" ? "crit" : r === "medium" ? "warn" : "ok";
}

export function stateClass(s: IncidentState): string {
  if (isFailureState(s)) return "crit";
  if (s === "Verified" || s === "Closed") return "ok";
  if (s === "ApprovalPending") return "warn";
  return "info";
}

export function formatTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleString("ko-KR", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}
