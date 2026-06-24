import type { Incident, RemediationPolicy, ProviderSettings } from "./types";
import { mockIncidents, mockPolicies } from "./mock";
import { loadSettings } from "./settingsStore";

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

export async function fetchIncidents(): Promise<Incident[]> {
  if (USE_MOCK) return Promise.resolve(mockIncidents);
  return getJSON<Incident[]>("/incidents"); // TODO(backend): GET /api/incidents
}

export async function fetchIncident(id: string): Promise<Incident | undefined> {
  if (USE_MOCK) return Promise.resolve(mockIncidents.find((i) => i.incidentId === id));
  return getJSON<Incident>(`/incidents/${encodeURIComponent(id)}`);
}

export async function fetchPolicies(): Promise<RemediationPolicy[]> {
  if (USE_MOCK) return Promise.resolve(mockPolicies);
  return getJSON<RemediationPolicy[]>("/policies");
}

export async function fetchSettings(): Promise<ProviderSettings> {
  // mock 모드: 사용자가 저장한 설정(localStorage)을 사용. 미저장이면 기본값.
  if (USE_MOCK) return Promise.resolve(loadSettings());
  return getJSON<ProviderSettings>("/settings");
}

// 미래 기능(MVP-2): 승인/반려 액션. 현재는 비활성(백엔드 미구현).
export async function decideApproval(_id: string, _decision: "approve" | "reject"): Promise<void> {
  // TODO(backend): POST /api/incidents/:id/approval { decision }
  throw new Error("승인 액션은 아직 구현되지 않았습니다 (MVP-2 예정).");
}

export const isMockMode = USE_MOCK;
