import { useParams, Link } from "react-router-dom";
import { fetchIncident } from "../api/client";
import { useAsync } from "../lib/useAsync";
import { STATE_FLOW, severityClass, stateClass, riskClass, formatTime, isFailureState } from "../lib/format";

// 근거 품질 뱃지 (백엔드가 코드로 계산한 값)
function evidenceBadge(q?: string) {
  if (!q) return null;
  if (q === "rich") return <span className="badge ok">근거 충분</span>;
  if (q === "partial") return <span className="badge warn">근거 부분</span>;
  return <span className="badge crit">근거 부족 · 조사용</span>; // none
}

export default function IncidentDetail() {
  const { id = "" } = useParams();
  const { data: inc, loading } = useAsync(() => fetchIncident(id), [id]);

  if (loading) return <p className="muted">로딩 중…</p>;
  if (!inc) return <p>인시던트를 찾을 수 없습니다. <Link to="/incidents">목록으로</Link></p>;

  const currentIdx = STATE_FLOW.indexOf(inc.state);

  return (
    <>
      <p className="muted" style={{ marginBottom: 6 }}><Link to="/incidents">← Incidents</Link></p>
      <h1 className="page-title">
        <code>{inc.alert}</code>{" "}
        <span className={`badge ${severityClass(inc.severity)}`}>{inc.severity}</span>{" "}
        <span className={`badge ${stateClass(inc.state)}`}>{inc.state}</span>
        {inc.rule && inc.rule.category !== "Unknown" && (
          <> <span className="badge info" title={inc.rule.rationale}>rule: {inc.rule.category}</span></>
        )}
      </h1>
      <p className="page-sub mono">{inc.incidentId} · {inc.namespace}/{inc.workload}{inc.pod ? ` · ${inc.pod}` : ""} · {formatTime(inc.createdAt)}</p>

      {/* 상태 타임라인 (architecture.md §5) */}
      <div className="section">
        <h3>진행 상태</h3>
        <div className="timeline">
          {STATE_FLOW.map((s, idx) => {
            const cls = isFailureState(inc.state)
              ? idx < currentIdx ? "done" : ""
              : idx < currentIdx ? "done" : idx === currentIdx ? "current" : "";
            return <span key={s} className={`step ${cls}`}>{s}</span>;
          })}
          {isFailureState(inc.state) && <span className="step" style={{ color: "var(--crit)", borderColor: "#5a2a2a" }}>{inc.state}</span>}
        </div>
      </div>

      {/* RCA */}
      {inc.diagnosis && (
        <div className="section">
          <h3>AI 진단 (RCA)</h3>
          <div className="kv" style={{ marginBottom: 12 }}>
            <span className="k">Root Cause</span><span>{inc.diagnosis.rootCause}</span>
            <span className="k">Summary</span><span>{inc.diagnosis.summary}</span>
            <span className="k">Confidence</span>
            <span style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span className="confidence-bar"><div style={{ width: `${inc.diagnosis.confidence * 100}%` }} /></span>
              {Math.round(inc.diagnosis.confidence * 100)}%
              {evidenceBadge(inc.diagnosis.evidenceQuality)}
            </span>
          </div>
          {inc.diagnosis.evidenceQuality && inc.diagnosis.evidenceQuality !== "rich" && (
            <p className="muted" style={{ fontSize: 12, marginTop: -6, marginBottom: 12 }}>
              ⚠️ 근거(metric/log/event)가 제한적이라 <b>조사용 진단</b>입니다. 아래 "동시 발생 alert"와 함께 검토하세요.
            </p>
          )}

          <h3>제안 조치 <span className="tag">AI는 제안만, 적용은 정책·승인 후</span></h3>
          <ul className="actions-list">
            {inc.diagnosis.proposedActions.map((a, idx) => (
              <li key={idx}>
                <span className="badge dim">{a.type}</span>{" "}
                <span className={`badge ${riskClass(a.risk)}`}>risk: {a.risk}</span>
                <div style={{ marginTop: 6 }}>{a.description}</div>
                {a.target && <div className="mono muted" style={{ marginTop: 4 }}>→ {a.target}</div>}
              </li>
            ))}
          </ul>

          <div className="btn-row" style={{ marginTop: 12 }}>
            <button className="primary" disabled title="MVP-2 예정">승인</button>
            <button disabled title="MVP-2 예정">반려</button>
            {inc.prUrl && <a href={inc.prUrl} target="_blank" rel="noreferrer"><button>PR 열기 ↗</button></a>}
          </div>
          <p className="muted" style={{ fontSize: 12, marginTop: 8 }}>
            승인/반려 액션은 MVP-2에서 활성화됩니다. (architecture.md §4.7 / §7)
          </p>
        </div>
      )}

      {/* Evidence */}
      {inc.evidence && (
        <div className="section">
          <h3>근거 (Evidence)</h3>
          {inc.evidence.relatedAlerts && inc.evidence.relatedAlerts.length > 0 && (
            <>
              <p className="k muted" style={{ margin: "0 0 4px" }}>동시 발생 alert (상관 분석 입력)</p>
              <div className="logs">
                {inc.evidence.relatedAlerts.map((a, i) => (
                  <div key={i}>• <code>{a.alertname}</code>{a.namespace ? ` (${a.namespace})` : ""}{a.severity ? ` · ${a.severity}` : ""}{a.summary ? ` — ${a.summary}` : ""}</div>
                ))}
              </div>
            </>
          )}
          {inc.evidence.gitContext && (
            <p className="mono muted">git: {inc.evidence.gitContext.repo}/{inc.evidence.gitContext.path} @ {inc.evidence.gitContext.lastCommit}</p>
          )}
          {inc.evidence.resourceStatus && Object.keys(inc.evidence.resourceStatus).length > 0 && (
            <>
              <p className="k muted" style={{ margin: "10px 0 4px" }}>Resource Status (K8s API)</p>
              <div className="logs mono">{JSON.stringify(inc.evidence.resourceStatus)}</div>
            </>
          )}
          {inc.evidence.events.length > 0 && (
            <>
              <p className="k muted" style={{ margin: "10px 0 4px" }}>Events (K8s)</p>
              <div className="logs">{inc.evidence.events.map((e, i) => <div key={i}>• {e}</div>)}</div>
            </>
          )}
          {inc.evidence.logs.length > 0 && (
            <>
              <p className="k muted" style={{ margin: "12px 0 4px" }}>Logs</p>
              <div className="logs mono">{inc.evidence.logs.map((l, i) => <div key={i}>{l}</div>)}</div>
            </>
          )}
          {inc.evidence.metrics.length > 0 && (
            <>
              <p className="k muted" style={{ margin: "12px 0 4px" }}>Metrics</p>
              <div className="logs mono">{inc.evidence.metrics.map((m, i) => <div key={i}>{m.name}: {m.query}</div>)}</div>
            </>
          )}
        </div>
      )}
    </>
  );
}
