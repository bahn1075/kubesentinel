import { useEffect, useState } from "react";

// 간단한 데이터 로딩 훅. 백엔드 API 연동 후에도 그대로 사용 가능.
export function useAsync<T>(fn: () => Promise<T>, deps: unknown[] = []): {
  data: T | undefined;
  loading: boolean;
  error: string | undefined;
} {
  const [data, setData] = useState<T>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  useEffect(() => {
    let alive = true;
    setLoading(true);
    fn()
      .then((d) => alive && setData(d))
      .catch((e) => alive && setError(String(e)))
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return { data, loading, error };
}
