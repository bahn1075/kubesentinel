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

	// 2. 근거 보강 → AI 분석 → 알림 (비동기 실행)
	go func(b *models.EvidenceBundle) {
		fmt.Printf("\n[KubeSentinel] 🔍 Analyzing Incident: %s\n", b.IncidentID)

		// 2-1. Prometheus/Loki로 EvidenceBundle 보강 (best-effort)
		if s.Enricher != nil {
			s.Enricher.Enrich(b)
		}

		// 2-2. AI RCA 분석
		result, err := s.Engine.Analyze(b)
		state := "DiagnosisCompleted"
		if err != nil {
			fmt.Printf("[KubeSentinel] ❌ Analysis Failed: %v\n", err)
			state = "ValidationFailed"
			result = nil
		} else {
			fmt.Printf("[KubeSentinel] ✅ Analysis Complete!\n")
			fmt.Printf("  - Root Cause: %s\n", result.RootCause)
			fmt.Printf("  - Summary: %s\n", result.Summary)
			if len(result.ProposedActions) > 0 {
				fmt.Printf("  - Proposed Actions: %d\n", len(result.ProposedActions))
			}
		}

		// 2-3. 인시던트 영속화 (DB, 대시보드 조회용)
		if s.Store != nil {
			view := models.NewIncidentView(b, result, state)
			if e := s.Store.SaveIncident(view); e != nil {
				fmt.Printf("[KubeSentinel] ⚠️  Save Incident Failed: %v\n", e)
			}
		}

		// 2-4. 알림 채널 전송 (분석 성공 시) — MVP-0: 읽기 전용 RCA + 알림
		if result != nil && s.Notifier != nil {
			if err := s.Notifier.NotifyDiagnosis(b, result); err != nil {
				fmt.Printf("[KubeSentinel] ⚠️  Notification Failed: %v\n", err)
			}
		}
	}(bundle)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
