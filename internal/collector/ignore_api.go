package collector

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"kubesentinel-ai/internal/models"
)

// ignoreListResponse는 무시 목록 조회 응답이다.
// rules: 사용자 관리(DB, 편집 가능), config: 설정(values/env)로 고정된 alertname(읽기 전용).
type ignoreListResponse struct {
	Rules  []models.IgnoreRule `json:"rules"`
	Config []string            `json:"config"`
}

// handleIgnores는 무시 규칙 목록 조회/추가. GET/POST /api/ignores
func (s *WebhookServer) handleIgnores(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		http.Error(w, "store not configured (DATABASE_URL 미설정)", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rules, err := s.Store.ListIgnoreRules()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, ignoreListResponse{Rules: rules, Config: s.configIgnores()})
	case http.MethodPost:
		var in struct {
			Keyword string `json:"keyword"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(in.Keyword) == "" {
			http.Error(w, "keyword is required", http.StatusBadRequest)
			return
		}
		rule, err := s.Store.AddIgnoreRule(in.Keyword)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.RefreshIgnoreRules()
		writeJSON(w, http.StatusOK, rule)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleIgnoreDetail은 단일 규칙 토글/삭제. PATCH/DELETE /api/ignores/{id}
func (s *WebhookServer) handleIgnoreDetail(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		http.Error(w, "store not configured (DATABASE_URL 미설정)", http.StatusServiceUnavailable)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/ignores/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var in struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if err := s.Store.SetIgnoreRuleEnabled(id, in.Enabled); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.RefreshIgnoreRules()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		if err := s.Store.DeleteIgnoreRule(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.RefreshIgnoreRules()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// configIgnores는 설정(values/env) baseline으로 고정된 무시 alertname 목록을 반환한다.
func (s *WebhookServer) configIgnores() []string {
	out := make([]string, 0, len(s.IgnoreAlerts))
	for k := range s.IgnoreAlerts {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
