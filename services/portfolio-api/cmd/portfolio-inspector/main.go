package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/YuLiu003/real-time-analytics-platform/services/portfolio-api/internal/result"
	"github.com/YuLiu003/real-time-analytics-platform/services/portfolio-api/internal/store"
)

func main() {
	expectedTotal := flag.String("expected-total", "", "expected total_market_value")
	expectedPositions := flag.Int("expected-positions", -1, "expected number of positions")
	expectedSHA := flag.String("expected-sha256", "", "expected canonical JSON SHA-256")
	expectedSilver := flag.Int("expected-silver-objects", -1, "expected silver object count")
	expectedGold := flag.Int("expected-gold-objects", -1, "expected gold object count")
	portfolio := flag.String("portfolio", "demo", "portfolio identifier")
	flag.Parse()

	settings, err := store.FromEnvironment()
	if err != nil {
		fatal(err)
	}
	objectStore, err := store.New(context.Background(), settings)
	if err != nil {
		fatal(err)
	}
	data, err := objectStore.Latest(context.Background(), *portfolio)
	if err != nil {
		fatal(err)
	}
	snapshot, err := result.DecodeStrict(data)
	if err != nil {
		fatal(err)
	}
	digest := sha256.Sum256(data)
	encodedSHA := hex.EncodeToString(digest[:])
	if *expectedTotal != "" && snapshot.TotalMarketValue != *expectedTotal {
		fatal(fmt.Errorf("total_market_value = %s, want %s", snapshot.TotalMarketValue, *expectedTotal))
	}
	if *expectedPositions >= 0 && len(snapshot.Positions) != *expectedPositions {
		fatal(fmt.Errorf("positions = %d, want %d", len(snapshot.Positions), *expectedPositions))
	}
	if *expectedSHA != "" && encodedSHA != *expectedSHA {
		fatal(fmt.Errorf("result SHA-256 = %s, want %s", encodedSHA, *expectedSHA))
	}
	silverKeys, err := objectStore.ListKeys(context.Background(), "silver/market_prices/")
	if err != nil {
		fatal(err)
	}
	goldKeys, err := objectStore.ListKeys(context.Background(), "gold/portfolio_allocations/")
	if err != nil {
		fatal(err)
	}
	if *expectedSilver >= 0 && len(silverKeys) != *expectedSilver {
		fatal(fmt.Errorf("silver objects = %d, want %d", len(silverKeys), *expectedSilver))
	}
	if *expectedGold >= 0 && len(goldKeys) != *expectedGold {
		fatal(fmt.Errorf("gold objects = %d, want %d", len(goldKeys), *expectedGold))
	}
	fmt.Printf("portfolio=%s total=%s positions=%d input_objects=%d silver_objects=%d gold_objects=%d input_set_sha256=%s result_sha256=%s\n", snapshot.PortfolioID, snapshot.TotalMarketValue, len(snapshot.Positions), snapshot.InputObjectCount, len(silverKeys), len(goldKeys), snapshot.InputSetSHA256, encodedSHA)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
