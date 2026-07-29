import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { fetchIncidents, acknowledgeIncident } from "../api/client";
import { useAsync } from "../lib/useAsync";
import { severityClass, stateClass, formatTime } from "../lib/format";

export default function Incidents() {
  const nav = useNavigate();
  const { data: incidents, loading } = useAsync(fetchIncidents);
  const [q, setQ] = useState("");
  const [acked, setAcked] = useState<Set<string>>(new Set());

  if (loading || !incidents) return <p className="muted">로딩 중…</p>;

  const filtered = incidents.filter(
    (i) =>
      !acked.has(i.incidentId) &&
      `${i.alert} ${i.namespace} ${i.workload} ${i.state}`.toLowerCase().includes(q.toLowerCase()),
  );

  // 확인됨 처리: 즉시 목록에서 숨기고(낙관적) 백엔드에 반영. 실패 시 복구.
  async function ack(id: string) {
    setAcked((s) => new Set(s).add(id));
    try {
      await acknowledgeIncident(id);
    } catch {
      setAcked((s) => {
        const n = new Set(s);
        n.delete(id);
        return n;
      });
    }
  }

  return (
    <>
      <h1 className="page-title">Incidents</h1>
      <p className="page-sub">감지된 모든 장애 신호와 진단·조치 진행 상태 (확인됨 처리 시 목록에서 숨겨집니다)</p>

      <input
        placeholder="검색: alert / namespace / workload / state"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        style={{ width: "100%", maxWidth: 360, marginBottom: 14, padding: "8px 10px",
          background: "var(--bg-elev)", border: "1px solid var(--border)", borderRadius: 6, color: "var(--text)" }}
      />

      <table>
        <thead>
          <tr><th>시각</th><th>Incident</th><th>대상</th><th>심각도</th><th>신뢰도</th><th>상태</th><th>확인</th></tr>
        </thead>
        <tbody>
          {filtered.map((i) => (
            <tr key={i.incidentId} onClick={() => nav(`/incidents/${i.incidentId}`)}>
              <td className="muted">{formatTime(i.createdAt)}</td>
              <td><code>{i.incidentId}</code></td>
              <td className="mono">{i.namespace}/{i.workload}</td>
              <td><span className={`badge ${severityClass(i.severity)}`}>{i.severity}</span></td>
              <td className="muted">{i.diagnosis ? `${Math.round(i.diagnosis.confidence * 100)}%` : "—"}</td>
              <td><span className={`badge ${stateClass(i.state)}`}>{i.state}</span></td>
              <td onClick={(e) => e.stopPropagation()}>
                <label style={{ display: "inline-flex", alignItems: "center", gap: 6, cursor: "pointer" }}
                  title="확인됨으로 표시하고 목록에서 숨김">
                  <input type="checkbox" onChange={() => ack(i.incidentId)} /> 확인됨
                </label>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {filtered.length === 0 && <p className="muted" style={{ marginTop: 12 }}>표시할 인시던트가 없습니다.</p>}
    </>
  );
}
