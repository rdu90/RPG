package economy

import (
	"testing"

	"github.com/rdu90/RPG/internal/rng"
)

func TestGenerateMarketCoversAllCommodities(t *testing.T) {
	prices := GenerateMarket(rng.New(1), 3)
	if len(prices) != len(Commodities) {
		t.Fatalf("expected %d prices, got %d", len(Commodities), len(prices))
	}
	for _, p := range prices {
		if p.Price < 1 {
			t.Errorf("commodity %s has non-positive price %d", p.CommodityID, p.Price)
		}
		if _, ok := Find(p.CommodityID); !ok {
			t.Errorf("price references unknown commodity %s", p.CommodityID)
		}
	}
}

func TestGenerateMarketDeterministic(t *testing.T) {
	a := GenerateMarket(rng.New(42), 4)
	b := GenerateMarket(rng.New(42), 4)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed produced different prices: %+v vs %+v", a[i], b[i])
		}
	}
}

func TestGenerateMarketDevelopmentTrend(t *testing.T) {
	// Average across many seeds to smooth out jitter and isolate the trend.
	const trials = 200
	var lowNormal, highNormal, lowIllegal, highIllegal int

	for seed := int64(0); seed < trials; seed++ {
		low := GenerateMarket(rng.New(seed), 1)
		high := GenerateMarket(rng.New(seed+1_000_000), 5)
		for i, c := range Commodities {
			switch c.Category {
			case CategoryIllegal, CategoryImmoral:
				lowIllegal += low[i].Price
				highIllegal += high[i].Price
			default:
				lowNormal += low[i].Price
				highNormal += high[i].Price
			}
		}
	}

	if highNormal >= lowNormal {
		t.Errorf("expected normal/exotic goods to get cheaper at high development: low=%d high=%d", lowNormal, highNormal)
	}
	if highIllegal <= lowIllegal {
		t.Errorf("expected illegal/immoral goods to get pricier at high development: low=%d high=%d", lowIllegal, highIllegal)
	}
}

func TestFind(t *testing.T) {
	if _, ok := Find("food"); !ok {
		t.Fatal("expected to find commodity \"food\"")
	}
	if _, ok := Find("nonexistent"); ok {
		t.Fatal("expected not to find unknown commodity")
	}
}
