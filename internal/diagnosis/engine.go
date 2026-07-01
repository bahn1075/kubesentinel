package diagnosis

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"kubesentinel-ai/internal/models"
)

// ToolSpec는 LLM에 노출되는 read-only 근거 수집 도구의 설명입니다.
type ToolSpec struct {
	Name        string
	Description string
	Args        string // args 스키마 설명 (예: `{"query": "<PromQL>"}`)
}

// ToolRunner는 agentic 분석에서 LLM이 요청하는 도구를 실행합니다(collector가 구현).
// import cycle 방지를 위해 인터페이스를 diagnosis에 두고 collector가 구현·주입한다.
type ToolRunner interface {
	Specs() []ToolSpec
	Run(name string, args map[string]interface{}) string
}

// Engine은 AI를 사용하여 장애를 분석하는 핵심 엔진입니다.
type Engine struct {
	aiClient models.AIClient
	tools    ToolRunner // nil이면 단발 분석(도구 루프 없음)
	maxIter  int
}

// NewEngine은 새로운 Diagnosis Engine을 생성합니다. tools가 nil이면 agentic 루프를 건너뛴다.
func NewEngine(client models.AIClient, tools ToolRunner) *Engine {
	return &Engine{aiClient: client, tools: tools, maxIter: 3}
}

// 진단 결과 JSON 스키마 + 심층분석 규칙 (모든 최종 응답에 공통 적용) — L1
const schemaRules = "Respond with ONLY a single JSON object, no markdown, EXACTLY this shape and types:\n" +
	"{\n" +
	`  "root_cause": "<a single plain-text sentence>",` + "\n" +
	`  "summary": "<a short plain-text paragraph>",` + "\n" +
	`  "confidence": 0.0,` + "  // number 0..1\n" +
	`  "proposed_actions": [ { "type": "git_pr|rollback|runtime_patch|suggestion", "description": "<text>", "target": "<resource or file path>", "risk": "low|medium|high" } ]` + "\n" +
	"}\n" +
	"'root_cause' and 'summary' MUST be plain strings (never nested objects).\n" +
	"CORRELATION: 'related_alerts' in the evidence lists OTHER alerts firing simultaneously — correlate them; the true cause may originate from a different alert/resource (e.g., a failing CronJob/Job or node problem making control-plane targets look 'Down').\n" +
	"CONFIDENCE GATING: if metrics, logs, events and resource_status are all empty, you are guessing from the alert name — set confidence <= 0.4, phrase root_cause as a hypothesis, state what evidence is missing, and make proposed_actions INVESTIGATION steps (type='suggestion', risk='low'), not specific fixes."

// Analyze는 EvidenceBundle을 기반으로 AI 분석을 수행합니다.
// tools가 있으면 agentic(도구 수집) 루프 후 검증, 없으면 단발 후 검증.
func (e *Engine) Analyze(bundle *models.EvidenceBundle) (*models.DiagnosisResult, error) {
	contextBytes, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal evidence: %w", err)
	}
	evidenceJSON := string(contextBytes)

	var result *models.DiagnosisResult
	if e.tools != nil {
		result, err = e.analyzeAgentic(evidenceJSON)
	} else {
		result, err = e.analyzeSingle(evidenceJSON)
	}
	if err != nil {
		return nil, err
	}

	// 검증 패스 (L3): 근거 대비 진단을 비판·보정 (실패 시 원본 유지)
	result = e.verify(evidenceJSON, result)
	result.IncidentID = bundle.IncidentID
	return result, nil
}

// analyzeSingle: 단발 진단 (도구 없음/로컬 fallback).
func (e *Engine) analyzeSingle(evidenceJSON string) (*models.DiagnosisResult, error) {
	resp, err := e.aiClient.ChatMessages([]models.ChatMessage{
		{Role: "system", Content: "You are a Kubernetes SRE. " + schemaRules},
		{Role: "user", Content: "INCIDENT EVIDENCE (JSON):\n" + evidenceJSON + "\n\nProduce the diagnosis JSON."},
	})
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}
	return e.parseJSONResponse(resp.Content)
}

