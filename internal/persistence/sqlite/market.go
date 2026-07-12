package sqlite

import (
	"context"
	"fmt"

	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/galaxy"
)

// SaveMarket implements ports.MarketRepository.
func (s *Store) SaveMarket(ctx context.Context, nodeID galaxy.NodeID, prices []economy.Price) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: save market for %s: %w", nodeID, err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, p := range prices {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO market_prices (node_id, commodity_id, price) VALUES (?, ?, ?)`,
			nodeID, p.CommodityID, p.Price,
		); err != nil {
			return fmt.Errorf("sqlite: save market price %s at %s: %w", p.CommodityID, nodeID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: save market for %s: %w", nodeID, err)
	}
	return nil
}

// GetMarket implements ports.MarketRepository. Results are returned in the
// fixed economy.Commodities catalog order (SQLite gives no ordering
// guarantee on its own), so the trade screen lists commodities in a stable
// order run to run.
func (s *Store) GetMarket(ctx context.Context, nodeID galaxy.NodeID) ([]economy.Price, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT commodity_id, price FROM market_prices WHERE node_id = ?`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get market for %s: %w", nodeID, err)
	}
	defer func() { _ = rows.Close() }()

	byID := make(map[economy.CommodityID]int)
	for rows.Next() {
		var id economy.CommodityID
		var price int
		if err := rows.Scan(&id, &price); err != nil {
			return nil, fmt.Errorf("sqlite: scan market price: %w", err)
		}
		byID[id] = price
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: get market for %s: %w", nodeID, err)
	}

	var prices []economy.Price
	for _, c := range economy.Commodities {
		if price, ok := byID[c.ID]; ok {
			prices = append(prices, economy.Price{CommodityID: c.ID, Price: price})
		}
	}
	return prices, nil
}
