package result

import (
	"encoding/json"
	"strings"
	"testing"
)

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

func TestDecodeStrictAcceptsCanonicalSnapshot(t *testing.T) {
	decoded, err := DecodeStrict(validResultBytes(t))
	if err != nil {
		t.Fatalf("DecodeStrict() error = %v", err)
	}
	if decoded.PortfolioID != "demo" {
		t.Fatalf("portfolio = %q", decoded.PortfolioID)
	}
}

func TestDecodeStrictRejectsInvalidAndTrailingJSON(t *testing.T) {
	for _, data := range [][]byte{[]byte(`{"schema_version":`), append(validResultBytes(t), []byte(` {}`)...), []byte(`{}`)} {
		if _, err := DecodeStrict(data); err == nil {
			t.Fatal("DecodeStrict() unexpectedly succeeded")
		}
	}
}

func TestValidateRejectsEveryInvalidResultClass(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{name: "identity", mutate: func(s *Snapshot) { s.SchemaVersion = 1 }},
		{name: "as of", mutate: func(s *Snapshot) { s.AsOf = "invalid" }},
		{name: "input metadata", mutate: func(s *Snapshot) { s.InputObjectCount = 0 }},
		{name: "no positions", mutate: func(s *Snapshot) { s.Positions = nil }},
		{name: "position fields", mutate: func(s *Snapshot) { s.Positions[0].Quantity = "1" }},
		{name: "position timestamp", mutate: func(s *Snapshot) { s.Positions[0].PriceAsOf = "invalid" }},
		{name: "duplicate position", mutate: func(s *Snapshot) { s.Positions = append(s.Positions, s.Positions[0]) }},
		{name: "benchmark fields", mutate: func(s *Snapshot) { s.Benchmark.AssetType = "etf" }},
		{name: "benchmark position collision", mutate: func(s *Snapshot) { s.Benchmark.Instrument = "QQQ" }},
		{name: "benchmark timestamp", mutate: func(s *Snapshot) { s.Benchmark.PriceAsOf = "invalid" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := validSnapshot()
			tt.mutate(&snapshot)
			if err := snapshot.Validate(); err == nil {
				t.Fatal("Validate() unexpectedly succeeded")
			}
		})
	}
}

func TestDisplayNameAndPositionSemanticsValidation(t *testing.T) {
	for _, name := range []string{"", " padded", strings.Repeat("x", 101), "control\nname"} {
		if validDisplayName(name) {
			t.Fatalf("validDisplayName(%q) = true", name)
		}
	}
	if !validDisplayName("S&P 500 Index") {
		t.Fatal("validDisplayName() rejected a valid name")
	}
	if !validPositionSemantics("etf", "market_price") || !validPositionSemantics("mutual_fund", "nav") {
		t.Fatal("validPositionSemantics() rejected a supported position")
	}
	if validPositionSemantics("index", "index_level") {
		t.Fatal("validPositionSemantics() accepted an index position")
	}
}

func validResultBytes(t *testing.T) []byte {
	t.Helper()
	data, err := json.Marshal(validSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	return data
}
