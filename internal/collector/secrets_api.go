package collector

import (
	"encoding/json"
	"net/http"

	"kubesentinel-ai/internal/models"
)

// handleSecrets는 민감정보(키/토큰)를 write-only로 다룹니다.
//
//	GET  /api/secrets → 어떤 시크릿이 설정돼 있는지 여부만 ({aiApiKey:bool, gitToken:bool})
//	PUT  /api/secrets → 값 설정/변경/삭제. 값은 절대 반환하지 않는다.
//
// 본문(PUT): { "aiApiKey": "<값>" | "" , "gitToken": "<값>" | "" }
//   - 키 존재 + 값 있음 → 설정/변경
//   - 키 존재 + 빈 문자열 → 삭제
//   - 키 없음(null/미포함) → 변경 없음
func (s *WebhookServer) handleSecrets(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		http.Error(w, "store not configured (DATABASE_URL 미설정)", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		names, err := s.Store.SecretNames()
		if err != nil {
			http.Error(w, "failed to read secrets: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{
			"aiApiKey": names[models.SecretAIAPIKey],
			"gitToken": names[models.SecretGitToken],
		})

	case http.MethodPut:
		var in struct {
			AIAPIKey *string `json:"aiApiKey"`
			GitToken *string `json:"gitToken"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.applySecret(models.SecretAIAPIKey, in.AIAPIKey); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.applySecret(models.SecretGitToken, in.GitToken); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// 변경 후 현재 set 상태 반환(값 제외)
		names, _ := s.Store.SecretNames()
		writeJSON(w, http.StatusOK, map[string]bool{
			"aiApiKey": names[models.SecretAIAPIKey],
			"gitToken": names[models.SecretGitToken],
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// applySecret: nil=변경없음, ""=삭제, 그 외=설정.
func (s *WebhookServer) applySecret(name string, v *string) error {
	if v == nil {
		return nil
	}
	if *v == "" {
		return s.Store.DeleteSecret(name)
	}
	return s.Store.SetSecret(name, *v)
}
