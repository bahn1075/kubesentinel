import { useEffect, useState } from "react";
import type { ProviderSettings } from "../api/types";
import {
  fetchSettings, saveSettings, fetchAIStatus, checkAIHealth,
  fetchSecretsStatus, saveSecrets,
  type AIStatus, type AIHealth, type SecretsStatus,
} from "../api/client";

// frontier provider별 기본 엔드포인트 (OpenAI 호환 base)
const FRONTIER_ENDPOINTS: Record<string, string> = {
  openai: "https://api.openai.com/v1",
  anthropic: "https://api.anthropic.com/v1",
  "azure-openai": "",
  google: "https://generativelanguage.googleapis.com/v1beta/openai",
  custom: "",
};
// git provider별 인증 방식 (token만 동작, 나머지 준비중)
const GIT_AUTH: Record<string, { value: string; label: string }[]> = {
  github: [{ value: "token", label: "Personal Access Token" }, { value: "oauth", label: "OAuth (준비 중)" }, { value: "ssh", label: "SSH 키 (준비 중)" }],
  gitlab: [{ value: "token", label: "Personal Access Token" }, { value: "oauth", label: "OAuth (준비 중)" }, { value: "ssh", label: "SSH 키 (준비 중)" }],
  gitea: [{ value: "token", label: "Access Token" }, { value: "ssh", label: "SSH 키 (준비 중)" }],
};

