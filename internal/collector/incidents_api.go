package collector

import (
	"net/http"
	"strings"
)

// handleIncidents는 인시던트 목록을 반환합니다. GET /api/incidents
func (s *WebhookServer) handleIncidents(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		http.Error(w, "store not configured (DATABASE_URL 미설정)", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	list, err := s.Store.ListIncidents(100)
	if err != nil {
		http.Error(w, "failed to list incidents: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// handleIncidentDetail은 단일 인시던트를 반환합니다. GET /api/incidents/{id}
func (s *WebhookServer) handleIncidentDetail(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		http.Error(w, "store not configured (DATABASE_URL 미설정)", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/incidents/")
	if id == "" {
		http.Error(w, "incident id required", http.StatusBadRequest)
		return
	}
	raw, err := s.Store.GetIncident(id)
	if err != nil {
		http.Error(w, "failed to get incident: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if raw == nil {
		http.Error(w, "incident not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}
