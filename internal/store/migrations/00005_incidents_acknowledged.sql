-- +goose Up
-- 확인됨(acknowledged) 인시던트는 기본 목록에서 숨긴다(운영자가 처리 완료 표시).
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS acknowledged boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE incidents DROP COLUMN IF EXISTS acknowledged;
