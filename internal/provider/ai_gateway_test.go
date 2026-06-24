package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kubesentinel-ai/internal/config"
)

// TestChat_EndpointResolutionAndParse는 base 엔드포인트에 chat/completions가
// 정확히 한 번 붙는지, OpenAI 호환 응답이 파싱되는지 검증한다.
func TestChat_EndpointResolutionAndParse(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing/incorrect Authorization header: %q", r.Header.Get("Authorization"))
		}
		var req ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": `{"root_cause":"x"}`}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// base 엔드포인트에 trailing slash + /v1 형태로 주어도 정상 보정되어야 한다.
	g := NewAIGateway(&config.AIConfig{Endpoint: srv.URL + "/v1/", Model: "test", APIKey: "test-key"})

	resp, err := g.Chat("question", "context")
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if want := "/v1/chat/completions"; gotPath != want {
		t.Errorf("request path = %q, want %q", gotPath, want)
	}
	if !strings.Contains(resp.Content, "root_cause") {
		t.Errorf("unexpected content: %q", resp.Content)
	}
}
