package main

import (
	"fmt"
	"log"
	"kubesentinel-ai/internal/collector"
	"kubesentinel-ai/internal/config"
	"kubesentinel-ai/internal/diagnosis"
	"kubesentinel-ai/internal/provider"
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

	// [C] Webhook Server 생성 (엔진을 주입하여 인수 불일치 해결)
	server := collector.NewWebhookServer(fmt.Sprintf("%d", cfg.App.Port), engine)

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
