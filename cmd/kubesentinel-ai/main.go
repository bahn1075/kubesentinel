package main

import (
	"fmt"
	"kubesentinel-ai/internal/collector"
	"kubesentinel-ai/internal/config"
	"kubesentinel-ai/internal/diagnosis"
	"kubesentinel-ai/internal/notifier"
	"kubesentinel-ai/internal/provider"
	"log"
)

func main() {
	fmt.Println("Starting KubeSentinel AI...")

	// 1. Load Configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	fmt.Printf("Configuration loaded successfully (LogLevel: %s, Port: %d)\n", cfg.App.LogLevel, cfg.App.Port)
	fmt.Printf("AI Provider: %s (Endpoint: %s)\n", cfg.AI.ProviderType, cfg.AI.Endpoint)

	// 2. Initialize Components

	// [A] AI Gateway 생성 (LLM 통신 엔진)
	aiGateway := provider.NewAIGateway(&cfg.AI)

	// [B] Diagnosis Engine 생성 (분석 로직 엔진)
	engine := diagnosis.NewEngine(aiGateway)

	// [C] Evidence Enricher 생성 (Prometheus/Loki 근거 보강)
	enricher := collector.NewEnricher(cfg.Collector)

	// [D] Notifier 생성 (알림 채널)
	notify, err := notifier.New(cfg.Notifier, cfg.Collector.GrafanaURL)
	if err != nil {
		log.Fatalf("Failed to initialize notifier: %v", err)
	}

	// [E] Webhook Server 생성 (컴포넌트 주입)
	server := collector.NewWebhookServer(fmt.Sprintf("%d", cfg.App.Port), engine, enricher, notify)

	// 3. Start Webhook Server in a Goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Webhook server failed to start: %v", err)
		}
	}()

	fmt.Println("KubeSentinel AI is now running. Press Ctrl+C to exit.")

	// 4. Keep the application running
	select {}
}
