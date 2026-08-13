// 로딩 스켈레톤: 최종 레이아웃의 형태(제목 + 행 목록)를 따라간다.
export default function Skeleton({ rows = 5, title = false }: { rows?: number; title?: boolean }) {
  return (
    <div className="skeleton" role="status" aria-label="불러오는 중">
      {title && <div className="sk-row lg" />}
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="sk-row" style={{ maxWidth: `${88 - (i % 3) * 9}%` }} />
      ))}
    </div>
  );
}
