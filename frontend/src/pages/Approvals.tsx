import { useNavigate } from "react-router-dom";
import { fetchIncidents } from "../api/client";
import { useAsync } from "../lib/useAsync";
import { severityClass, formatTime } from "../lib/format";

// MVP-2 예정 기능. 현재는 승인 대기 상태 인시던트를 읽기 전용으로 보여준다.
export default function Approvals() {
  const nav = useNavigate();
  const { data: incidents, loading } = useAsync(fetchIncidents);

  if (loading || !incidents) return <p className="muted">로딩 중…</p>;
  const pending = incidents.filter((i) => i.state === "ApprovalPending");

  return (
    <>
      <h1 className="page-title">Approvals <span className="tag">MVP-2 예정</span></h1>
      <p className="page-sub">정책상 승인이 필요한 조치 대기 목록. 승인/반려 액션은 추후 활성화됩니다.</p>

      {pending.length === 0 ? (
        <div className="section muted">승인 대기 중인 조치가 없습니다.</div>
      ) : (
        <table>
          <thead>
            <tr><th>시각</th><th>Incident</th><th>대상</th><th>심각도</th><th>제안</th><th></th></tr>
          </thead>
          <tbody>
            {pending.map((i) => (
              <tr key={i.incidentId} onClick={() => nav(`/incidents/${i.incidentId}`)}>
                <td className="muted">{formatTime(i.createdAt)}</td>
                <td><code>{i.alert}</code></td>
                <td className="mono">{i.namespace}/{i.workload}</td>
                <td><span className={`badge ${severityClass(i.severity)}`}>{i.severity}</span></td>
                <td className="muted">{i.diagnosis?.proposedActions[0]?.description ?? "—"}</td>
                <td className="muted">상세 →</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}
