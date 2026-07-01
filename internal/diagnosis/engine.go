package diagnosis

import (
	"encoding/json"
	"fmt"
	"kubesentinel-ai/internal/models"
	"strconv"
	"strings"
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

// Analyze는 EvidenceBundle을 기반으로 AI 분석을 수행하고 결과를 반환합니다.
func (e *Engine) Analyze(bundle *models.EvidenceBundle) (*models.DiagnosisResult, error) {
	// 1. 프롬프트 생성 (Context + Question)
	// 로컬 모델은 스키마를 흔들기 쉬워, 타입과 예시를 명시적으로 강제한다.
	prompt := "You are a Kubernetes SRE. Analyze the incident in the context and respond with ONLY a single JSON object, no markdown, with EXACTLY this shape and types:\n" +
		"{\n" +
		`  "root_cause": "<a single plain-text sentence>",` + "\n" +
		`  "summary": "<a short plain-text paragraph>",` + "\n" +
		`  "confidence": 0.0,` + "  // number between 0 and 1\n" +
		`  "proposed_actions": [ { "type": "git_pr|rollback|runtime_patch|suggestion", "description": "<text>", "target": "<resource or file path>", "risk": "low|medium|high" } ]` + "\n" +
		"}\n" +
		"IMPORTANT: 'root_cause' and 'summary' MUST be plain strings (never nested objects). Output valid JSON only.\n" +
		"\nDEEP ANALYSIS RULES:\n" +
		"1) CORRELATION: The context field 'related_alerts' lists OTHER alerts firing at the same time. You MUST correlate them. The true root cause may originate from a different alert/resource than this one (e.g., a failing CronJob/Job or a node problem can make control-plane targets look 'Down'). If a related alert better explains the situation, say so explicitly.\n" +
		"2) CONFIDENCE GATING: If 'metrics', 'logs', and 'events' are all empty, you are inferring from the alert name alone. In that case set confidence <= 0.4, phrase root_cause as a hypothesis (not a certainty), state the uncertainty and what evidence is missing in the summary, and make proposed_actions INVESTIGATION steps (type='suggestion', risk='low') — do NOT propose specific fixes (git_pr/rollback/runtime_patch) without evidence."

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

// parseJSONResponse는 AI의 텍스트 응답에서 JSON 부분만 찾아 구조체로 변환합니다.
func (e *Engine) parseJSONResponse(content string) (*models.DiagnosisResult, error) {
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonEnd < jsonStart {
		return nil, fmt.Errorf("no valid JSON found in AI response: %s", content)
	}

	jsonStr := content[jsonStart : jsonEnd+1]

	// 관대한 파싱: 로컬 모델이 문자열 대신 객체/숫자를 반환해도 견딘다.
	var raw struct {
		RootCause       json.RawMessage         `json:"root_cause"`
		Summary         json.RawMessage         `json:"summary"`
		Confidence      json.RawMessage         `json:"confidence"`
		ProposedActions []models.ProposedAction `json:"proposed_actions"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return &models.DiagnosisResult{
		RootCause:       asText(raw.RootCause),
		Summary:         asText(raw.Summary),
		Confidence:      asFloat(raw.Confidence),
		ProposedActions: raw.ProposedActions,
	}, nil
}

// asText는 RawMessage가 JSON 문자열이면 그 값을, 아니면(객체/배열 등) 원문 JSON을 반환한다.
func asText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

// asFloat는 RawMessage를 숫자로 해석한다(숫자 또는 숫자형 문자열 모두 허용).
func asFloat(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v
		}
	}
	return 0
}
