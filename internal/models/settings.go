package models

// AppSettings는 DB에 영속화되는 애플리케이션 설정입니다 (비민감 항목만).
// 민감정보(AI API key, notifier webhook, git token)는 DB에 저장하지 않고
// k8s Secret/env로 관리한다. (architecture.md R8 안전 가드)
// JSON 태그는 프론트엔드(frontend/src/api/types.ts)와 camelCase로 정합한다.
type AppSettings struct {
	AI        AISettings        `json:"ai"`
	Collector CollectorSettings `json:"collector"`
	Notifier  NotifierSettings  `json:"notifier"`
	GitOps    GitOpsSettings    `json:"gitops"`
}

type AISettings struct {
	Type          string `json:"type"`
	Endpoint      string `json:"endpoint"`
	Model         string `json:"model"`
	AllowExternal bool   `json:"allowExternal"`
	RedactSecrets bool   `json:"redactSecrets"`
}

type CollectorSettings struct {
	PrometheusURL string `json:"prometheusUrl"`
	LokiURL       string `json:"lokiUrl"`
	GrafanaURL    string `json:"grafanaUrl"`
}

type NotifierSettings struct {
	Type string `json:"type"`
}

type GitOpsSettings struct {
	Provider   string `json:"provider"`
	Repository string `json:"repository"`
	BaseBranch string `json:"baseBranch"`
}

// DefaultAppSettings는 DB에 저장된 설정이 없을 때 반환하는 기본값입니다.
func DefaultAppSettings() AppSettings {
	return AppSettings{
		AI:        AISettings{Type: "openai-compatible", RedactSecrets: true},
		Notifier:  NotifierSettings{Type: "slack"},
		GitOps:    GitOpsSettings{Provider: "github", BaseBranch: "main"},
		Collector: CollectorSettings{},
	}
}
