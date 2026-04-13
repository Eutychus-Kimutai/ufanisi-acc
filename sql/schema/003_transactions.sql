-- +goose Up
CREATE TABLE transactions (
    id UUID PRIMARY KEY,
    reference TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE transactions;