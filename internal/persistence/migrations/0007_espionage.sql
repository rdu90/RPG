-- +goose Up
CREATE TABLE spies (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    skill        INTEGER NOT NULL,
    status       INTEGER NOT NULL,
    missions_run INTEGER NOT NULL DEFAULT 0
);

-- +goose Down
DROP TABLE spies;
