import { Link, useNavigate } from "react-router-dom";
import { Siren, HourglassMedium, GitPullRequest, CheckCircle } from "@phosphor-icons/react";
import { fetchIncidents } from "../api/client";
import { useAsync } from "../lib/useAsync";
import { severityClass, stateClass, formatTime, isFailureState } from "../lib/format";
import Skeleton from "../components/Skeleton";

export default function Dashboard() {
  const nav = useNavigate();
  const { data: incidents, loading } = useAsync(fetchIncidents, [], 5000);

  if (loading || !incidents) return <Skeleton title rows={6} />;

  const open = incidents.filter((i) => !["Closed", "Verified"].includes(i.state) && !isFailureState(i.state)).length;
  const awaitingApproval = incidents.filter((i) => i.state === "ApprovalPending").length;
  const prs = incidents.filter((i) => i.prUrl).length;
  const resolved = incidents.filter((i) => i.state === "Verified" || i.state === "Closed").length;

  const tiles = [
    { label: "진행 중 인시던트", value: open, icon: Siren, color: "var(--crit-text)" },
    { label: "승인 대기", value: awaitingApproval, icon: HourglassMedium, color: "var(--warn)" },
    { label: "생성된 PR", value: prs, icon: GitPullRequest, color: "var(--accent-text)" },
    { label: "해결 완료", value: resolved, icon: CheckCircle, color: "var(--ok-text)" },
  ];

  return (
    <>
      <div className="page-heading">
        <div>
          <p className="eyebrow">CLUSTER OVERVIEW</p>
          <h1 className="page-title">Dashboard</h1>
          <p className="page-sub">클러스터 장애 감지·진단·조치 현황</p>
        </div>
        <Link className="text-link" to="/incidents">인시던트 보기 <span aria-hidden>↗</span></Link>
      </div>

      <div className="cards">
        {tiles.map((t) => (
          <div className="card" key={t.label}>
            <div className="label">
              <t.icon size={15} color={t.color} aria-hidden /> {t.label}
            </div>
            <div className="kpi">{t.value}</div>
          </div>
        ))}
      </div>

      <div className="section-heading">
        <h2>최근 인시던트 <span className="count">{incidents.length}</span></h2>
        <span className="muted">감지된 신호와 처리 상태</span>
      </div>
      {incidents.length === 0 ? (
        <div className="empty">아직 감지된 인시던트가 없습니다.</div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr><th>Alert</th><th>대상</th><th>심각도</th><th>상태</th><th>감지 시각</th></tr>
            </thead>
            <tbody>
              {incidents.map((i) => (
                <tr key={i.incidentId} className="rowlink" onClick={() => nav(`/incidents/${i.incidentId}`)}>
                  <td><Link className="incident-link" to={`/incidents/${i.incidentId}`} onClick={(e) => e.stopPropagation()}><code>{i.alert}</code></Link></td>
                  <td><span className="resource-name mono">{i.workload}</span><span className="resource-namespace">{i.namespace}</span></td>
                  <td><span className={`badge ${severityClass(i.severity)}`}>{i.severity}</span></td>
                  <td><span className={`badge ${stateClass(i.state)}`}>{i.state}</span></td>
                  <td className="muted mono timestamp">{formatTime(i.createdAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
