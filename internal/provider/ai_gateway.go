package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"kubesentinel-ai/internal/config"
	"kubesentinel-ai/internal/models"
)

// AIGateway는 설정된 정보를 바탕으로 AI 모델과 통신을 관리합니다.
// models.AIClient 인터페이스를 구현합니다.
type AIGateway struct {
	cfg    *config.AIConfig
	client *http.Client
}

// NewAIGateway는 새로운 AIGateway 인스턴스를 생성합니다.
func NewAIGateway(cfg *config.AIConfig) *AIGateway {
	return &AIGateway{
		cfg: cfg,
		client: &http.Client{
			// 로컬 모델(LM Studio/Ollama 등)은 추론이 느릴 수 있어 넉넉히 둔다.
			Timeout: 90 * time.Second,
		},
	}
}

// ChatRequest는 LLM에 보낼 요청 구조체입니다. (OpenAI 호환 스펙)
// response_format은 백엔드마다 지원이 달라(예: LM Studio는 json_object 거부) 사용하지 않고,
// 프롬프트의 스키마 강제 + 관대한 파싱(diagnosis.parseJSONResponse)으로 대응한다.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Chat은 AI에게 단발 프롬프트를 전달하고 응답을 받아옵니다.
func (g *AIGateway) Chat(prompt string, context string) (*models.ChatResponse, error) {
	fullPrompt := fmt.Sprintf("Context:\n%s\n\nQuestion: %s", context, prompt)
	return g.ChatMessages([]models.ChatMessage{
		{Role: "system", Content: "You are KubeSentinel AI, a Kubernetes expert. Respond with valid JSON only."},
		{Role: "user", Content: fullPrompt},
	})
}

// ChatMessages는 다중 턴 대화를 OpenAI 호환 엔드포인트로 전송한다(agentic 루프·검증 패스용).
func (g *AIGateway) ChatMessages(messages []models.ChatMessage) (*models.ChatResponse, error) {
	msgs := make([]Message, 0, len(messages))
	for _, m := range messages {
		msgs = append(msgs, Message{Role: m.Role, Content: m.Content})
	}
	reqBody := ChatRequest{
		Model:       g.cfg.Model,
		Temperature: 0.1, // 결정성 ↑ (architecture.md §4.2)
		Messages:    msgs,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 2. API 호출 (OpenAI 호환 엔드포인트로 요청)
	// base endpoint(예: http://localhost:11434/v1)에 chat/completions 경로를 보정한다.
	endpoint := strings.TrimRight(g.cfg.Endpoint, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
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

	return &models.ChatResponse{
		Content: apiResp.Choices[0].Message.Content,
	}, nil
}
