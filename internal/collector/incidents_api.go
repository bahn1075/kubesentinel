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
		switch r.Method {
		case http.MethodPost:
			s.handleIncidentReanalyze(w, r, id2) // 작업 시작 (202 즉시 반환)
		case http.MethodGet:
			s.handleIncidentReanalyzeStatus(w, r, id2) // 진행 상태 폴링
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
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

// reanalyzeStatusResponse는 재분석 작업의 진행 상태 응답입니다.
type reanalyzeStatusResponse struct {
	State      string `json:"state"` // idle | running | done | failed
	ElapsedSec int    `json:"elapsedSec"`
	Error      string `json:"error,omitempty"`
}

// handleIncidentReanalyze는 저장된 근거로 AI 진단을 다시 시도하는 작업을 시작한다.
// POST /api/incidents/{id}/reanalyze → 202 Accepted (즉시 반환)
//
// 로컬 LLM은 분석에 수 분이 걸려 동기 응답이 프록시 타임아웃(nginx 기본 60초)에 걸린다.
// 따라서 분석은 goroutine에서 수행하고, 진행 상태는 GET으로 폴링한다.
// 완료되면 인시던트가 DB에 갱신되므로 GET /api/incidents/{id}로 결과를 받는다.
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

	job, started := s.reanalyzeMgr().start(id)
	if !started {
		// 이미 실행 중 — 중복 실행하지 않고 현재 상태를 그대로 알려준다.
		cur, _ := s.reanalyzeMgr().get(id)
		writeJSON(w, http.StatusConflict, reanalyzeStatusResponse{
			State: cur.State, ElapsedSec: cur.ElapsedSec(),
			Error: "이미 재분석이 진행 중입니다.",
		})
		return
	}

	go s.runReanalyze(id, view)

	writeJSON(w, http.StatusAccepted, reanalyzeStatusResponse{
		State: job.State, ElapsedSec: 0,
	})
}

// handleIncidentReanalyzeStatus는 재분석 진행 상태를 반환한다.
// GET /api/incidents/{id}/reanalyze
//
// 작업 기록이 없으면 idle이다(한 번도 실행하지 않았거나 프로세스가 재시작된 경우).
func (s *WebhookServer) handleIncidentReanalyzeStatus(w http.ResponseWriter, r *http.Request, id string) {
	j, ok := s.reanalyzeMgr().get(id)
	if !ok {
		writeJSON(w, http.StatusOK, reanalyzeStatusResponse{State: "idle"})
		return
	}
	writeJSON(w, http.StatusOK, reanalyzeStatusResponse{
		State: j.State, ElapsedSec: j.ElapsedSec(), Error: j.Error,
	})
}
