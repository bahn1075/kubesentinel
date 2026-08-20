import type { Incident, RemediationPolicy, ProviderSettings, IgnoreRule, IgnoreList } from "./types";
import { mockIncidents, mockPolicies, mockSettings } from "./mock";

// 백엔드 API가 아직 없으므로 기본은 MOCK 모드.
// 백엔드에 조회 API(예: GET /api/incidents)가 생기면 VITE_USE_MOCK=false 로 두고
// 아래 fetch 분기를 채우면 된다. (현재는 read-only RCA + 알림 단계)
const USE_MOCK = (import.meta.env.VITE_USE_MOCK ?? "true") !== "false";
const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "/api";

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`);
  if (!res.ok) throw new Error(`API ${path} → ${res.status}`);
  return res.json() as Promise<T>;
}

async function putJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`API ${path} → ${res.status}`);
  return res.json() as Promise<T>;
}

async function sendJSON<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw new Error(`API ${method} ${path} → ${res.status}`);
  return (res.status === 204 ? undefined : res.json()) as Promise<T>;
}

// 인시던트 확인됨(acknowledged) 처리 — 확인됨이면 목록에서 숨겨진다.
export async function acknowledgeIncident(id: string, acknowledged = true): Promise<void> {
  await sendJSON("PATCH", `/incidents/${encodeURIComponent(id)}`, { acknowledged });
}

// ── 무시 규칙 (Ignore rules) ──
export async function fetchIgnores(): Promise<IgnoreList> {
  return getJSON<IgnoreList>("/ignores");
}
export async function addIgnore(keyword: string): Promise<IgnoreRule> {
  return sendJSON<IgnoreRule>("POST", "/ignores", { keyword });
}
export async function setIgnoreEnabled(id: number, enabled: boolean): Promise<void> {
  await sendJSON("PATCH", `/ignores/${id}`, { enabled });
}
export async function deleteIgnore(id: number): Promise<void> {
  await sendJSON("DELETE", `/ignores/${id}`);
}

// Incidents는 백엔드(DB)에서 조회한다. 백엔드가 없으면 mock으로 폴백(dev 편의).
export async function fetchIncidents(): Promise<Incident[]> {
  try {
    return await getJSON<Incident[]>("/incidents");
  } catch {
    return mockIncidents;
  }
}

export async function fetchIncident(id: string): Promise<Incident | undefined> {
  try {
    return await getJSON<Incident>(`/incidents/${encodeURIComponent(id)}`);
  } catch {
    return mockIncidents.find((i) => i.incidentId === id);
  }
}

export async function fetchPolicies(): Promise<RemediationPolicy[]> {
  if (USE_MOCK) return Promise.resolve(mockPolicies);
  return getJSON<RemediationPolicy[]>("/policies");
}

// Settings는 incidents/policies와 달리 항상 백엔드(DB)와 통신한다.
// 백엔드가 없으면(예: 순수 mock dev) 기본값으로 폴백한다.
export async function fetchSettings(): Promise<ProviderSettings> {
  try {
    return await getJSON<ProviderSettings>("/settings");
  } catch {
    return mockSettings;
  }
}

export async function saveSettings(s: ProviderSettings): Promise<ProviderSettings> {
  return putJSON<ProviderSettings>("/settings", s);
}

// ── 민감정보 (write-only) ─────────────────────────────────────────
export interface SecretsStatus {
  aiApiKey: boolean;
  gitToken: boolean;
}

// 어떤 시크릿이 설정돼 있는지 여부만 (값은 절대 반환되지 않음)
export async function fetchSecretsStatus(): Promise<SecretsStatus> {
  return getJSON<SecretsStatus>("/secrets");
}

// 시크릿 설정/변경/삭제. 값 있음=설정, ""=삭제, null/미포함=변경없음.
export async function saveSecrets(patch: { aiApiKey?: string | null; gitToken?: string | null }): Promise<SecretsStatus> {
  return putJSON<SecretsStatus>("/secrets", patch);
}

// 미래 기능(MVP-2): 승인/반려 액션. 현재는 비활성(백엔드 미구현).
export async function decideApproval(_id: string, _decision: "approve" | "reject"): Promise<void> {
  // TODO(backend): POST /api/incidents/:id/approval { decision }
  throw new Error("승인 액션은 아직 구현되지 않았습니다 (MVP-2 예정).");
}

export const isMockMode = USE_MOCK;

// 현재 백엔드가 연결하도록 설정된(활성) AI 제공자 정보
export interface AIStatus {
  endpoint: string;
  model: string;
  providerKind: string; // local | frontier | unknown
  providerName: string;
}

// 백엔드가 수행한 health check 결과
export interface AIHealth {
  healthy: boolean;
  latencyMs: number;
  models: string[];
  modelAvailable: boolean;
  error?: string;
}

// 활성 AI 제공자 정보 (백엔드 기준)
export async function fetchAIStatus(): Promise<AIStatus> {
  return getJSON<AIStatus>("/ai/status");
}

// 백엔드 → health check. endpoint를 주면 폼에 입력한 주소를 즉시 검사(저장/재시작 불필요).
// API key는 백엔드가 DB 시크릿에서 실시간 조회.
export async function checkAIHealth(endpoint?: string): Promise<AIHealth> {
  const q = endpoint ? `?endpoint=${encodeURIComponent(endpoint)}` : "";
  return getJSON<AIHealth>(`/ai/health${q}`);
}

// 이 앱 자신의 Deployment를 rollout-restart한다(저장된 AI 설정을 반영). RBAC 미부여 시 에러.
export async function restartAIPod(): Promise<void> {
  await sendJSON("POST", "/ai/restart");
}
