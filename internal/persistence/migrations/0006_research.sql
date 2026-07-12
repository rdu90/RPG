-- +goose Up
CREATE TABLE research (
    id                    INTEGER PRIMARY KEY CHECK (id = 1),
    active_tech           TEXT NOT NULL DEFAULT '',
    progress              INTEGER NOT NULL DEFAULT 0,
    last_tick_at          TIMESTAMP NOT NULL,
    rate_bonus            INTEGER NOT NULL DEFAULT 0,
    trade_greed_reduction INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE research_unlocked (
    tech_id TEXT PRIMARY KEY
);

-- +goose Down
DROP TABLE research_unlocked;
DROP TABLE research;

