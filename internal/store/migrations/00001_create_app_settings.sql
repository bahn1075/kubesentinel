-- +goose Up
-- 단일 행(id=1) 설정 테이블. 비민감 애플리케이션 설정을 JSONB로 보관한다.
CREATE TABLE IF NOT EXISTS app_settings (
    id         smallint     PRIMARY KEY DEFAULT 1,
    data       jsonb        NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT app_settings_single_row CHECK (id = 1)
);

INSERT INTO app_settings (id, data) VALUES (1, '{}'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS app_settings;
