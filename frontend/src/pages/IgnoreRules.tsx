import { useEffect, useState, useCallback } from "react";
import { fetchIgnores, addIgnore, setIgnoreEnabled, deleteIgnore } from "../api/client";
import type { IgnoreList } from "../api/types";
import Skeleton from "../components/Skeleton";

// 무시 규칙 관리 화면.
// 사용자가 keyword를 입력하면 alert명 또는 대상(네임스페이스/워크로드/파드/라벨)에
// 부분일치(양쪽 와일드카드 *keyword*)하는 alert를 인시던트로 처리하지 않는다.
export default function IgnoreRules() {
  const [data, setData] = useState<IgnoreList | null>(null);
  const [keyword, setKeyword] = useState("");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      setData(await fetchIgnores());
      setError(undefined);
    } catch (e) {
      setError(String(e));
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function run(fn: () => Promise<unknown>) {
    setBusy(true);
    setError(undefined);
    try {
      await fn();
      await load();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }

  const onAdd = () => {
    const k = keyword.trim();
    if (!k) return;
    run(() => addIgnore(k)).then(() => setKeyword(""));
  };

  return (
    <>
      <h1 className="page-title">무시 규칙 (Ignore rules)</h1>
      <p className="page-sub">
        입력한 <b>키워드</b>가 alert명 또는 대상(네임스페이스/워크로드/파드/라벨)에 <b>부분일치</b>(양쪽 와일드카드 <code>*키워드*</code>)하면,
        해당 alert는 <b>인시던트로 처리되지 않습니다.</b>
      </p>

      {error && <div className="test-result err" style={{ marginTop: 0, marginBottom: 14 }}>{error}</div>}

      <div className="section">
        <h3>규칙 추가</h3>
        <div className="btn-row" style={{ alignItems: "center", flexWrap: "nowrap" }}>
          <input
            type="text"
            placeholder="예: KubeCPUOvercommit, kafka-metrics, test-namespace"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && onAdd()}
            aria-label="무시 키워드"
            style={{ flex: 1 }}
          />
          <button className="primary" onClick={onAdd} disabled={busy || !keyword.trim()}>추가</button>
        </div>
        <p className="muted" style={{ fontSize: 12, marginTop: 10, marginBottom: 0 }}>대소문자 구분 없음. 같은 키워드를 다시 추가하면 활성화됩니다.</p>
      </div>

      <div className="section">
        <h3>등록된 규칙</h3>
        {!data ? (
          error
            ? <p className="muted" style={{ margin: 0 }}>규칙을 불러오지 못했습니다. 백엔드 연결 후 다시 시도하세요.</p>
            : <Skeleton rows={3} />
        ) : data.rules.length === 0 ? (
          <div className="empty" style={{ padding: "20px" }}>등록된 무시 규칙이 없습니다.</div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr><th>키워드</th><th>상태</th><th style={{ width: 170 }}>동작</th></tr>
              </thead>
              <tbody>
                {data.rules.map((r) => (
                  <tr key={r.id}>
                    <td className="mono">{r.keyword}</td>
                    <td>
                      {r.enabled
                        ? <span className="badge ok">활성</span>
                        : <span className="badge dim">해제됨</span>}
                    </td>
                    <td>
                      <div className="btn-row">
                        <button onClick={() => run(() => setIgnoreEnabled(r.id, !r.enabled))} disabled={busy}>
                          {r.enabled ? "해제" : "활성화"}
                        </button>
                        <button className="danger" onClick={() => run(() => deleteIgnore(r.id))} disabled={busy}>삭제</button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {data && data.config.length > 0 && (
        <div className="section">
          <h3>설정 고정 무시 <span className="tag">values/env · 읽기 전용</span></h3>
          <div>{data.config.map((c) => <span key={c} className="badge dim" style={{ marginRight: 6 }}>{c}</span>)}</div>
          <p className="muted" style={{ fontSize: 12, marginTop: 10, marginBottom: 0 }}>
            이 항목들은 배포 설정(helm values)으로 고정된 무시 목록입니다. 변경하려면 values의 <code>collector.ignoreAlerts</code>를 수정하세요.
          </p>
        </div>
      )}
    </>
  );
}
