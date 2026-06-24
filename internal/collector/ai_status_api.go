package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AIStatus는 현재 백엔드가 연결하도록 설정된(활성) AI 제공자 정보입니다.
type AIStatus struct {
	Endpoint     string `json:"endpoint"`
	Model        string `json:"model"`
	ProviderKind string `json:"providerKind"` // local | frontier | unknown
	ProviderName string `json:"providerName"` // LM Studio | Ollama | OpenAI ...
}

// AIHealth는 활성 제공자에 대한 health check 결과입니다.
type AIHealth struct {
	Healthy        bool     `json:"healthy"`
	LatencyMs      int64    `json:"latencyMs"`
	Models         []string `json:"models"`
	ModelAvailable bool     `json:"modelAvailable"` // 설정된 모델이 목록에 있는지
	Error          string   `json:"error,omitempty"`
}

// handleAIStatus는 활성 AI 제공자 정보를 반환합니다. GET /api/ai/status
func (s *WebhookServer) handleAIStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	kind, name := inferProvider(s.AI.Endpoint, s.AI.Model)
	writeJSON(w, http.StatusOK, AIStatus{
		Endpoint:     s.AI.Endpoint,
		Model:        s.AI.Model,
		ProviderKind: kind,
		ProviderName: name,
	})
}

// handleAIHealth는 백엔드에서 활성 제공자로 health check(GET /models)를 수행합니다.
// GET /api/ai/health
func (s *WebhookServer) handleAIHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	start := time.Now()
	models, err := fetchProviderModels(s.AI.Endpoint, s.AI.APIKey)
	res := AIHealth{LatencyMs: time.Since(start).Milliseconds(), Models: models}
	if err != nil {
		res.Healthy = false
		res.Error = err.Error()
	} else {
		res.Healthy = true
		for _, m := range models {
			if m == s.AI.Model {
				res.ModelAvailable = true
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, res)
}

// fetchProviderModels는 OpenAI 호환 GET {endpoint}/models로 모델 ID 목록을 가져옵니다.
func fetchProviderModels(endpoint, apiKey string) ([]string, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("AI endpoint가 설정되지 않았습니다")
	}
	url := strings.TrimRight(endpoint, "/") + "/models"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("엔드포인트 도달 불가: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("응답 파싱 실패: %w", err)
	}
	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// inferProvider는 endpoint/model로 제공자 종류와 이름을 추정합니다(휴리스틱).
func inferProvider(endpoint, _ string) (kind, name string) {
	e := strings.ToLower(endpoint)
	switch {
	case e == "":
		return "unknown", "(미설정)"
	case strings.Contains(e, "api.openai.com"):
		return "frontier", "OpenAI"
	case strings.Contains(e, "anthropic"):
		return "frontier", "Anthropic"
	case strings.Contains(e, "generativelanguage") || strings.Contains(e, "googleapis"):
		return "frontier", "Google Gemini"
	case strings.Contains(e, "azure"):
		return "frontier", "Azure OpenAI"
	case strings.Contains(e, "mistral.ai"):
		return "frontier", "Mistral"
	case strings.Contains(e, "groq"):
		return "frontier", "Groq"
	case strings.Contains(e, ":11434"):
		return "local", "Ollama"
	case strings.Contains(e, ":1234"):
		return "local", "LM Studio"
	case strings.Contains(e, "vllm"):
		return "local", "vLLM"
	case strings.Contains(e, "localai"):
		return "local", "LocalAI"
	case strings.Contains(e, "localhost"), strings.Contains(e, "127.0.0.1"),
		strings.Contains(e, "host.minikube.internal"), strings.Contains(e, "host.docker.internal"),
		strings.Contains(e, ".svc"):
		return "local", "Local (self-hosted)"
	default:
		return "unknown", "Custom"
	}
}
