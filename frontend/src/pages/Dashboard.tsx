import { useNavigate } from "react-router-dom";
import { fetchIncidents } from "../api/client";
import { useAsync } from "../lib/useAsync";
import { severityClass, stateClass, formatTime, isFailureState } from "../lib/format";

export default function Dashboard() {
  const nav = useNavigate();
  const { data: incidents, loading } = useAsync(fetchIncidents);

  if (loading || !incidents) return <p className="muted">로딩 중…</p>;

  const open = incidents.filter((i) => !["Closed", "Verified"].includes(i.state) && !isFailureState(i.state)).length;
  const awaitingApproval = incidents.filter((i) => i.state === "ApprovalPending").length;
  const prs = incidents.filter((i) => i.prUrl).length;
  const resolved = incidents.filter((i) => i.state === "Verified" || i.state === "Closed").length;

  return (
    <>
      <h1 className="page-title">Dashboard</h1>
      <p className="page-sub">클러스터 장애 감지·진단·조치 현황 요약</p>

      <div className="cards">
        <div className="card"><div className="kpi">{open}</div><div className="label">진행 중 인시던트</div></div>
        <div className="card"><div className="kpi">{awaitingApproval}</div><div className="label">승인 대기</div></div>
        <div className="card"><div className="kpi">{prs}</div><div className="label">생성된 PR</div></div>
        <div className="card"><div className="kpi">{resolved}</div><div className="label">해결 완료</div></div>
      </div>

      <h3 style={{ margin: "0 0 10px" }}>최근 인시던트</h3>
      <table>
        <thead>
          <tr><th>시각</th><th>Alert</th><th>대상</th><th>심각도</th><th>상태</th></tr>
        </thead>
        <tbody>
          {incidents.map((i) => (
            <tr key={i.incidentId} onClick={() => nav(`/incidents/${i.incidentId}`)}>
              <td className="muted">{formatTime(i.createdAt)}</td>
              <td><code>{i.alert}</code></td>
              <td className="mono">{i.namespace}/{i.workload}</td>
              <td><span className={`badge ${severityClass(i.severity)}`}>{i.severity}</span></td>
              <td><span className={`badge ${stateClass(i.state)}`}>{i.state}</span></td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
