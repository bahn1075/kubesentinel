package collector

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// fetchProviderModels가 endpoint에 이미 /models가 붙어 있어도(LM Studio 예시 URL을 그대로 붙여넣는 경우)
// /models/models로 중복 요청하지 않는지 검증한다.
func TestFetchProviderModels_NoDoubleModelsSuffix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"unexpected path: ` + r.URL.Path + `"}`))
			return
		}
		w.Write([]byte(`{"data":[{"id":"m1"},{"id":"m2"}]}`))
	}))
	defer srv.Close()

	for _, ep := range []string{srv.URL + "/v1", srv.URL + "/v1/models", srv.URL + "/v1/models/"} {
		ids, err := fetchProviderModels(ep, "")
		if err != nil {
			t.Fatalf("endpoint %q: unexpected error: %v", ep, err)
		}
		if len(ids) != 2 || ids[0] != "m1" || ids[1] != "m2" {
			t.Fatalf("endpoint %q: unexpected models: %v", ep, ids)
		}
	}
}
