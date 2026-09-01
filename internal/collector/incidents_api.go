package collector

import (
	"encoding/json"
	"net/http"
	"strings"

	"kubesentinel-ai/internal/models"
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
	id := strings.TrimPrefix(r.URL.Path, "/api/incidents/")
	if id == "" {
		http.Error(w, "incident id required", http.StatusBadRequest)
		return
	}

	// POST .../reanalyze: AI 진단이 없는(LLM 다운 등) 인시던트에 대해, 이미 수집된 근거로
	// 진단을 다시 시도한다(근거 재수집 없음 — LLM 연결이 그때 끊겨 있었을 뿐 근거는 유효하다).
	if id2, ok := strings.CutSuffix(id, "/reanalyze"); ok {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleIncidentReanalyze(w, r, id2)
		return
	}

	// PATCH: 확인됨(acknowledged) 토글 — 확인됨이면 목록에서 숨겨진다.
	if r.Method == http.MethodPatch {
		var in struct {
			Acknowledged bool `json:"acknowledged"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if err := s.Store.AcknowledgeIncident(id, in.Acknowledged); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

// handleIncidentReanalyze는 저장된 근거로 AI 진단을 다시 시도한다. POST /api/incidents/{id}/reanalyze
// LLM이 다시 실패하면 인시던트는 건드리지 않고 에러만 반환한다(성공한 것처럼 보이면 안 됨).
func (s *WebhookServer) handleIncidentReanalyze(w http.ResponseWriter, r *http.Request, id string) {
	raw, err := s.Store.GetIncident(id)
	if err != nil {
		http.Error(w, "failed to get incident: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if raw == nil {
		http.Error(w, "incident not found", http.StatusNotFound)
		return
	}
	var view models.IncidentView
	if err := json.Unmarshal(raw, &view); err != nil {
		http.Error(w, "failed to decode incident: "+err.Error(), http.StatusInternalServerError)
		return
	}

	bundle := models.EvidenceBundleFromView(view)
	result, err := s.Engine.Analyze(bundle)
	if err != nil {
		http.Error(w, "재분석 실패(AI 연결을 확인하세요): "+err.Error(), http.StatusBadGateway)
		return
	}

	updated := models.NewIncidentView(bundle, result, "DiagnosisCompleted")
	updated.CreatedAt = view.CreatedAt // 최초 발생 시각 보존 — 목록 정렬(created_at)이 바뀌지 않게
	updated.PRURL = view.PRURL
	if err := s.Store.SaveIncident(updated); err != nil {
		http.Error(w, "failed to save incident: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
