package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"kubesentinel-ai/internal/config"
)

// AIClient는 LLM과 통신하는 인터페이스입니다.
type AIClient interface {
	Chat(prompt string, context string) (*ChatResponse, error)
}

// ChatResponse는 AI의 응답을 담는 구조체입니다.
type ChatResponse struct {
	Content string `json:"content"` // AI의 텍ext 응답
}

// AIGateway는 설정된 정보를 바탕로 AI 모델과 통신을 관리합니다.
type AIGateway struct {
	cfg    *config.AIConfig
	client *http.Client
}

// NewAIGateway는 새로운 AIGateway 인스턴스를 생성합니다.
func NewAIGateway(cfg *config.AIConfig) *AIGateway {
	return &AIGateway{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ChatRequest는 LLM에 보낼 요청 구조체입니다. (OpenAI 호환 스펙)
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Chat은 AI에게 프롬프트를 전달하고 응답을 받아옵s니다.
func (g *AIGateway) Chat(prompt string, context string) (*ChatResponse, error) {
	// 1. 프롬프트 엔지니어링: 컨텍스트와 질문을 결합
	fullPrompt := fmt.Sprintf("Context:\n%s\n\nQuestion: %s", context, prompt)

	reqBody := ChatRequest{
		Model: g.cfg.Model,
		Messages: []Message{
			{Role: "system", Content: "You are KubeSentinel AI, a Kubernetes expert. Provide structured analysis in JSON format if possible."},
			{Role: "user", Content: fullPrompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 2. API 호출 (OpenAI 호환 엔드포인트로 요청)
	// Endpoint가 /v1으로 끝나지 않을 경우를 대비해 경로를 조정할 수 있습니다.
	endpoint := g.cfg.Endpoint
	if endpoint != "" && endpoint[len(endpoint)-1] != '/' {
		// 이미 경로가 포함되어 있을 수 있으므로 체크 (예: http://localhost:11434/v1)
		if !contains(endpoint, "chat/completions") {
			endpoint = endpoint + "/chat/completions"
		}
	}
	if !contains(endpoint, "chat/completions") {
		endpoint = endpoint + "/chat/completions"
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if g.cfg.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.cfg.APIKey))
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call AI endpoint: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("AI API error (status %d): %s", resp.StatusCode, string(body))
	}
	defer resp.Body.Close()

	// 3. 응답 파싱 (OpenAI 호환 스펙)
	var apiResp struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty choices in AI response")
	}

	return &ChatResponse{
		Content: apiResp.Choices[0].Message.Content,
	}, nil
}

// helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}
