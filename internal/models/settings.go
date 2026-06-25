package models

// AppSettings는 DB에 영속화되는 애플리케이션 설정입니다 (비민감 항목만).
// 민감정보(AI API key, git token)는 app_secrets 테이블에 write-only로 저장한다.
// JSON 태그는 프론트엔드(frontend/src/api/types.ts)와 camelCase로 정합한다.
type AppSettings struct {
	AI        AISettings        `json:"ai"`
	Collector CollectorSettings `json:"collector"`
	Notifier  NotifierSettings  `json:"notifier"`
	Git       GitSettings       `json:"git"`
}

type AISettings struct {
	Kind          string `json:"kind"`     // frontier | local
	Provider      string `json:"provider"` // (frontier) openai | anthropic | azure-openai | google | custom
	Type          string `json:"type"`     // API 형식: openai-compatible (현재 백엔드가 지원)
	Endpoint      string `json:"endpoint"` // (local) base URL, (frontier) provider API base
	Model         string `json:"model"`
	AuthMethod    string `json:"authMethod"` // (frontier) api-key | oauth | machine
	AllowExternal bool   `json:"allowExternal"`
	RedactSecrets bool   `json:"redactSecrets"`
}

type CollectorSettings struct {
	PrometheusURL   string `json:"prometheusUrl"`
	LokiURL         string `json:"lokiUrl"`
	AlertmanagerURL string `json:"alertmanagerUrl"`
	GrafanaURL      string `json:"grafanaUrl"`
}

type NotifierSettings struct {
	Type string `json:"type"`
}

// GitSettings: 추후 git 직접 업데이트(PR/commit) 대상 repo 설정.
type GitSettings struct {
	Provider   string `json:"provider"`   // github | gitlab | gitea
	AuthMethod string `json:"authMethod"` // token | oauth | ssh
	Repository string `json:"repository"`
	BaseBranch string `json:"baseBranch"`
}

// DefaultAppSettings는 DB에 저장된 설정이 없을 때 반환하는 기본값입니다.
func DefaultAppSettings() AppSettings {
	return AppSettings{
		AI: AISettings{
			Kind:          "local",
			Type:          "openai-compatible",
			AuthMethod:    "api-key",
			RedactSecrets: true,
		},
		Collector: CollectorSettings{},
		Notifier:  NotifierSettings{Type: "slack"},
		Git:       GitSettings{Provider: "github", AuthMethod: "token", BaseBranch: "main"},
	}
}

// 시크릿 이름 상수 (app_secrets 키)
const (
	SecretAIAPIKey = "ai_api_key"
	SecretGitToken = "git_token"
)
