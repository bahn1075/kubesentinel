-- +goose Up
-- 사용자 관리 무시 규칙. keyword가 alert명 또는 대상(네임스페이스/워크로드/파드/라벨)에
-- 부분일치(양쪽 와일드카드)하면 해당 alert를 인시던트로 처리하지 않는다.
CREATE TABLE IF NOT EXISTS ignore_rules (
    id         bigserial   PRIMARY KEY,
    keyword    text        NOT NULL UNIQUE,
    enabled    boolean     NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS ignore_rules;
