package models

// DiagnosisResult는 LLM RCA 분석의 구조화된 결과입니다. (architecture.md §4.3)
type DiagnosisResult struct {
	IncidentID      string           `json:"incident_id"`
	RootCause       string           `json:"root_cause"`
	Summary         string           `json:"summary"`
	Confidence      float64          `json:"confidence,omitempty"`
	ProposedActions []ProposedAction `json:"proposed_actions"`
}

// ProposedAction은 진단이 제안하는 조치 후보 1건입니다.
// AI는 제안자일 뿐이며 적용 여부는 Policy/Approval이 결정합니다. (설계 원칙 1)
type ProposedAction struct {
	Type        string `json:"type"` // suggestion | git_pr | rollback | runtime_patch ...
	Description string `json:"description"`
	Target      string `json:"target"`
	Risk        string `json:"risk"` // low | medium | high
}
