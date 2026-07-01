package models

import "time"

// IncidentView는 프론트엔드(frontend/src/api/types.ts의 Incident)와 정합하는
// camelCase API 응답 모델이다. DB에는 이 형태(JSONB)로 영속화한다.
type IncidentView struct {
	IncidentID string         `json:"incidentId"`
	Alert      string         `json:"alert"`
	Namespace  string         `json:"namespace"`
	Workload   string         `json:"workload"`
	Pod        string         `json:"pod,omitempty"`
	Severity   string         `json:"severity"`
	State      string         `json:"state"`
	CreatedAt  time.Time      `json:"createdAt"`
	Diagnosis  *DiagnosisView `json:"diagnosis,omitempty"`
	Evidence   *EvidenceView  `json:"evidence,omitempty"`
	PRURL      string         `json:"prUrl,omitempty"`
}

type DiagnosisView struct {
	RootCause       string           `json:"rootCause"`
	Summary         string           `json:"summary"`
	Confidence      float64          `json:"confidence"`
	ProposedActions []ProposedAction `json:"proposedActions"`
	// none|partial|rich — 코드로 계산한 근거 품질(LLM 자기신고 아님).
	EvidenceQuality string `json:"evidenceQuality,omitempty"`
}

type EvidenceView struct {
	Metrics       []map[string]interface{} `json:"metrics"`
	Logs          []string                 `json:"logs"`
	Events        []string                 `json:"events"`
	GitContext    *GitContextView          `json:"gitContext,omitempty"`
	RelatedAlerts []RelatedAlert           `json:"relatedAlerts,omitempty"`
}

type GitContextView struct {
	Repo       string `json:"repo"`
	Path       string `json:"path"`
	LastCommit string `json:"lastCommit"`
}

// NewIncidentView는 수집 근거(+선택적 진단)로부터 API/저장용 뷰를 만든다.
func NewIncidentView(b *EvidenceBundle, d *DiagnosisResult, state string) IncidentView {
	v := IncidentView{
		IncidentID: b.IncidentID,
		Alert:      b.Alert,
		Namespace:  b.Namespace,
		Workload:   b.Workload,
		Pod:        b.Pod,
		Severity:   b.Severity,
		State:      state,
		CreatedAt:  time.Now().UTC(),
		Evidence: &EvidenceView{
			Metrics:       b.Metrics,
			Logs:          b.Logs,
			Events:        b.Events,
			RelatedAlerts: b.RelatedAlerts,
		},
	}
	if b.GitContext.Repo != "" || b.GitContext.Path != "" {
		v.Evidence.GitContext = &GitContextView{
			Repo:       b.GitContext.Repo,
			Path:       b.GitContext.Path,
			LastCommit: b.GitContext.LastCommit,
		}
	}
	if d != nil {
		v.Diagnosis = &DiagnosisView{
			RootCause:       d.RootCause,
			Summary:         d.Summary,
			Confidence:      d.Confidence,
			ProposedActions: d.ProposedActions,
			EvidenceQuality: evidenceQuality(b),
		}
	}
	return v
}

// evidenceQuality는 수집된 근거의 충실도를 코드로 판정한다(LLM에 의존하지 않음).
//
//	none    — metric·log·event 모두 없음(alert 이름만으로 진단 = 조사용)
//	partial — 일부만 존재
//	rich    — metric과 log 모두 존재
func evidenceQuality(b *EvidenceBundle) string {
	m := len(b.Metrics) > 0
	l := len(b.Logs) > 0
	e := len(b.Events) > 0
	switch {
	case m && l:
		return "rich"
	case m || l || e:
		return "partial"
	default:
		return "none"
	}
}
