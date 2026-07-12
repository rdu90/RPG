-- +goose Up
CREATE TABLE colonies (
    node_id TEXT PRIMARY KEY,
    focus TEXT NOT NULL,
    population INTEGER NOT NULL,
    last_tick_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE colonies;
