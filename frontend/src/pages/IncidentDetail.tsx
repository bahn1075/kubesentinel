import { useParams, Link } from "react-router-dom";
import { Warning, BookOpen, ArrowLeft, ArrowSquareOut } from "@phosphor-icons/react";
import { fetchIncident } from "../api/client";
import { useAsync } from "../lib/useAsync";
import { STATE_FLOW, severityClass, stateClass, riskClass, formatTime, isFailureState } from "../lib/format";
import Skeleton from "../components/Skeleton";

// 근거 품질 뱃지 (백엔드가 코드로 계산한 값)
function evidenceBadge(q?: string) {
  if (!q) return null;
  if (q === "rich") return <span className="badge ok">근거 충분</span>;
  if (q === "partial") return <span className="badge warn">근거 부분</span>;
  return <span className="badge crit">근거 부족 · 조사용</span>; // none
}

// 자동 조사 프로브 결과 블록 (결정론적 근거. arch 불일치/태그 없음 등을 강조)
function ProbeBlock({ findings }: { findings: string[] }) {
  if (!findings.length) return null;
  return (
    <>
      <p className="k muted" style={{ margin: "0 0 4px" }}>자동 조사 결과 (Probe · 결정론적)</p>
      <div className="logs">
        {findings.map((f, i) => {
          const alert = f.includes("⚠️") || f.includes("불일치") || f.includes("없음");
          return (
            <div key={i} style={alert ? { color: "var(--crit-text)" } : undefined}>{f}</div>
          );
        })}
      </div>
    </>
  );
}

