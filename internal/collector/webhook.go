package collector

import (
	"encoding/json"
	"fmt"
	"kubesentinel-ai/internal/config"
	"kubesentinel-ai/internal/diagnosis"
	"kubesentinel-ai/internal/models"
	"kubesentinel-ai/internal/notifier"
	"kubesentinel-ai/internal/store"
	"net/http"
)

// WebhookServer는 HTTP 요청을 수신하는 서버입니다.
type WebhookServer struct {
	Port     string
	Engine   *diagnosis.Engine
	Enricher *Enricher
	Notifier notifier.Notifier
	Store    *store.Store    // 설정 영속화 (nil이면 /api/settings 비활성)
	AI       config.AIConfig // 현재 활성(병합된) AI 설정 — 상태/health 표시용

	// Alertmanager 폴링 (pull) — URL 설정 시에만 활성
	AlertmanagerURL string
	PollIntervalSec int
}

// NewWebhookServer는 새로운 WebhookServer 인스턴스를 생성합니다.
func NewWebhookServer(port string, engine *diagnosis.Engine, enricher *Enricher, n notifier.Notifier, st *store.Store, ai config.AIConfig) *WebhookServer {
	return &WebhookServer{
		Port:     port,
		Engine:   engine,
		Enricher: enricher,
		Notifier: n,
		Store:    st,
		AI:       ai,
	}
}

// Start는 HTTP 서버를 시작합니다.
func (s *WebhookServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/alerts", s.handleAlertmanagerWebhook)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/secrets", s.handleSecrets)
	mux.HandleFunc("/api/incidents", s.handleIncidents)
	mux.HandleFunc("/api/incidents/", s.handleIncidentDetail)
	mux.HandleFunc("/api/ai/status", s.handleAIStatus)
	mux.HandleFunc("/api/ai/health", s.handleAIHealth)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	fmt.Printf("Starting Webhook Server on port %s...\n", s.Port)
	return http.ListenAndServe(":"+s.Port, mux)
}

// handleAlertmanagerWebhook은 Alertmanager의 Webhook 요청을 처리합니다.
func (s *WebhookServer) handleAlertmanagerWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload models.AlertmanagerPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode payload: %v", err), http.StatusBadRequest)
		return
	}

	// 1. Evidence Bundle 생성 (models 패키지의 함수 사용)
	bundle := models.NewEvidenceBundle(payload)
	if bundle == nil {
		http.Error(w, "Failed to create evidence bundle", http.StatusBadRequest)
		return
	}

	// 2. 근거 보강 → AI 분석 → 영속화 → 알림 (비동기 실행). webhook·폴러 공용 경로.
	go s.processBundle(bundle)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
