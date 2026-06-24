import { useState } from "react";
import type { ProviderSettings } from "../api/types";
import { loadSettings, saveSettings, resetSettings } from "../api/settingsStore";

// AIProvider / Collector / Notifier / GitOps 설정 편집 (architecture.md §4).
// 입력값은 localStorage에 저장되어 앱 전체(fetchSettings)에서 사용된다.
export default function Settings() {
  const [s, setS] = useState<ProviderSettings>(loadSettings);
  const [saved, setSaved] = useState(false);

  // 중첩 필드 업데이트 헬퍼
  function update<K extends keyof ProviderSettings>(section: K, patch: Partial<ProviderSettings[K]>) {
    setS((prev) => ({ ...prev, [section]: { ...prev[section], ...patch } }));
    setSaved(false);
  }

  function onSave() {
    saveSettings(s);
    setSaved(true);
  }

  function onReset() {
    setS(resetSettings());
    setSaved(false);
  }

  return (
    <>
      <h1 className="page-title">Settings</h1>
      <p className="page-sub">플랫폼 연동 설정. 입력값은 브라우저에 저장되어 앱에서 사용됩니다 (백엔드 API 연동 시 서버로 전송).</p>

      {/* AI Provider */}
      <div className="section">
        <h3>AI Provider</h3>
        <div className="form-grid">
          <label>Type</label>
          <select value={s.ai.type} onChange={(e) => update("ai", { type: e.target.value })}>
            <option value="openai-compatible">openai-compatible</option>
          </select>

          <label>Endpoint</label>
          <input value={s.ai.endpoint} placeholder="http://ollama.llm.svc:11434/v1"
            onChange={(e) => update("ai", { endpoint: e.target.value })} />

          <label>Model</label>
          <input value={s.ai.model} placeholder="llama3 | gpt-4o-mini"
            onChange={(e) => update("ai", { model: e.target.value })} />

          <label>External 허용</label>
          <label className="chk"><input type="checkbox" checked={s.ai.allowExternal}
            onChange={(e) => update("ai", { allowExternal: e.target.checked })} /> 외부 모델로 evidence 전송 허용</label>

          <label>Secret redact</label>
          <label className="chk"><input type="checkbox" checked={s.ai.redactSecrets}
            onChange={(e) => update("ai", { redactSecrets: e.target.checked })} /> 전송 전 시크릿 마스킹</label>
        </div>
      </div>

      {/* Collector */}
      <div className="section">
        <h3>Collector</h3>
        <div className="form-grid">
          <label>Prometheus</label>
          <input value={s.collector.prometheusUrl} placeholder="http://prometheus-operated.monitoring.svc:9090"
            onChange={(e) => update("collector", { prometheusUrl: e.target.value })} />
          <label>Loki</label>
          <input value={s.collector.lokiUrl} placeholder="http://loki-gateway.monitoring.svc:80"
            onChange={(e) => update("collector", { lokiUrl: e.target.value })} />
          <label>Grafana</label>
          <input value={s.collector.grafanaUrl} placeholder="(선택) 알림 딥링크용"
            onChange={(e) => update("collector", { grafanaUrl: e.target.value })} />
        </div>
      </div>

      {/* Notifier */}
      <div className="section">
        <h3>Notifier</h3>
        <div className="form-grid">
          <label>Type</label>
          <select value={s.notifier.type} onChange={(e) => update("notifier", { type: e.target.value })}>
            <option value="slack">slack</option>
            <option value="discord">discord</option>
            <option value="teams">teams</option>
          </select>
          <label>Webhook</label>
          <input value={s.notifier.webhook} placeholder="https://hooks.slack.com/services/..."
            onChange={(e) => update("notifier", { webhook: e.target.value })} />
          <label>상태</label>
          <span><span className={`badge ${s.notifier.webhook ? "ok" : "dim"}`}>{s.notifier.webhook ? "설정됨" : "미설정"}</span></span>
        </div>
      </div>

      {/* GitOps */}
      <div className="section">
        <h3>GitOps <span className="tag">MVP-1</span></h3>
        <div className="form-grid">
          <label>Provider</label>
          <select value={s.gitops.provider} onChange={(e) => update("gitops", { provider: e.target.value })}>
            <option value="github">github</option>
            <option value="gitlab">gitlab</option>
            <option value="bitbucket">bitbucket</option>
          </select>
          <label>Repository</label>
          <input value={s.gitops.repository} placeholder="your-org/manifests"
            onChange={(e) => update("gitops", { repository: e.target.value })} />
          <label>Base branch</label>
          <input value={s.gitops.baseBranch} placeholder="main"
            onChange={(e) => update("gitops", { baseBranch: e.target.value })} />
        </div>
      </div>

      <div className="btn-row" style={{ alignItems: "center" }}>
        <button className="primary" onClick={onSave}>저장</button>
        <button onClick={onReset}>기본값으로 초기화</button>
        {saved && <span className="badge ok">저장되었습니다</span>}
      </div>
    </>
  );
}
