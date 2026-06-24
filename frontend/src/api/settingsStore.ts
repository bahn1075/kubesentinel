import type { ProviderSettings } from "./types";
import { mockSettings } from "./mock";

// 백엔드 settings API가 없는 동안 사용자가 입력한 설정을 브라우저 localStorage에 보관한다.
// 백엔드가 생기면 saveSettings에서 PUT /api/settings 로 전송하도록 확장한다.
const KEY = "kubesentinel.settings";

// 섹션별 병합으로 스키마가 바뀌어도 안전하게 기본값을 채운다.
export function loadSettings(): ProviderSettings {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return mockSettings;
    const p = JSON.parse(raw) as Partial<ProviderSettings>;
    return {
      ai: { ...mockSettings.ai, ...p.ai },
      collector: { ...mockSettings.collector, ...p.collector },
      notifier: { ...mockSettings.notifier, ...p.notifier },
      gitops: { ...mockSettings.gitops, ...p.gitops },
    };
  } catch {
    return mockSettings;
  }
}

export function saveSettings(s: ProviderSettings): void {
  localStorage.setItem(KEY, JSON.stringify(s));
  // TODO(backend): await fetch(`${API_BASE}/settings`, { method: "PUT", body: JSON.stringify(s) })
}

export function resetSettings(): ProviderSettings {
  localStorage.removeItem(KEY);
  return mockSettings;
}