export default function IncidentDetail() {
  const { id = "" } = useParams();
  const { data: inc, loading } = useAsync(() => fetchIncident(id), [id]);

  if (loading) return <Skeleton title rows={8} />;
  if (!inc) return (
    <div className="empty">
      인시던트를 찾을 수 없습니다. <Link to="/incidents" style={{ color: "var(--accent-text)" }}>목록으로</Link>
    </div>
  );

  const currentIdx = STATE_FLOW.indexOf(inc.state);

  // 구버전(string) / 신버전(object) runbook 형태를 모두 정규화
  const runbooks = (inc.evidence?.runbooks ?? []).map((r) =>
    typeof r === "string" ? { title: r } : r,
  );
  const runbooksWithBody = runbooks.filter((r) => r.body);
  const probeFindings = inc.evidence?.probeFindings ?? [];

  return (
    <>
      <p style={{ marginBottom: 8 }}>
        <Link to="/incidents" className="muted" style={{ display: "inline-flex", alignItems: "center", gap: 5 }}>
          <ArrowLeft size={14} aria-hidden /> Incidents
        </Link>
      </p>
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
          {isFailureState(inc.state) && <span className="step failed">{inc.state}</span>}
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
            <span style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
              <span className="confidence-bar"><div style={{ width: `${inc.diagnosis.confidence * 100}%` }} /></span>
              <span className="mono">{Math.round(inc.diagnosis.confidence * 100)}%</span>
              {evidenceBadge(inc.diagnosis.evidenceQuality)}
            </span>
          </div>
          {inc.diagnosis.evidenceQuality && inc.diagnosis.evidenceQuality !== "rich" && (
            <p className="muted" style={{ fontSize: 12.5, marginTop: -6, marginBottom: 12, display: "flex", gap: 6 }}>
              <Warning size={15} color="var(--warn)" aria-hidden style={{ flex: "none", marginTop: 2 }} />
              <span>근거(metric/log/event)가 제한적이라 <b>조사용 진단</b>입니다. 아래 "동시 발생 alert"와 함께 검토하세요.</span>
            </p>
          )}

          <h3>제안 조치 <span className="tag">AI는 제안만, 적용은 정책·승인 후</span></h3>
          <ul className="actions-list">
            {inc.diagnosis.proposedActions.map((a, idx) => (
              <li key={idx}>
                <span className="badge dim">{a.type}</span>{" "}
                <span className={`badge ${riskClass(a.risk)}`}>risk: {a.risk}</span>
                <div style={{ marginTop: 6 }}>{a.description}</div>
                {a.target && <div className="mono muted" style={{ marginTop: 4 }}>대상: {a.target}</div>}
              </li>
            ))}
          </ul>

          <div className="btn-row" style={{ marginTop: 14 }}>
            <button className="primary" disabled title="MVP-2 예정">승인</button>
            <button disabled title="MVP-2 예정">반려</button>
            {inc.prUrl && (
              <a href={inc.prUrl} target="_blank" rel="noreferrer">
                <button style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
                  PR 열기 <ArrowSquareOut size={14} aria-hidden />
                </button>
              </a>
            )}
          </div>
          <p className="muted" style={{ fontSize: 12, marginTop: 10 }}>
            승인/반려 액션은 MVP-2에서 활성화됩니다. (architecture.md §4.7 / §7)
          </p>
        </div>
      )}

      {/* 권장 조치 (룰/Runbook 기반). AI 진단이 없을 때(LLM 실패 등) 결정적 해결책을 제공 */}
      {!inc.diagnosis && (
        <div className="section">
          <h3>권장 조치 <span className="tag">룰 · Runbook 기반</span></h3>
          <p className="muted" style={{ fontSize: 12.5, marginTop: -6, marginBottom: 12, display: "flex", gap: 6 }}>
            <Warning size={15} color="var(--warn)" aria-hidden style={{ flex: "none", marginTop: 2 }} />
            <span>AI 진단(LLM)이 생성되지 않아 자동 RCA가 없습니다. 아래는 결정적 <b>룰 분류</b>와 매칭된 <b>Runbook</b>에 기반한 권장 조치입니다.</span>
          </p>
          {inc.rule && inc.rule.category !== "Unknown" && (
            <p style={{ marginTop: 0 }}>
              <span className="badge info">분류: {inc.rule.category}</span>{" "}
              {inc.rule.rationale && <span className="muted">{inc.rule.rationale}</span>}
            </p>
          )}
          {probeFindings.length > 0 && <div style={{ marginBottom: 12 }}><ProbeBlock findings={probeFindings} /></div>}
          {runbooksWithBody.length > 0 ? (
            runbooksWithBody.map((rb, i) => (
              <div key={i} style={{ marginTop: 10 }}>
                <div style={{ fontWeight: 600, marginBottom: 6, display: "flex", alignItems: "center", gap: 6 }}>
                  <BookOpen size={15} color="var(--accent-text)" aria-hidden /> {rb.title}
                </div>
                <div className="logs">
                  {rb.body!.split("\n").map((ln, j) => (
                    <div key={j} style={ln.startsWith("## ") ? { color: "var(--text)", fontWeight: 600, marginTop: 6 } : undefined}>
                      {ln.replace(/^##\s*/, "")}
                    </div>
                  ))}
                </div>
              </div>
            ))
          ) : (
            <p className="muted">매칭된 Runbook 본문이 없습니다. 위 근거(Events/Metrics)를 검토해 수동 조치하세요. (백엔드 재분석 시 Runbook 조치가 채워집니다.)</p>
          )}
        </div>
      )}

      {/* Evidence */}
      {inc.evidence && (
        <div className="section">
          <h3>근거 (Evidence)</h3>
          {probeFindings.length > 0 && (
            <div style={{ marginBottom: 12 }}><ProbeBlock findings={probeFindings} /></div>
          )}
          {inc.evidence.relatedAlerts && inc.evidence.relatedAlerts.length > 0 && (
            <>
              <p className="k muted" style={{ margin: "0 0 4px" }}>동시 발생 alert (상관 분석 입력)</p>
              <div className="logs">
                {inc.evidence.relatedAlerts.map((a, i) => (
                  <div key={i}><code>{a.alertname}</code>{a.namespace ? ` (${a.namespace})` : ""}{a.severity ? ` · ${a.severity}` : ""}{a.summary ? `: ${a.summary}` : ""}</div>
                ))}
              </div>
            </>
          )}
          {inc.evidence.gitContext && (
            <p className="mono muted">git: {inc.evidence.gitContext.repo}/{inc.evidence.gitContext.path} @ {inc.evidence.gitContext.lastCommit}</p>
          )}
          {runbooks.length > 0 && (
            <>
              <p className="k muted" style={{ margin: "0 0 4px" }}>매칭된 Runbook</p>
              <div>{runbooks.map((r, i) => <span key={i} className="badge ok" style={{ marginRight: 6 }}>{r.title}</span>)}</div>
            </>
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
              <div className="logs">{inc.evidence.events.map((e, i) => <div key={i}>{e}</div>)}</div>
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
