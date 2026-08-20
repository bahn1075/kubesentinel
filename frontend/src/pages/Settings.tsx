import { useEffect, useState } from "react";
import { Info } from "@phosphor-icons/react";
import type { ProviderSettings } from "../api/types";
import {
  fetchSettings, saveSettings, fetchAIStatus, checkAIHealth, restartAIPod,
  fetchSecretsStatus, saveSecrets,
  type AIStatus, type AIHealth, type SecretsStatus,
} from "../api/client";
import Skeleton from "../components/Skeleton";

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

  const [statusChecking, setStatusChecking] = useState(false);
  const [statusErr, setStatusErr] = useState<string | null>(null);

  const [restarting, setRestarting] = useState(false);
  const [restartMsg, setRestartMsg] = useState<string | null>(null);
  const [restartErr, setRestartErr] = useState<string | null>(null);

  useEffect(() => {
    fetchSettings().then(setS).catch((e) => setLoadErr(String(e)));
    refreshStatus();
    fetchSecretsStatus().then(setSecretsSet).catch(() => {});
  }, []);

  async function refreshStatus() {
    setStatusChecking(true); setStatusErr(null);
    try {
      setStatus(await fetchAIStatus());
    } catch (e) {
      setStatus(null);
      setStatusErr(e instanceof Error ? e.message : String(e));
    } finally {
      setStatusChecking(false);
    }
  }

  function update<K extends keyof ProviderSettings>(section: K, patch: Partial<ProviderSettings[K]>) {
    setS((prev) => (prev ? { ...prev, [section]: { ...prev[section], ...patch } } : prev));
    setSaved(false);
  }

  // 저장 성공 시 true를 반환한다(Pod 재시작 전 자동 저장에 사용).
  async function onSave(): Promise<boolean> {
    if (!s) return false;
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
      return true;
    } catch (e) {
      setSaveErr(e instanceof Error ? e.message : String(e));
      return false;
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

  async function onRestartPod() {
    if (!window.confirm("현재 화면의 AI 설정(Endpoint/Model)을 저장한 뒤 Pod를 재시작하시겠습니까? 재시작 중 잠시 진단이 중단됩니다.")) return;
    setRestarting(true); setRestartMsg(null); setRestartErr(null);
    try {
      // Pod 재시작은 DB에 저장된 설정을 다시 읽어들이므로, 화면에서 고른 값이 반영되도록 먼저 저장한다.
      if (!(await onSave())) {
        setRestartErr("설정 저장에 실패해 재시작을 취소했습니다.");
        return;
      }
      await restartAIPod();
      setRestartMsg("설정을 저장하고 재시작을 요청했습니다. 반영까지 잠시 기다린 후 상태를 다시 확인하세요.");
    } catch (e) {
      setRestartErr(e instanceof Error ? e.message : String(e));
    } finally {
      setRestarting(false);
    }
  }

  function onChangeProvider(p: string) {
    const ep = FRONTIER_ENDPOINTS[p];
    update("ai", { provider: p, ...(ep !== undefined && (!s?.ai.endpoint || Object.values(FRONTIER_ENDPOINTS).includes(s?.ai.endpoint || "")) ? { endpoint: ep } : {}) });
  }

  if (loadErr) return <p className="test-result err">설정 로드 실패: {loadErr}</p>;
  if (!s) return <Skeleton title rows={8} />;

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
              <option value="">선택</option>
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
              <input type="password" placeholder={secretsSet.aiApiKey ? "설정됨 (변경 시 새 값 입력)" : "sk-..."}
                value={aiApiKeyInput} onChange={(e) => { setAiApiKeyInput(e.target.value); setSaved(false); }} />
            </>}
            {s.ai.authMethod !== "api-key" && (
              <><label></label><span className="muted">선택한 인증 방식은 준비 중입니다. 현재는 API Key만 동작합니다.</span></>
            )}
          </>}

          <label>Endpoint</label>
          <div>
            <input value={s.ai.endpoint} placeholder={isLocal ? "http://host.minikube.internal:1234/v1" : "https://api.openai.com/v1"}
              onChange={(e) => update("ai", { endpoint: e.target.value })} />
            <div className="muted" style={{ fontSize: 12, marginTop: 4 }}>
              끝에 <code>/models</code>는 붙이지 마세요 (예: <code>{isLocal ? "http://host:1234/v1" : "https://api.openai.com/v1"}</code>). 조회 시 자동으로 붙습니다.
            </div>
          </div>

          <label>현재 활성 Model</label>
          <div className="btn-row" style={{ alignItems: "center" }}>
            <span>
              {status ? `${status.providerName} · ${status.model || "(모델 미설정)"}` : (statusErr ? "조회 실패" : "(조회 안 됨)")}
            </span>
            <button type="button" onClick={refreshStatus} disabled={statusChecking}>
              {statusChecking ? "새로고침 중" : "새로고침"}
            </button>
            <span className="muted" style={{ fontSize: 12 }}>저장·재시작된 설정 기준(위 Endpoint 입력값과 무관)</span>
          </div>

          <label>Model</label>
          {health && health.healthy && health.models.length > 0 ? (
            <select value={s.ai.model} onChange={(e) => update("ai", { model: e.target.value })}>
              {!health.models.includes(s.ai.model) && <option value={s.ai.model}>{s.ai.model || "선택"}</option>}
              {health.models.map((m) => <option key={m} value={m}>{m}</option>)}
            </select>
          ) : (
            <input value={s.ai.model} placeholder="모델명 (상태확인으로 조회 후 선택)"
              onChange={(e) => update("ai", { model: e.target.value })} />
          )}

          <label>상태 확인</label>
          <div>
            <button onClick={onCheckHealth} disabled={checking || !s.ai.endpoint}>
              {checking ? "조회 중" : "상태 확인 (모델 조회)"}
            </button>
          </div>

          <label>Pod 재시작</label>
          <div className="btn-row" style={{ alignItems: "center" }}>
            <button type="button" onClick={onRestartPod} disabled={restarting}>
              {restarting ? "재시작 요청 중" : "Pod 재시작 (설정 반영)"}
            </button>
            {restartMsg && <span className="badge ok">{restartMsg}</span>}
            {restartErr && <span className="badge crit">{restartErr}</span>}
          </div>

          <label>External 허용</label>
          <label className="chk"><input type="checkbox" checked={s.ai.allowExternal} onChange={(e) => update("ai", { allowExternal: e.target.checked })} /> 외부 모델로 evidence 전송 허용</label>
          <label>Secret redact</label>
          <label className="chk"><input type="checkbox" checked={s.ai.redactSecrets} onChange={(e) => update("ai", { redactSecrets: e.target.checked })} /> 전송 전 시크릿 마스킹</label>
        </div>

        {healthErr && <div className="test-result err">연결 실패: {healthErr}</div>}
        {health && (health.healthy ? (
          <div className="test-result ok">
            <strong>연결 성공</strong> ({health.latencyMs}ms) · {health.models.length}개 모델. 위 Model 항목에서 선택하세요.
          </div>
        ) : <div className="test-result err">연결 실패: {health.error}</div>)}

        <p className="muted" style={{ fontSize: 12, marginTop: 12, marginBottom: 0 }}>
          설정 저장 후 실제 반영을 위해서는 Pod 재시작이 필요합니다(AI 설정은 기동 시에만 로드됩니다).
        </p>
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
        <div className="test-result" style={{ display: "flex", gap: 8 }}>
          <Info size={16} color="var(--accent-text)" aria-hidden style={{ flex: "none", marginTop: 2 }} />
          <div>
            KubeSentinel은 Alertmanager의 <b>수신자</b>입니다. Alertmanager 설정에 아래 receiver를 추가하세요:
            <div className="mono" style={{ marginTop: 6 }}>http://&lt;backend-svc&gt;.&lt;namespace&gt;.svc:8080/v1/alerts</div>
            <div className="muted" style={{ fontSize: 12, marginTop: 4 }}>위 Alertmanager URL은 추후 alert 상태 조회/검증에 사용됩니다(현재는 저장만).</div>
          </div>
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
            <input type="password" placeholder={secretsSet.gitToken ? "설정됨 (변경 시 새 값 입력)" : "ghp_... / glpat-... / gitea token"}
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
        <button className="primary" onClick={onSave} disabled={saving}>{saving ? "저장 중" : "저장"}</button>
        {saved && <span className="badge ok">저장되었습니다 (DB)</span>}
        {saveErr && <span className="badge crit">저장 실패: {saveErr}</span>}
      </div>
    </>
  );
}
