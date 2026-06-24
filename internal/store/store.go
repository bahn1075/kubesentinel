// Package store는 Postgres 기반 설정 영속화를 담당합니다.
// 스키마 변경은 goose 임베드 마이그레이션(migrations/*.sql)으로 관리한다.
package store

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql용 pgx 드라이버 등록
	"github.com/pressly/goose/v3"

	"kubesentinel-ai/internal/models"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// Store는 설정 저장소입니다.
type Store struct {
	db *sql.DB
}

// New는 Postgres에 연결하고 핑을 확인합니다.
func New(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	// Postgres가 늦게 기동하는 경우(예: k8s 기동 순서)를 대비해 핑을 재시도한다.
	var pingErr error
	for i := 0; i < 30; i++ {
		if pingErr = db.Ping(); pingErr == nil {
			return &Store{db: db}, nil
		}
		fmt.Printf("waiting for database... (%d/30): %v\n", i+1, pingErr)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("ping db after retries: %w", pingErr)
}

// Migrate는 임베드된 goose 마이그레이션을 적용합니다 (기동 시 자동 실행).
func (s *Store) Migrate() error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.Up(s.db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

// GetSettings는 저장된 설정을 반환합니다. 비어 있으면 기본값을 채워 반환합니다.
func (s *Store) GetSettings() (models.AppSettings, error) {
	var raw []byte
	err := s.db.QueryRow(`SELECT data FROM app_settings WHERE id = 1`).Scan(&raw)
	if err == sql.ErrNoRows || len(raw) == 0 || string(raw) == "{}" {
		return models.DefaultAppSettings(), nil
	}
	if err != nil {
		return models.AppSettings{}, fmt.Errorf("query settings: %w", err)
	}
	// 기본값 위에 저장값을 덧씌워 누락 필드를 보강한다.
	out := models.DefaultAppSettings()
	if err := json.Unmarshal(raw, &out); err != nil {
		return models.AppSettings{}, fmt.Errorf("decode settings: %w", err)
	}
	return out, nil
}

// SaveSettings는 설정을 단일 행에 upsert 합니다.
func (s *Store) SaveSettings(set models.AppSettings) error {
	raw, err := json.Marshal(set)
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO app_settings (id, data, updated_at) VALUES (1, $1, now())
		 ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data, updated_at = now()`,
		raw,
	)
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}
