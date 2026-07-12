// Package economy models tradeable commodities and per-system pricing. A
// commodity's fixed Category (Normal/Illegal/Exotic/Immoral) is the
// mechanical basis for the player's derived alignment in later milestones.
package economy

import "math/rand/v2"

// CommodityID identifies a commodity definition.
type CommodityID string

// Category is a commodity's fixed legal/moral classification.
type Category string

const (
	CategoryNormal  Category = "normal"
	CategoryIllegal Category = "illegal"
	CategoryExotic  Category = "exotic"
	CategoryImmoral Category = "immoral"
)

// Commodity is a tradeable good definition.
type Commodity struct {
	ID        CommodityID
	Name      string
	Category  Category
	BasePrice int // credits per unit at a mid-development (level 3) system
}

// Commodities is the fixed, data-driven catalog of tradeable goods.
var Commodities = []Commodity{
	{ID: "food", Name: "Food Rations", Category: CategoryNormal, BasePrice: 12},
	{ID: "machinery", Name: "Machinery", Category: CategoryNormal, BasePrice: 60},
	{ID: "medicine", Name: "Medicine", Category: CategoryNormal, BasePrice: 45},
	{ID: "narcotics", Name: "Narcotics", Category: CategoryIllegal, BasePrice: 150},
	{ID: "weapons", Name: "Weapons", Category: CategoryIllegal, BasePrice: 120},
	{ID: "artifacts", Name: "Alien Artifacts", Category: CategoryExotic, BasePrice: 300},
	{ID: "labor", Name: "Indentured Labor", Category: CategoryImmoral, BasePrice: 200},
}

// Find looks up a commodity definition by ID.
func Find(id CommodityID) (Commodity, bool) {
	for _, c := range Commodities {
		if c.ID == id {
			return c, true
		}
	}
	return Commodity{}, false
}

// Price is a commodity's credit-per-unit price at a specific system.
type Price struct {
	CommodityID CommodityID
	Price       int
}

// GenerateMarket produces a price for every commodity at a system of the
// given development level (1..5), jittered by r. Normal and exotic goods
// get cheaper as development rises (more local supply); illegal and
// immoral goods get pricier (higher risk premium from enforcement).
func GenerateMarket(r *rand.Rand, developmentLevel int) []Price {
	prices := make([]Price, 0, len(Commodities))
	dev := float64(developmentLevel)

	for _, c := range Commodities {
		var factor float64
		switch c.Category {
		case CategoryIllegal, CategoryImmoral:
			factor = 0.7 + 0.15*dev
		default:
			factor = 1.3 - 0.1*dev
		}

		jitter := 0.9 + r.Float64()*0.2 // +/-10%
		price := int(float64(c.BasePrice) * factor * jitter)
		if price < 1 {
			price = 1
		}

		prices = append(prices, Price{CommodityID: c.ID, Price: price})
	}
	return prices
}
