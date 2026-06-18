package config

import (
	"fmt"
	"os"
)

// Config는 KubeSentinel AI의 전체 시스템 설정을 담는 루트 구조체입니다.
type Config struct {
	App      AppConfig      `yaml:"app"`
	AI       AIConfig       `yaml:"ai"`
	GitOps   GitOpsConfig   `yaml:"gitops"`
	Notifier NotifierConfig `yaml:"notifier"`
}

// AppConfig는 애플리케이션 자체의 기본 설정을 담습니다.
type AppConfig struct {
	LogLevel string `yaml:"log_level"`
	Port     int    `yaml:"port"`
}

// AIConfig는 LLM 및 AI Gateway 설정을 담습니다. (architecture.md §4.2 반영)
type AIConfig struct {
	ProviderType    string `yaml:"provider_type"` // e.g., "openai-compatible"
	Endpoint        string `yaml:"endpoint"`
	Model           string `yaml:"model"`
	APIKey          string `yaml:"api_key"`
	AllowExternal   bool   `yaml:"allow_external"`
	RedactSecrets   bool   `yaml:"redact_secrets"`
	MaxInputTokens  int    `yaml:"max_input_tokens"`
}

// GitOpsConfig는 Git 연동 및 PR 생성 설정을 담습니다. (architecture.md §4.5 반영)
type GitOpsConfig struct {
	Provider      string   `yaml:"provider"` // e.g., "github", "gitlab"
	Repository    string   `yaml:"repository"`
	BaseBranch    string   `yaml:"base_branch"`
	AllowedPaths  []string `yaml:"allowed_paths"`
	DeniedPaths   []string `yaml:"denied_paths"`
	Token         string   `yaml:"token"`
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
	if val := os.Getenv("KUBESENTINEL_AI_NOTIFIER_WEBHOOK"); val != "" {
		cfg.Notifier.Webhook = val
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
