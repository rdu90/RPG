-- +goose Up
ALTER TABLE player ADD COLUMN alignment_legality REAL NOT NULL DEFAULT 0;
ALTER TABLE player ADD COLUMN alignment_morality REAL NOT NULL DEFAULT 0;

CREATE TABLE player_reputation (
    node_id    TEXT PRIMARY KEY,
    reputation INTEGER NOT NULL
);

-- +goose Down
DROP TABLE player_reputation;
ALTER TABLE player DROP COLUMN alignment_morality;
ALTER TABLE player DROP COLUMN alignment_legality;
