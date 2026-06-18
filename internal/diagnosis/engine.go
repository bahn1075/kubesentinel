package diagnosis

import (
	"encoding/json"
	"fmt"
	"strings"
	"kubesentinel-ai/internal/models"
)

// Engine은 AI를 사용하여 장애를 분석하는 핵심 엔진입니다.
type Engine struct {
	aiClient models.AIClient
}

// NewEngine은 새로운 Diagnosis Engine을 생성합니다.
func NewEngine(client models.AIClient) *Engine {
	return &Engine{
		aiClient: client,
	}
}

// Analyze는 EvidenceBundle을 기반로 AI 분석을 수행하고 결과를 반환합니다.
func (e *Engine) Analyze(bundle *models.EvidenceBundle) (*models.DiagnosisResult, error) {
	// 1. 프롬프트 생성 (Context + Question)
	prompt := "Analyze the following Kubernetes incident and provides a structured response in JSON format. " +
		"The JSON must contain 'root_cause', 'summary', and 'proposed_actions' (list of objects with 'type', 'description', 'target', 'risk')."
	
	contextBytes, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal evidence: %w", err)
	}
	context := string(contextBytes)

	// 2. AI 호출
	resp, err := e.aiClient.Chat(prompt, context)
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	// 3. AI 응답에서 JSON 추출 및 파싱
	result, err := e.parseJSONResponse(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	// 4. 결과 보정 (Incident ID 매핑)
	result.IncidentID = bundle.IncidentID

	return result, nil
}

// parseJSONResponse는 AI의 텍-스트 응답에서 JSON 부분만 찾아 구조체로 변환합니다.
func (e *Engine) parseJSONResponse(content string) (*models.DiagnosisResult, error) {
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonEnd < jsonStart {
		return nil, fmt.Errorf("no valid JSON found in AI response: %s", content)
	}

	jsonStr := content[jsonStart : jsonEnd+1]

	var result models.DiagnosisResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return &result, nil
}

