import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { fetchIncidents, acknowledgeIncident, fetchIncidentCounts } from "../api/client";
import type { IncidentFilter, IncidentCounts } from "../api/types";
import { useAsync } from "../lib/useAsync";
import { severityClass, stateClass, formatTime } from "../lib/format";
import Skeleton from "../components/Skeleton";

const TABS: { key: IncidentFilter; label: string }[] = [
  { key: "open", label: "진행 중" },
  { key: "acknowledged", label: "확인됨" },
  { key: "all", label: "전체" },
];

export default function Incidents() {
  const nav = useNavigate();
  const [tab, setTab] = useState<IncidentFilter>("open");
  const { data: incidents, loading } = useAsync(() => fetchIncidents(tab), [tab]);
  const [q, setQ] = useState("");
  const [acked, setAcked] = useState<Set<string>>(new Set());
  const [counts, setCounts] = useState<IncidentCounts | null>(null);

  // 탭 배지 건수. 확인됨 처리 후에도 갱신한다.
  const loadCounts = useCallback(() => {
    fetchIncidentCounts().then(setCounts).catch(() => setCounts(null));
  }, []);
  useEffect(loadCounts, [loadCounts]);

  // 탭을 옮기면 낙관적으로 숨겨둔 목록을 비운다(해당 탭에서는 다시 보여야 한다).
  useEffect(() => setAcked(new Set()), [tab]);

  if (loading || !incidents) return <Skeleton title rows={7} />;

  const filtered = incidents.filter(
    (i) =>
      !acked.has(i.incidentId) &&
      `${i.alert} ${i.namespace} ${i.workload} ${i.state}`.toLowerCase().includes(q.toLowerCase()),
  );

  // 확인됨 토글. 현재 탭에서 사라져야 하는 경우에만 낙관적으로 숨긴다
  // ("전체" 탭에서는 확인 여부와 무관하게 계속 보여야 한다).
  async function toggleAck(id: string, next: boolean) {
    const hideFromThisTab = tab !== "all";
    if (hideFromThisTab) setAcked((s) => new Set(s).add(id));
    try {
      await acknowledgeIncident(id, next);
      loadCounts();
    } catch {
      if (hideFromThisTab) {
        setAcked((s) => {
          const n = new Set(s);
          n.delete(id);
          return n;
        });
      }
    }
  }

  return (
    <>
      <h1 className="page-title">Incidents</h1>
      <p className="page-sub">
        감지된 장애 신호와 진단·조치 진행 상태. 확인됨 처리한 인시던트는
        <b> 확인됨</b> 탭에서 다시 볼 수 있습니다.
      </p>

      <div className="seg" role="tablist" aria-label="인시던트 목록 필터" style={{ marginBottom: 16 }}>
        {TABS.map((t) => (
          <button
            key={t.key}
            type="button"
            role="tab"
            aria-selected={tab === t.key}
            className={tab === t.key ? "active" : ""}
            onClick={() => setTab(t.key)}
          >
            {t.label}
            {counts && <span className="seg-count">{counts[t.key]}</span>}
          </button>
        ))}
      </div>

      <input
        type="text"
        placeholder="검색: alert / namespace / workload / state"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        style={{ maxWidth: 380, marginBottom: 16 }}
        aria-label="인시던트 검색"
      />

      {filtered.length === 0 ? (
        <div className="empty">
          {q
            ? "검색 조건에 맞는 인시던트가 없습니다."
            : tab === "acknowledged"
              ? "확인됨 처리한 인시던트가 없습니다."
              : "표시할 인시던트가 없습니다."}
        </div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr><th>시각</th><th>Incident</th><th>대상</th><th>심각도</th><th>신뢰도</th><th>상태</th><th>확인</th></tr>
            </thead>
            <tbody>
              {filtered.map((i) => (
                <tr key={i.incidentId} className="rowlink" onClick={() => nav(`/incidents/${i.incidentId}`)}>
                  <td className="muted mono">{formatTime(i.createdAt)}</td>
                  <td><code>{i.incidentId}</code></td>
                  <td className="mono">{i.namespace}/{i.workload}</td>
                  <td><span className={`badge ${severityClass(i.severity)}`}>{i.severity}</span></td>
                  <td className="muted mono">{i.diagnosis ? `${Math.round(i.diagnosis.confidence * 100)}%` : "-"}</td>
                  <td><span className={`badge ${stateClass(i.state)}`}>{i.state}</span></td>
                  <td onClick={(e) => e.stopPropagation()}>
                    {tab === "acknowledged" ? (
                      <button type="button" onClick={() => toggleAck(i.incidentId, false)}
                        title="확인을 취소하고 진행 중 목록으로 되돌림">
                        확인 취소
                      </button>
                    ) : (
                      <label style={{ display: "inline-flex", alignItems: "center", gap: 6, cursor: "pointer" }}
                        title="확인됨으로 표시하고 진행 중 목록에서 숨김">
                        <input type="checkbox" onChange={() => toggleAck(i.incidentId, true)} /> 확인됨
                      </label>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
