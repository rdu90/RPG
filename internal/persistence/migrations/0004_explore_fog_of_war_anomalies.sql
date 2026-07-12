-- +goose Up
CREATE TABLE player_discovered (
    node_id TEXT PRIMARY KEY
);

CREATE TABLE player_claimed_anomalies (
    node_id TEXT PRIMARY KEY
);

-- +goose Down
DROP TABLE player_claimed_anomalies;
DROP TABLE player_discovered;
