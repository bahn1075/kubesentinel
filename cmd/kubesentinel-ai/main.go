package main

import (
	"fmt"
	"kubesentinel-ai/internal/collector"
	"kubesentinel-ai/internal/config"
	"kubesentinel-ai/internal/diagnosis"
	"kubesentinel-ai/internal/models"
	"kubesentinel-ai/internal/notifier"
	"kubesentinel-ai/internal/provider"
	"kubesentinel-ai/internal/store"
	"log"
)

func main() {
	fmt.Println("Starting KubeSentinel AI...")

	// 1. Load Configuration (env + 기본값)
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	fmt.Printf("Configuration loaded successfully (LogLevel: %s, Port: %d)\n", cfg.App.LogLevel, cfg.App.Port)

	// 2. Settings Store (Postgres). DATABASE_URL 미설정 시 비활성(설정 API 503).
	var st *store.Store
	if cfg.Database.URL != "" {
		st, err = store.New(cfg.Database.URL)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		if err := st.Migrate(); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
		fmt.Println("Database connected and migrations applied.")

		// 2-1. DB에 저장된 설정을 cfg에 병합한다 (비민감 항목). (implementation-status §3.6)
		// 민감정보(API key/webhook/token)는 계속 env/Secret에서 가져온다.
		if s, err := st.GetSettings(); err != nil {
			log.Printf("warning: failed to load settings from DB, using env config: %v", err)
		} else {
			applyDBSettings(cfg, s)
			fmt.Println("Applied settings from database (non-sensitive overrides).")
		}

		// 시크릿(write-only DB) 로드 → cfg 주입 (env보다 우선)
		if v, ok, _ := st.GetSecret(models.SecretAIAPIKey); ok && v != "" {
			cfg.AI.APIKey = v
			fmt.Println("Loaded AI API key from database secret.")
		}
		if v, ok, _ := st.GetSecret(models.SecretGitToken); ok && v != "" {
			cfg.GitOps.Token = v
		}
	} else {
		fmt.Println("DATABASE_URL not set — settings persistence disabled.")
	}

	fmt.Printf("AI Provider: %s (Endpoint: %s, Model: %s)\n", cfg.AI.ProviderType, cfg.AI.Endpoint, cfg.AI.Model)

	// 3. Initialize Components (병합된 cfg 기준)
	aiGateway := provider.NewAIGateway(&cfg.AI)
	engine := diagnosis.NewEngine(aiGateway)
	enricher := collector.NewEnricher(cfg.Collector)
	notify, err := notifier.New(cfg.Notifier, cfg.Collector.GrafanaURL)
	if err != nil {
		log.Fatalf("Failed to initialize notifier: %v", err)
	}

	// 4. Webhook/API Server
	server := collector.NewWebhookServer(fmt.Sprintf("%d", cfg.App.Port), engine, enricher, notify, st, cfg.AI)
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Webhook server failed to start: %v", err)
		}
	}()

	fmt.Println("KubeSentinel AI is now running. Press Ctrl+C to exit.")
	select {}
}

// applyDBSettings는 DB에 저장된 비민감 설정을 cfg에 덮어쓴다(빈 문자열은 건너뜀).
// bool 값(allowExternal/redactSecrets)은 DB를 권위 소스로 본다.
func applyDBSettings(cfg *config.Config, s models.AppSettings) {
	if s.AI.Type != "" {
		cfg.AI.ProviderType = s.AI.Type
	}
	if s.AI.Endpoint != "" {
		cfg.AI.Endpoint = s.AI.Endpoint
	}
	if s.AI.Model != "" {
		cfg.AI.Model = s.AI.Model
	}
	cfg.AI.AllowExternal = s.AI.AllowExternal
	cfg.AI.RedactSecrets = s.AI.RedactSecrets

	if s.Collector.PrometheusURL != "" {
		cfg.Collector.PrometheusURL = s.Collector.PrometheusURL
	}
	if s.Collector.LokiURL != "" {
		cfg.Collector.LokiURL = s.Collector.LokiURL
	}
	if s.Collector.GrafanaURL != "" {
		cfg.Collector.GrafanaURL = s.Collector.GrafanaURL
	}

	if s.Notifier.Type != "" {
		cfg.Notifier.Type = s.Notifier.Type
	}

	if s.Git.Provider != "" {
		cfg.GitOps.Provider = s.Git.Provider
	}
	if s.Git.Repository != "" {
		cfg.GitOps.Repository = s.Git.Repository
	}
	if s.Git.BaseBranch != "" {
		cfg.GitOps.BaseBranch = s.Git.BaseBranch
	}
}
