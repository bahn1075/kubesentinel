import { useEffect, useState } from "react";
import type { ProviderSettings } from "../api/types";
import { fetchSettings, saveSettings, fetchAIStatus, checkAIHealth, type AIStatus, type AIHealth } from "../api/client";

// AIProvider / Collector / Notifier / GitOps 설정 편집 (architecture.md §4).
// 비민감 설정은 백엔드 API(/api/settings → Postgres)에 저장된다.
// 민감정보(API Key/token)는 DB가 아닌 k8s Secret/env로 관리한다.
export default function Settings() {
  const [s, setS] = useState<ProviderSettings | null>(null);
  const [loadErr, setLoadErr] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [saveErr, setSaveErr] = useState<string | null>(null);

  // 활성 제공자 정보 + health check
  const [status, setStatus] = useState<AIStatus | null>(null);
  const [checking, setChecking] = useState(false);
  const [health, setHealth] = useState<AIHealth | null>(null);
  const [healthErr, setHealthErr] = useState<string | null>(null);

  useEffect(() => {
    fetchSettings().then(setS).catch((e) => setLoadErr(String(e)));
    fetchAIStatus().then(setStatus).catch(() => setStatus(null));
  }, []);

  function update<K extends keyof ProviderSettings>(section: K, patch: Partial<ProviderSettings[K]>) {
    setS((prev) => (prev ? { ...prev, [section]: { ...prev[section], ...patch } } : prev));
    setSaved(false);
  }

  async function onSave() {
    if (!s) return;
    setSaving(true);
    setSaveErr(null);
    try {
      const persisted = await saveSettings(s);
      setS(persisted);
      setSaved(true);
    } catch (e) {
      setSaveErr(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  async function onCheckHealth() {
    setChecking(true);
    setHealth(null);
    setHealthErr(null);
    try {
      setHealth(await checkAIHealth());
    } catch (e) {
      setHealthErr(e instanceof Error ? e.message : String(e));
    } finally {
      setChecking(false);
    }
  }

  if (loadErr) return <p className="test-result err">설정 로드 실패: {loadErr}</p>;
  if (!s) return <p className="muted">로딩 중…</p>;

  const kindLabel = (k?: string) => (k === "frontier" ? "프론티어" : k === "local" ? "로컬" : "알 수 없음");

  return (
    <>
      <h1 className="page-title">Settings</h1>
      <p className="page-sub">플랫폼 연동 설정. 비민감 항목은 백엔드 DB에 저장됩니다 (민감정보는 k8s Secret으로 관리).</p>

      {/* AI Provider */}
      <div className="section">
        <h3>AI Provider</h3>
        <div className="form-grid">
          <label>Type</label>
          <select value={s.ai.type} onChange={(e) => update("ai", { type: e.target.value })}>
            <option value="openai-compatible">openai-compatible</option>
          </select>

          <label>Endpoint</label>
          <input value={s.ai.endpoint} placeholder="http://host.minikube.internal:1234/v1"
            onChange={(e) => update("ai", { endpoint: e.target.value })} />

          <label>Model</label>
          <input value={s.ai.model} placeholder="gemma-4-26b-a4b-it-mlx"
            onChange={(e) => update("ai", { model: e.target.value })} />

          <label>API Key</label>
          <span className="muted">k8s Secret/env로 관리 (DB 미저장)</span>

          <label>External 허용</label>
          <label className="chk"><input type="checkbox" checked={s.ai.allowExternal}
            onChange={(e) => update("ai", { allowExternal: e.target.checked })} /> 외부 모델로 evidence 전송 허용</label>

          <label>Secret redact</label>
          <label className="chk"><input type="checkbox" checked={s.ai.redactSecrets}
            onChange={(e) => update("ai", { redactSecrets: e.target.checked })} /> 전송 전 시크릿 마스킹</label>
        </div>

        {/* 현재 연결됨(활성) — 백엔드가 실제로 사용 중인 제공자. 폼 저장 후 재시작 시 갱신됨 */}
        <div className="provider-panel">
          <div className="provider-head">
            <strong>현재 연결됨</strong>
            <span className="muted" style={{ fontSize: 12 }}>저장한 설정은 백엔드 재시작 시 반영</span>
          </div>
          {status ? (
            <div className="kv">
              <span className="k">제공자 종류</span>
              <span><span className={`badge ${status.providerKind === "frontier" ? "warn" : status.providerKind === "local" ? "ok" : "dim"}`}>{kindLabel(status.providerKind)}</span></span>
              <span className="k">제공자</span><span>{status.providerName}</span>
              <span className="k">모델</span><span className="mono">{status.model || "—"}</span>
              <span className="k">Endpoint</span><span className="mono">{status.endpoint || "—"}</span>
            </div>
          ) : (
            <p className="muted">활성 제공자 정보를 불러올 수 없습니다.</p>
          )}

          <div className="btn-row" style={{ marginTop: 12 }}>
            <button onClick={onCheckHealth} disabled={checking}>{checking ? "확인 중…" : "상태 확인"}</button>
            {health && health.healthy && (
              <span className="badge ok">정상 · {health.latencyMs}ms · 모델 {health.models.length}개{health.modelAvailable ? " · 설정 모델 사용가능 ✓" : ""}</span>
            )}
            {health && !health.healthy && <span className="badge crit">비정상</span>}
          </div>

          {health && !health.healthy && <div className="test-result err" style={{ marginTop: 8 }}>{health.error}</div>}
          {healthErr && <div className="test-result err" style={{ marginTop: 8 }}>상태 확인 실패: {healthErr}</div>}
          {health && health.healthy && !health.modelAvailable && status?.model && (
            <div className="test-result err" style={{ marginTop: 8 }}>
              ⚠️ 설정된 모델 <code>{status.model}</code> 이(가) 제공자 모델 목록에 없습니다.
            </div>
          )}
          {health && health.healthy && health.models.length > 0 && (
            <div className="test-result ok" style={{ marginTop: 8 }}>
              사용 가능 모델 (클릭하면 위 Model 필드에 적용):
              <div className="model-chips">
                {health.models.map((m) => (
                  <button key={m} type="button"
                    className={`model-chip ${m === s.ai.model ? "active" : ""}`}
                    onClick={() => update("ai", { model: m })}>
                    {m}{m === s.ai.model ? " ✓" : ""}
                  </button>
                ))}
              </div>
            </div>
          )}
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
          <label>Webhook URL</label>
          <span className="muted">k8s Secret으로 관리 (DB 미저장)</span>
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
          <label>Token</label>
          <span className="muted">k8s Secret으로 관리 (DB 미저장)</span>
        </div>
      </div>

      <div className="btn-row" style={{ alignItems: "center" }}>
        <button className="primary" onClick={onSave} disabled={saving}>{saving ? "저장 중…" : "저장"}</button>
        {saved && <span className="badge ok">저장되었습니다 (DB)</span>}
        {saveErr && <span className="badge crit">저장 실패: {saveErr}</span>}
      </div>
    </>
  );
}
