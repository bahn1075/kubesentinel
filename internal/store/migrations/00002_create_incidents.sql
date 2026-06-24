-- +goose Up
CREATE TABLE IF NOT EXISTS incidents (
    incident_id text        PRIMARY KEY,
    created_at  timestamptz NOT NULL DEFAULT now(),
    data        jsonb       NOT NULL
);
CREATE INDEX IF NOT EXISTS incidents_created_at_idx ON incidents (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS incidents;
