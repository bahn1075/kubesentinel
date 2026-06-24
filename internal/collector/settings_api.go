package collector

import (
	"encoding/json"
	"net/http"

	"kubesentinel-ai/internal/models"
)

// handleSettings는 비민감 애플리케이션 설정의 조회/저장 API입니다.
//
//	GET  /api/settings  → 현재 설정 (DB, 없으면 기본값)
//	PUT  /api/settings  → 설정 저장(upsert)
//
// 민감정보(API key/webhook/token)는 이 API로 다루지 않는다 (k8s Secret/env 관리).
func (s *WebhookServer) handleSettings(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		http.Error(w, "settings store not configured (DATABASE_URL 미설정)", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		settings, err := s.Store.GetSettings()
		if err != nil {
			http.Error(w, "failed to load settings: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, settings)

	case http.MethodPut:
		var in models.AppSettings
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid settings payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.Store.SaveSettings(in); err != nil {
			http.Error(w, "failed to save settings: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, in)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
