-- +goose Up
-- 민감정보(키/토큰) 저장. write-only로 다루며 API는 값을 반환하지 않는다.
-- (평문 저장 — 추후 애플리케이션 레벨 암호화 또는 KMS 연동 과제)
CREATE TABLE IF NOT EXISTS app_secrets (
    name       text        PRIMARY KEY,
    value      text        NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS app_secrets;
