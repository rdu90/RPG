-- +goose Up
CREATE TABLE galaxy_nodes (
    id                TEXT PRIMARY KEY,
    name              TEXT NOT NULL,
    x                 INTEGER NOT NULL,
    y                 INTEGER NOT NULL,
    development_level INTEGER NOT NULL
);

CREATE TABLE galaxy_edges (
    from_id   TEXT NOT NULL,
    to_id     TEXT NOT NULL,
    turn_cost INTEGER NOT NULL,
    PRIMARY KEY (from_id, to_id)
);

CREATE TABLE market_prices (
    node_id      TEXT NOT NULL,
    commodity_id TEXT NOT NULL,
    price        INTEGER NOT NULL,
    PRIMARY KEY (node_id, commodity_id)
);

CREATE TABLE player (
    id                    INTEGER PRIMARY KEY CHECK (id = 1),
    credits               INTEGER NOT NULL,
    node_id               TEXT NOT NULL,
    cargo_capacity        INTEGER NOT NULL,
    turns_max             INTEGER NOT NULL,
    turns_remaining       INTEGER NOT NULL,
    turns_refill_every_ms INTEGER NOT NULL,
    turns_last_refill_at  TIMESTAMP NOT NULL
);

CREATE TABLE player_cargo (
    commodity_id TEXT PRIMARY KEY,
    quantity     INTEGER NOT NULL
);

-- +goose Down
DROP TABLE player_cargo;
DROP TABLE player;
DROP TABLE market_prices;
DROP TABLE galaxy_edges;
DROP TABLE galaxy_nodes;
