package models

import (
	"fmt"
	"time"
)

// EvidenceBundle은 한 incident에 대해 수집한 모든 근거를 담는 구조체입니다.
// (architecture.md §4.1 JSON 스키마)
type EvidenceBundle struct {
	IncidentID   string                   `json:"incident_id"`
	Source       string                   `json:"source"`
	Alert        string                   `json:"alert"`
	Namespace    string                   `json:"namespace"`
	Workload     string                   `json:"workload"`
	Pod          string                   `json:"pod"`
	Kind         string                   `json:"kind,omitempty"`
	Severity     string                   `json:"severity,omitempty"`
	Annotations  map[string]string        `json:"annotations,omitempty"`
	Metrics      []map[string]interface{} `json:"metrics"`
	Logs         []string                 `json:"logs"`
	Events       []string                 `json:"events"`
	ResourceYAML map[string]interface{}   `json:"resource_yaml"`
	GitContext   GitContext               `json:"git_context"`
	// 동시에 firing 중인 다른 alert들 (상관 분석용 컨텍스트). LLM에 함께 전달된다.
	RelatedAlerts []RelatedAlert `json:"related_alerts,omitempty"`
	// 결정론적 룰 분류(LLM 이전). LLM 컨텍스트에 prior로 포함된다.
	Rule *RuleResult `json:"rule_classification,omitempty"`
	// 매칭된 운영자 runbook(메타데이터/키워드 검색). 본문이 LLM 컨텍스트에 포함된다.
	Runbooks []RunbookMatch `json:"matched_runbooks,omitempty"`
}

// RunbookMatch는 인시던트에 매칭된 runbook입니다.
type RunbookMatch struct {
	Title    string `json:"title"`
	Category string `json:"category,omitempty"`
	Body     string `json:"body,omitempty"`
}

// RelatedAlert는 상관 분석을 위한 동시 발생 alert의 요약입니다.
type RelatedAlert struct {
	Alertname string `json:"alertname"`
	Namespace string `json:"namespace"`
	Severity  string `json:"severity"`
	Summary   string `json:"summary"`
}

// GitContext는 대상 워크로드의 git 매니페스트 컨텍스트입니다.
type GitContext struct {
	Repo       string `json:"repo"`
	Path       string `json:"path"`
	LastCommit string `json:"last_commit"`
}

// NewEvidenceBundle은 Alertmanager 페이로드의 첫 alert를 기반으로
// 최소 EvidenceBundle을 생성합니다. (Prom/Loki/Event 보강은 Collector 후속 단계)
// alert가 하나도 없으면 nil을 반환합니다.
func NewEvidenceBundle(payload AlertmanagerPayload) *EvidenceBundle {
	if len(payload.Alerts) == 0 {
		return nil
	}
	alert := payload.Alerts[0]

	alertName := alert.Labels["alertname"]
	namespace := alert.Labels["namespace"]
	pod := alert.Labels["pod"]
	severity := alert.Labels["severity"]

	// 워크로드/종류 추정: 라벨 우선순위대로. kind는 client-go 수집 시 분기용.
	workload := firstNonEmpty(
		alert.Labels["deployment"],
		alert.Labels["workload"],
		alert.Labels["statefulset"],
		alert.Labels["daemonset"],
		alert.Labels["job"],
		alert.Labels["job_name"],
		pod,
	)
	kind := ""
	switch {
	case alert.Labels["deployment"] != "":
		kind = "Deployment"
	case alert.Labels["statefulset"] != "":
		kind = "StatefulSet"
	case alert.Labels["daemonset"] != "":
		kind = "DaemonSet"
	case alert.Labels["job"] != "" || alert.Labels["job_name"] != "":
		kind = "Job"
	case pod != "":
		kind = "Pod"
	}

	source := payload.Receiver
	if source == "" {
		source = "alertmanager"
	}

	return &EvidenceBundle{
		IncidentID:   fmt.Sprintf("inc-%s-%s", time.Now().Format("20060102"), alertName),
		Source:       source,
		Alert:        alertName,
		Namespace:    namespace,
		Workload:     workload,
		Pod:          pod,
		Kind:         kind,
		Severity:     severity,
		Annotations:  alert.Annotations,
		Metrics:      []map[string]interface{}{},
		Logs:         []string{},
		Events:       []string{},
		ResourceYAML: map[string]interface{}{},
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
