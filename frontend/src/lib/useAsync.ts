import { useEffect, useState } from "react";

// 간단한 데이터 로딩 훅. 백엔드 API 연동 후에도 그대로 사용 가능.
export function useAsync<T>(fn: () => Promise<T>, deps: unknown[] = [], refreshMs = 0): {
  data: T | undefined;
  loading: boolean;
  error: string | undefined;
} {
  const [data, setData] = useState<T>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  useEffect(() => {
    let alive = true;
    let timer: ReturnType<typeof setTimeout>;
    setLoading(true);
    async function load() {
      try {
        const d = await fn();
        if (alive) { setData(d); setError(undefined); }
      } catch (e) {
        if (alive) setError(String(e));
      } finally {
        if (alive) {
          setLoading(false);
          if (refreshMs > 0) timer = setTimeout(load, refreshMs);
        }
      }
    }
    void load();
    return () => {
      alive = false;
      clearTimeout(timer);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, refreshMs]);

  return { data, loading, error };
}
