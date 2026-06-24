import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { fetchIncidents } from "../api/client";
import { useAsync } from "../lib/useAsync";
import { severityClass, stateClass, formatTime } from "../lib/format";

export default function Incidents() {
  const nav = useNavigate();
  const { data: incidents, loading } = useAsync(fetchIncidents);
  const [q, setQ] = useState("");

  if (loading || !incidents) return <p className="muted">로딩 중…</p>;

  const filtered = incidents.filter((i) =>
    `${i.alert} ${i.namespace} ${i.workload} ${i.state}`.toLowerCase().includes(q.toLowerCase()),
  );

  return (
    <>
      <h1 className="page-title">Incidents</h1>
      <p className="page-sub">감지된 모든 장애 신호와 진단·조치 진행 상태</p>

      <input
        placeholder="검색: alert / namespace / workload / state"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        style={{ width: "100%", maxWidth: 360, marginBottom: 14, padding: "8px 10px",
          background: "var(--bg-elev)", border: "1px solid var(--border)", borderRadius: 6, color: "var(--text)" }}
      />

      <table>
        <thead>
          <tr><th>시각</th><th>Incident</th><th>대상</th><th>심각도</th><th>신뢰도</th><th>상태</th></tr>
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
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
