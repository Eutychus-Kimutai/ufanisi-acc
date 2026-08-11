-- +goose Up
CREATE TABLE IF NOT EXISTS clients (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    client_type TEXT NOT NULL CHECK (client_type IN ('loan', 'investment')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS clients;