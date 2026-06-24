import { fetchPolicies } from "../api/client";
import { useAsync } from "../lib/useAsync";

// RemediationPolicy 뷰 (architecture.md §9). 현재는 읽기 전용 표시.
export default function Policies() {
  const { data: policies, loading } = useAsync(fetchPolicies);
  if (loading || !policies) return <p className="muted">로딩 중…</p>;

  return (
    <>
      <h1 className="page-title">Policies</h1>
      <p className="page-sub">조치 허용 범위·승인 조건을 정의하는 RemediationPolicy</p>

      {policies.map((p) => (
        <div className="section" key={p.name}>
          <h3>{p.name} <span className="badge dim">{p.mode}</span></h3>
          <div className="kv">
            <span className="k">Allowed paths</span>
            <span className="mono">{p.allowedPaths.join(", ")}</span>
            <span className="k">Denied paths</span>
            <span className="mono">{p.deniedPaths.join(", ")}</span>
            <span className="k">Allowed actions</span>
            <span>{p.allowedActions.map((a) => <span key={a} className="badge ok" style={{ marginRight: 4 }}>{a}</span>)}</span>
            <span className="k">Approval 필요</span>
            <span>{p.approvalRequiredFor.map((a) => <span key={a} className="badge warn" style={{ marginRight: 4 }}>{a}</span>)}</span>
            <span className="k">PR 최소 신뢰도</span>
            <span>{Math.round(p.minConfidenceForPR * 100)}%</span>
          </div>
        </div>
      ))}
      <p className="muted" style={{ fontSize: 12 }}>정책 편집은 추후 지원됩니다. 현재는 클러스터의 RemediationPolicy를 읽기 전용으로 표시합니다.</p>
    </>
  );
}
