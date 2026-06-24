package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"kubesentinel-ai/internal/models"
)

// SaveIncident는 인시던트 뷰를 upsert 합니다 (incident_id 기준).
func (s *Store) SaveIncident(v models.IncidentView) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode incident: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO incidents (incident_id, created_at, data) VALUES ($1, $2, $3)
		 ON CONFLICT (incident_id) DO UPDATE SET data = EXCLUDED.data, created_at = EXCLUDED.created_at`,
		v.IncidentID, v.CreatedAt, raw,
	)
	if err != nil {
		return fmt.Errorf("save incident: %w", err)
	}
	return nil
}

// ListIncidents는 최신순으로 인시던트 JSON을 반환합니다.
func (s *Store) ListIncidents(limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT data FROM incidents ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	out := []json.RawMessage{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		out = append(out, json.RawMessage(raw))
	}
	return out, rows.Err()
}

// GetIncident는 단일 인시던트 JSON을 반환합니다. 없으면 (nil, nil).
func (s *Store) GetIncident(id string) (json.RawMessage, error) {
	var raw []byte
	err := s.db.QueryRow(`SELECT data FROM incidents WHERE incident_id = $1`, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}
	return json.RawMessage(raw), nil
}