// analyzeAgentic: LLM이 read-only 도구로 근거를 스스로 수집(최대 maxIter)한 뒤 최종 진단.
func (e *Engine) analyzeAgentic(evidenceJSON string) (*models.DiagnosisResult, error) {
	sys := "You are a Kubernetes SRE performing a root-cause investigation.\n" +
		"You may gather MORE read-only evidence using tools before concluding.\n" +
		"AVAILABLE TOOLS:\n" + toolList(e.tools.Specs()) +
		"\nPROTOCOL: To call a tool respond with ONLY {\"tool\":\"<name>\",\"args\":{...},\"reason\":\"<why>\"}.\n" +
		"When you have enough evidence, respond with the FINAL diagnosis JSON instead.\n" +
		"FINAL FORMAT: " + schemaRules

	messages := []models.ChatMessage{
		{Role: "system", Content: sys},
		{Role: "user", Content: "INCIDENT EVIDENCE (JSON):\n" + evidenceJSON +
			"\n\nInvestigate. Call a tool to gather evidence, or output the FINAL diagnosis JSON if confident."},
	}

	for i := 0; i < e.maxIter; i++ {
		resp, err := e.aiClient.ChatMessages(messages)
		if err != nil {
			return nil, fmt.Errorf("AI analysis failed: %w", err)
		}
		if name, args, ok := parseToolRequest(resp.Content); ok {
			out := e.tools.Run(name, args)
			fmt.Printf("[KubeSentinel] 🔧 tool: %s → %d chars\n", name, len(out))
			messages = append(messages,
				models.ChatMessage{Role: "assistant", Content: resp.Content},
				models.ChatMessage{Role: "user", Content: "TOOL RESULT [" + name + "]:\n" + truncate(out, 4000)})
			continue
		}
		if r, err := e.parseJSONResponse(resp.Content); err == nil && r.RootCause != "" {
			return r, nil
		}
		messages = append(messages,
			models.ChatMessage{Role: "assistant", Content: resp.Content},
			models.ChatMessage{Role: "user", Content: "That was neither a valid tool call nor final JSON. Call a tool OR output the FINAL diagnosis JSON."})
	}

	// 도구 루프가 수렴하지 않으면 최종 응답 강제
	messages = append(messages, models.ChatMessage{Role: "user",
		Content: "STOP investigating. Output the FINAL diagnosis JSON now (schema only, no tool)."})
	resp, err := e.aiClient.ChatMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}
	return e.parseJSONResponse(resp.Content)
}

// verify: 회의적 리뷰어로 진단을 근거 대비 검증·보정. 실패 시 원본 유지.
func (e *Engine) verify(evidenceJSON string, d *models.DiagnosisResult) *models.DiagnosisResult {
	dj, err := json.Marshal(d)
	if err != nil {
		return d
	}
	resp, err := e.aiClient.ChatMessages([]models.ChatMessage{
		{Role: "system", Content: "You are a skeptical senior SRE reviewer. Verify the proposed diagnosis against the evidence. " +
			"If root_cause is NOT supported by metrics/logs/events/resource_status, lower confidence and correct it. " +
			"Prefer explanations consistent with related_alerts. Return ONLY the corrected FINAL diagnosis JSON. " + schemaRules},
		{Role: "user", Content: "EVIDENCE:\n" + evidenceJSON + "\n\nPROPOSED DIAGNOSIS:\n" + string(dj) +
			"\n\nReturn the corrected FINAL diagnosis JSON."},
	})
	if err != nil {
		return d
	}
	if r, perr := e.parseJSONResponse(resp.Content); perr == nil && r.RootCause != "" {
		return r
	}
	return d
}

func toolList(specs []ToolSpec) string {
	var b strings.Builder
	for _, s := range specs {
		fmt.Fprintf(&b, "- %s: %s  args=%s\n", s.Name, s.Description, s.Args)
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…(truncated)"
}

// parseToolRequest는 응답이 도구 호출({"tool":...})인지 판별하고 name/args를 추출한다.
func parseToolRequest(content string) (string, map[string]interface{}, bool) {
	js := extractJSON(content)
	if js == "" {
		return "", nil, false
	}
	var req struct {
		Tool      string                 `json:"tool"`
		Args      map[string]interface{} `json:"args"`
		RootCause json.RawMessage        `json:"root_cause"`
	}
	if err := json.Unmarshal([]byte(js), &req); err != nil {
		return "", nil, false
	}
	// root_cause가 있으면 최종 응답으로 간주(도구 아님)
	if req.Tool == "" || len(req.RootCause) > 0 {
		return "", nil, false
	}
	if req.Args == nil {
		req.Args = map[string]interface{}{}
	}
	return req.Tool, req.Args, true
}

func extractJSON(content string) string {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return content[start : end+1]
}

// parseJSONResponse는 AI의 텍스트 응답에서 JSON 부분만 찾아 구조체로 변환합니다.
func (e *Engine) parseJSONResponse(content string) (*models.DiagnosisResult, error) {
	jsonStr := extractJSON(content)
	if jsonStr == "" {
		return nil, fmt.Errorf("no valid JSON found in AI response: %s", content)
	}

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
