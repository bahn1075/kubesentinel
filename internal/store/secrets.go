package store

import (
	"database/sql"
	"fmt"
)

// SetSecret은 이름별 시크릿을 upsert 합니다. (write-only — 값은 API로 반환하지 않음)
func (s *Store) SetSecret(name, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO app_secrets (name, value, updated_at) VALUES ($1, $2, now())
		 ON CONFLICT (name) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		name, value,
	)
	if err != nil {
		return fmt.Errorf("set secret %s: %w", name, err)
	}
	return nil
}

// GetSecret은 시크릿 값을 반환합니다. 없으면 ("", false, nil).
func (s *Store) GetSecret(name string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM app_secrets WHERE name = $1`, name).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get secret %s: %w", name, err)
	}
	return v, true, nil
}

// DeleteSecret은 시크릿을 제거합니다.
func (s *Store) DeleteSecret(name string) error {
	_, err := s.db.Exec(`DELETE FROM app_secrets WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("delete secret %s: %w", name, err)
	}
	return nil
}

// SecretNames는 설정된 시크릿 이름 집합을 반환합니다(값은 제외).
func (s *Store) SecretNames() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT name FROM app_secrets`)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out[n] = true
	}
	return out, rows.Err()
}
