package config

import (
	"fmt"
	"os"
)

// Config는 KubeSentinel AI의 전체 시스템 설정을 담는 루트 구조체입니다.
type Config struct {
	App       AppConfig       `yaml:"app"`
	AI        AIConfig        `yaml:"ai"`
	Collector CollectorConfig `yaml:"collector"`
	GitOps    GitOpsConfig    `yaml:"gitops"`
	Notifier  NotifierConfig  `yaml:"notifier"`
}

// CollectorConfig는 근거 수집(Signal Collector) 엔드포인트 설정입니다. (architecture.md §4.1)
// 모든 주소는 값으로 주입하며 코드에 하드코딩하지 않는다. (§2 설계 원칙)
type CollectorConfig struct {
	PrometheusURL string `yaml:"prometheus_url"` // e.g., http://prometheus.monitoring.svc:9090
	LokiURL       string `yaml:"loki_url"`       // e.g., http://loki.monitoring.svc:3100
	GrafanaURL    string `yaml:"grafana_url"`    // 알림 딥링크용 (선택)
	LogLines      int    `yaml:"log_lines"`      // Loki에서 가져올 최근 로그 라인 수
}

// AppConfig는 애플리케이션 자체의 기본 설정을 담습니다.
type AppConfig struct {
	LogLevel string `yaml:"log_level"`
	Port     int    `yaml:"port"`
}

// AIConfig는 LLM 및 AI Gateway 설정을 담습니다. (architecture.md §4.2 반영)
type AIConfig struct {
	ProviderType   string `yaml:"provider_type"` // e.g., "openai-compatible"
	Endpoint       string `yaml:"endpoint"`
	Model          string `yaml:"model"`
	APIKey         string `yaml:"api_key"`
	AllowExternal  bool   `yaml:"allow_external"`
	RedactSecrets  bool   `yaml:"redact_secrets"`
	MaxInputTokens int    `yaml:"max_input_tokens"`
}

// GitOpsConfig는 Git 연동 및 PR 생성 설정을 담습니다. (architecture.md §4.5 반영)
type GitOpsConfig struct {
	Provider     string   `yaml:"provider"` // e.g., "github", "gitlab"
	Repository   string   `yaml:"repository"`
	BaseBranch   string   `yaml:"base_branch"`
	AllowedPaths []string `yaml:"allowed_paths"`
	DeniedPaths  []string `yaml:"denied_paths"`
	Token        string   `yaml:"token"`
}

// NotifierConfig는 알림 채널 설정을 담습니다. (architecture.md §4.7 반영)
type NotifierConfig struct {
	Type    string `yaml:"type"` // e.g., "slack", "discord", "teams"
	Webhook string `yaml:"webhook"`
}

// LoadConfig는 환경 변수 또는 기본값을 기반으로 설정을 로드합니다.
func LoadConfig() (*Config, error) {
	// 기본값 설정 (Default values)
	cfg := &Config{
		App: AppConfig{
			LogLevel: "info",
			Port:     8080,
		},
		AI: AIConfig{
			ProviderType:   "openai-compatible",
			MaxInputTokens: 120000,
			RedactSecrets:  true,
		},
		Collector: CollectorConfig{
			LogLines: 50,
		},
		GitOps: GitOpsConfig{
			BaseBranch:   "main",
			AllowedPaths: []string{"/"},
		},
	}

	// 환경 변수 오버라이드 (Environment Variable Overrides)
	if val := os.Getenv("KUBESENTINEL_AI_API_KEY"); val != "" {
		cfg.AI.APIKey = val
	}
	if val := os.Getenv("KUBESENTINEL_AI_ENDPOINT"); val != "" {
		cfg.AI.Endpoint = val
	}
	if val := os.Getenv("KUBESENTINEL_AI_MODEL"); val != "" {
		cfg.AI.Model = val
	}
	if val := os.Getenv("KUBESENTINEL_AI_GIT_TOKEN"); val != "" {
		cfg.GitOps.Token = val
	}
	if val := os.Getenv("KUBESENTINEL_AI_NOTIFIER_TYPE"); val != "" {
		cfg.Notifier.Type = val
	}
	if val := os.Getenv("KUBESENTINEL_AI_NOTIFIER_WEBHOOK"); val != "" {
		cfg.Notifier.Webhook = val
	}
	if val := os.Getenv("KUBESENTINEL_AI_PROMETHEUS_URL"); val != "" {
		cfg.Collector.PrometheusURL = val
	}
	if val := os.Getenv("KUBESENTINEL_AI_LOKI_URL"); val != "" {
		cfg.Collector.LokiURL = val
	}
	if val := os.Getenv("KUBESENTINEL_AI_GRAFANA_URL"); val != "" {
		cfg.Collector.GrafanaURL = val
	}

	// 검증 (Validation)
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// Validate는 설정값의 유효성을 검사합니다.
func (c *Config) Validate() error {
	if c.AI.ProviderType == "" {
		return fmt.Errorf("ai.provider_type must be specified")
	}
	if c.AI.Endpoint == "" && c.AI.ProviderType == "openai-compatible" {
		return fmt.Errorf("ai.endpoint must be specified for openai-compatible provider")
	}
	return nil
}
