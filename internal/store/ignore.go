package store

import (
	"fmt"
	"strings"

	"kubesentinel-ai/internal/models"
)

// ListIgnoreRules는 모든 무시 규칙을 최신순으로 반환합니다.
func (s *Store) ListIgnoreRules() ([]models.IgnoreRule, error) {
	rows, err := s.db.Query(`SELECT id, keyword, enabled, created_at FROM ignore_rules ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list ignore rules: %w", err)
	}
	defer rows.Close()
	out := []models.IgnoreRule{}
	for rows.Next() {
		var r models.IgnoreRule
		if err := rows.Scan(&r.ID, &r.Keyword, &r.Enabled, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ignore rule: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AddIgnoreRule은 규칙을 추가(같은 keyword면 enabled=true로 갱신)하고 저장된 규칙을 반환합니다.
func (s *Store) AddIgnoreRule(keyword string) (models.IgnoreRule, error) {
	keyword = strings.TrimSpace(keyword)
	var r models.IgnoreRule
	if keyword == "" {
		return r, fmt.Errorf("keyword is empty")
	}
	err := s.db.QueryRow(
		`INSERT INTO ignore_rules (keyword, enabled) VALUES ($1, true)
		 ON CONFLICT (keyword) DO UPDATE SET enabled = true
		 RETURNING id, keyword, enabled, created_at`, keyword,
	).Scan(&r.ID, &r.Keyword, &r.Enabled, &r.CreatedAt)
	if err != nil {
		return r, fmt.Errorf("add ignore rule: %w", err)
	}
	return r, nil
}

// SetIgnoreRuleEnabled는 규칙의 활성 여부를 토글합니다.
func (s *Store) SetIgnoreRuleEnabled(id int64, enabled bool) error {
	_, err := s.db.Exec(`UPDATE ignore_rules SET enabled = $2 WHERE id = $1`, id, enabled)
	if err != nil {
		return fmt.Errorf("set ignore rule enabled: %w", err)
	}
	return nil
}

// DeleteIgnoreRule은 규칙을 삭제합니다.
func (s *Store) DeleteIgnoreRule(id int64) error {
	_, err := s.db.Exec(`DELETE FROM ignore_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete ignore rule: %w", err)
	}
	return nil
}
