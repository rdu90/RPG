-- +goose Up
ALTER TABLE colonies ADD COLUMN owner TEXT NOT NULL DEFAULT 'player';
ALTER TABLE colonies ADD COLUMN garrison_attack INTEGER NOT NULL DEFAULT 0;
ALTER TABLE colonies ADD COLUMN garrison_defense INTEGER NOT NULL DEFAULT 0;
ALTER TABLE colonies ADD COLUMN garrison_hull INTEGER NOT NULL DEFAULT 0;
ALTER TABLE colonies ADD COLUMN garrison_max_hull INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE colonies DROP COLUMN owner;
ALTER TABLE colonies DROP COLUMN garrison_attack;
ALTER TABLE colonies DROP COLUMN garrison_defense;
ALTER TABLE colonies DROP COLUMN garrison_hull;
ALTER TABLE colonies DROP COLUMN garrison_max_hull;
