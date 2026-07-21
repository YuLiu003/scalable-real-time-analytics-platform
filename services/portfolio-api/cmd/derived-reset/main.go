package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/YuLiu003/real-time-analytics-platform/services/portfolio-api/internal/store"
)

func main() {
	settings, err := store.FromEnvironment()
	if err != nil {
		fatal(err)
	}
	objectStore, err := store.New(context.Background(), settings)
	if err != nil {
		fatal(err)
	}
	deleted := make(map[string]int)
	for _, prefix := range []string{"silver/market_prices/", "gold/portfolio_allocations/"} {
		count, err := objectStore.DeletePrefix(context.Background(), prefix)
		if err != nil {
			fatal(err)
		}
		deleted[prefix] = count
	}
	encoded, _ := json.Marshal(map[string]any{"event": "derived analytics reset", "deleted": deleted})
	fmt.Println(string(encoded))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