export default function Settings() {
  const [s, setS] = useState<ProviderSettings | null>(null);
  const [loadErr, setLoadErr] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [saveErr, setSaveErr] = useState<string | null>(null);

  const [status, setStatus] = useState<AIStatus | null>(null);
  const [secretsSet, setSecretsSet] = useState<SecretsStatus>({ aiApiKey: false, gitToken: false });
  // 새로 입력한 시크릿(저장 시 전송). 빈 문자열이면 변경 없음으로 취급.
  const [aiApiKeyInput, setAiApiKeyInput] = useState("");
  const [gitTokenInput, setGitTokenInput] = useState("");

  const [checking, setChecking] = useState(false);
  const [health, setHealth] = useState<AIHealth | null>(null);
  const [healthErr, setHealthErr] = useState<string | null>(null);

  useEffect(() => {
    fetchSettings().then(setS).catch((e) => setLoadErr(String(e)));
    fetchAIStatus().then(setStatus).catch(() => setStatus(null));
    fetchSecretsStatus().then(setSecretsSet).catch(() => {});
  }, []);

  function update<K extends keyof ProviderSettings>(section: K, patch: Partial<ProviderSettings[K]>) {
    setS((prev) => (prev ? { ...prev, [section]: { ...prev[section], ...patch } } : prev));
    setSaved(false);
  }

  async function onSave() {
    if (!s) return;
    setSaving(true); setSaveErr(null);
    try {
      const persisted = await saveSettings(s);
      setS(persisted);
      // 입력된 시크릿만 전송 (빈 문자열 = 변경 없음 → null)
      const patch: { aiApiKey?: string | null; gitToken?: string | null } = {};
      if (aiApiKeyInput) patch.aiApiKey = aiApiKeyInput;
      if (gitTokenInput) patch.gitToken = gitTokenInput;
      if (patch.aiApiKey !== undefined || patch.gitToken !== undefined) {
        setSecretsSet(await saveSecrets(patch));
        setAiApiKeyInput(""); setGitTokenInput("");
      }
      setSaved(true);
    } catch (e) {
      setSaveErr(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  async function onCheckHealth() {
    if (!s) return;
    setChecking(true); setHealth(null); setHealthErr(null);
    try {
      setHealth(await checkAIHealth(s.ai.endpoint || undefined));
    } catch (e) {
      setHealthErr(e instanceof Error ? e.message : String(e));
    } finally {
      setChecking(false);
    }
  }

  function onChangeProvider(p: string) {
    const ep = FRONTIER_ENDPOINTS[p];
    update("ai", { provider: p, ...(ep !== undefined && (!s?.ai.endpoint || Object.values(FRONTIER_ENDPOINTS).includes(s?.ai.endpoint || "")) ? { endpoint: ep } : {}) });
  }

  if (loadErr) return <p className="test-result err">설정 로드 실패: {loadErr}</p>;
  if (!s) return <p className="muted">로딩 중…</p>;

  const isLocal = s.ai.kind === "local";
  const isFrontier = s.ai.kind === "frontier";

  return (
    <>
      <h1 className="page-title">Settings</h1>
      <p className="page-sub">설정은 백엔드 DB에 저장됩니다. 민감정보(키/토큰)는 write-only로 저장되어 값은 다시 표시되지 않습니다.</p>

      {/* ── AI Provider ── */}
      <div className="section">
        <h3>AI Provider</h3>
        <div className="form-grid">
          <label>종류</label>
          <div className="btn-row">
            <label className="chk"><input type="radio" name="aikind" checked={isLocal} onChange={() => update("ai", { kind: "local" })} /> 로컬 (LM Studio·Ollama 등)</label>
            <label className="chk"><input type="radio" name="aikind" checked={isFrontier} onChange={() => update("ai", { kind: "frontier" })} /> 프론티어 (OpenAI·Anthropic 등)</label>
          </div>

          {isFrontier && <>
            <label>제공자</label>
            <select value={s.ai.provider} onChange={(e) => onChangeProvider(e.target.value)}>
              <option value="">선택…</option>
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="azure-openai">Azure OpenAI</option>
              <option value="google">Google Gemini</option>
              <option value="custom">Custom</option>
            </select>

            <label>인증 방식</label>
            <select value={s.ai.authMethod} onChange={(e) => update("ai", { authMethod: e.target.value })}>
              <option value="api-key">API Key</option>
              <option value="oauth">OAuth (준비 중)</option>
              <option value="machine">Machine 인증 (준비 중)</option>
            </select>

            {s.ai.authMethod === "api-key" && <>
              <label>API Key</label>
              <input type="password" placeholder={secretsSet.aiApiKey ? "설정됨 — 변경 시 새 값 입력" : "sk-..."}
                value={aiApiKeyInput} onChange={(e) => { setAiApiKeyInput(e.target.value); setSaved(false); }} />
            </>}
            {s.ai.authMethod !== "api-key" && (
              <><label></label><span className="muted">선택한 인증 방식은 준비 중입니다. 현재는 API Key만 동작합니다.</span></>
            )}
          </>}

          <label>Endpoint{isFrontier ? "" : ""}</label>
          <input value={s.ai.endpoint} placeholder={isLocal ? "http://host.minikube.internal:1234/v1" : "https://api.openai.com/v1"}
            onChange={(e) => update("ai", { endpoint: e.target.value })} />

          <label>Model</label>
          <input value={s.ai.model} placeholder="모델명 (상태확인으로 조회 후 선택)"
            onChange={(e) => update("ai", { model: e.target.value })} />

          <label>상태 확인</label>
          <div>
            <button onClick={onCheckHealth} disabled={checking || !s.ai.endpoint}>
              {checking ? "조회 중…" : "상태 확인 (모델 조회)"}
            </button>
          </div>

          <label>External 허용</label>
          <label className="chk"><input type="checkbox" checked={s.ai.allowExternal} onChange={(e) => update("ai", { allowExternal: e.target.checked })} /> 외부 모델로 evidence 전송 허용</label>
          <label>Secret redact</label>
          <label className="chk"><input type="checkbox" checked={s.ai.redactSecrets} onChange={(e) => update("ai", { redactSecrets: e.target.checked })} /> 전송 전 시크릿 마스킹</label>
        </div>

        {healthErr && <div className="test-result err">연결 실패: {healthErr}</div>}
        {health && (health.healthy ? (
          <div className="test-result ok">
            <strong>연결 성공</strong> ({health.latencyMs}ms) — {health.models.length}개 모델 (클릭하면 Model에 적용):
            <div className="model-chips">
              {health.models.map((m) => (
                <button key={m} type="button" className={`model-chip ${m === s.ai.model ? "active" : ""}`}
                  onClick={() => update("ai", { model: m })}>{m}{m === s.ai.model ? " ✓" : ""}</button>
              ))}
            </div>
          </div>
        ) : <div className="test-result err">연결 실패: {health.error}</div>)}

        {status && (
          <p className="muted" style={{ fontSize: 12, marginTop: 10 }}>
            현재 활성(백엔드): {status.providerName} · {status.model || "(모델 미설정)"} — 저장 후 백엔드 재시작 시 반영
          </p>
        )}
      </div>

      {/* ── Collector ── */}
      <div className="section">
        <h3>Collector</h3>
        <div className="form-grid">
          <label>Prometheus</label>
          <input value={s.collector.prometheusUrl} placeholder="http://prometheus-operated.monitoring.svc:9090"
            onChange={(e) => update("collector", { prometheusUrl: e.target.value })} />
          <label>Loki</label>
          <input value={s.collector.lokiUrl} placeholder="http://loki-gateway.monitoring.svc:80"
            onChange={(e) => update("collector", { lokiUrl: e.target.value })} />
          <label>Alertmanager</label>
          <input value={s.collector.alertmanagerUrl} placeholder="http://alertmanager-operated.monitoring.svc:9093"
            onChange={(e) => update("collector", { alertmanagerUrl: e.target.value })} />
          <label>Grafana</label>
          <input value={s.collector.grafanaUrl} placeholder="(선택) 알림 딥링크용"
            onChange={(e) => update("collector", { grafanaUrl: e.target.value })} />
        </div>
        <div className="test-result" style={{ background: "var(--bg-elev2)" }}>
          ℹ️ KubeSentinel은 Alertmanager의 <b>수신자</b>입니다. Alertmanager 설정에 아래 receiver를 추가하세요:
          <div className="mono" style={{ marginTop: 6 }}>http://&lt;backend-svc&gt;.&lt;namespace&gt;.svc:8080/v1/alerts</div>
          <div className="muted" style={{ fontSize: 12, marginTop: 4 }}>위 Alertmanager URL은 추후 alert 상태 조회/검증에 사용됩니다(현재는 저장만).</div>
        </div>
      </div>

      {/* ── Notifier ── */}
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

      {/* ── Git (추후 직접 업데이트 대상) ── */}
      <div className="section">
        <h3>Git <span className="tag">MVP-1</span></h3>
        <div className="form-grid">
          <label>Provider</label>
          <select value={s.git.provider} onChange={(e) => update("git", { provider: e.target.value, authMethod: (GIT_AUTH[e.target.value] || [{ value: "token" }])[0].value })}>
            <option value="github">GitHub</option>
            <option value="gitlab">GitLab</option>
            <option value="gitea">Gitea</option>
          </select>

          <label>인증 방식</label>
          <select value={s.git.authMethod} onChange={(e) => update("git", { authMethod: e.target.value })}>
            {(GIT_AUTH[s.git.provider] || []).map((a) => <option key={a.value} value={a.value}>{a.label}</option>)}
          </select>

          {s.git.authMethod === "token" && <>
            <label>Token</label>
            <input type="password" placeholder={secretsSet.gitToken ? "설정됨 — 변경 시 새 값 입력" : "ghp_... / glpat-... / gitea token"}
              value={gitTokenInput} onChange={(e) => { setGitTokenInput(e.target.value); setSaved(false); }} />
          </>}
          {s.git.authMethod !== "token" && (
            <><label></label><span className="muted">선택한 인증 방식은 준비 중입니다. 현재는 Token만 동작합니다.</span></>
          )}

          <label>Repository</label>
          <input value={s.git.repository} placeholder="your-org/manifests"
            onChange={(e) => update("git", { repository: e.target.value })} />
          <label>Base branch</label>
          <input value={s.git.baseBranch} placeholder="main"
            onChange={(e) => update("git", { baseBranch: e.target.value })} />
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
