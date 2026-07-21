package result

import "testing"

const testSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func validSnapshot() Snapshot {
	return Snapshot{
		SchemaVersion:       2,
		PortfolioID:         "demo",
		DisplayName:         "Synthetic Fund Portfolio",
		BaseCurrency:        "USD",
		AsOf:                "2026-07-21T00:01:30Z",
		InputObjectCount:    4,
		InputSetSHA256:      testSHA,
		SilverParquetObject: "silver/market_prices/v1/run=" + testSHA + "/part-00000.parquet",
		GoldParquetObject:   "gold/portfolio_allocations/v2/portfolio=demo/run=" + testSHA + "/allocation.parquet",
		TotalMarketValue:    "10.00000000",
		Positions: []Position{{
			Instrument:    "QQQ",
			DisplayName:   "Invesco QQQ",
			AssetType:     "etf",
			ValuationType: "market_price",
			Quantity:      "10.00000000",
			Price:         "1.00000000",
			MarketValue:   "10.00000000",
			AllocationPct: "100.0000",
			PriceAsOf:     "2026-07-21T00:00:00Z",
		}},
		Benchmark: Benchmark{
			Instrument:    "SP500",
			DisplayName:   "S&P 500 Index",
			AssetType:     "index",
			ValuationType: "index_level",
			Price:         "6500.00000000",
			PriceAsOf:     "2026-07-21T00:01:30Z",
		},
	}
}

func TestValidateAcceptsMatchingDataProductKeys(t *testing.T) {
	if err := validSnapshot().Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsMismatchedDataProductKey(t *testing.T) {
	snapshot := validSnapshot()
	snapshot.GoldParquetObject = "gold/portfolio_allocations/v2/portfolio=other/run=" + testSHA + "/allocation.parquet"
	if err := snapshot.Validate(); err == nil {
		t.Fatal("Validate() accepted a data product key for another portfolio")
	}
}
