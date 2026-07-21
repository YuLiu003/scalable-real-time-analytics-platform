package result

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"
)

var (
	hexSHA256        = regexp.MustCompile(`^[0-9a-f]{64}$`)
	instrument       = regexp.MustCompile(`^[A-Z0-9][A-Z0-9.-]{0,14}$`)
	currency         = regexp.MustCompile(`^[A-Z]{3}$`)
	decimal8         = regexp.MustCompile(`^(0|[1-9][0-9]*)\.[0-9]{8}$`)
	decimal4         = regexp.MustCompile(`^(0|[1-9][0-9]*)\.[0-9]{4}$`)
	portfolioID      = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	errInvalidResult = errors.New("invalid portfolio allocation result")
)

type Snapshot struct {
	SchemaVersion       int        `json:"schema_version"`
	PortfolioID         string     `json:"portfolio_id"`
	BaseCurrency        string     `json:"base_currency"`
	AsOf                string     `json:"as_of"`
	InputObjectCount    int        `json:"input_object_count"`
	InputSetSHA256      string     `json:"input_set_sha256"`
	SilverParquetObject string     `json:"silver_parquet_object"`
	GoldParquetObject   string     `json:"gold_parquet_object"`
	TotalMarketValue    string     `json:"total_market_value"`
	Positions           []Position `json:"positions"`
}

type Position struct {
	Instrument    string `json:"instrument"`
	Quantity      string `json:"quantity"`
	Price         string `json:"price"`
	MarketValue   string `json:"market_value"`
	AllocationPct string `json:"allocation_pct"`
	PriceAsOf     string `json:"price_as_of"`
}

func DecodeStrict(data []byte) (Snapshot, error) {
	var snapshot Snapshot
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("%w: decode: %v", errInvalidResult, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Snapshot{}, fmt.Errorf("%w: trailing JSON", errInvalidResult)
	}
	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func (s Snapshot) Validate() error {
	if s.SchemaVersion != 1 || !portfolioID.MatchString(s.PortfolioID) || !currency.MatchString(s.BaseCurrency) {
		return errInvalidResult
	}
	if _, err := time.Parse(time.RFC3339, s.AsOf); err != nil {
		return fmt.Errorf("%w: as_of", errInvalidResult)
	}
	if s.InputObjectCount < 1 || !hexSHA256.MatchString(s.InputSetSHA256) || !decimal8.MatchString(s.TotalMarketValue) {
		return errInvalidResult
	}
	expectedSilver := fmt.Sprintf("silver/market_prices/v1/run=%s/part-00000.parquet", s.InputSetSHA256)
	expectedGold := fmt.Sprintf(
		"gold/portfolio_allocations/v1/portfolio=%s/run=%s/allocation.parquet",
		s.PortfolioID,
		s.InputSetSHA256,
	)
	if s.SilverParquetObject != expectedSilver || s.GoldParquetObject != expectedGold {
		return fmt.Errorf("%w: data product key", errInvalidResult)
	}
	if len(s.Positions) == 0 {
		return fmt.Errorf("%w: no positions", errInvalidResult)
	}
	seen := make(map[string]struct{}, len(s.Positions))
	for _, position := range s.Positions {
		if !instrument.MatchString(position.Instrument) || !decimal8.MatchString(position.Quantity) ||
			!decimal8.MatchString(position.Price) || !decimal8.MatchString(position.MarketValue) ||
			!decimal4.MatchString(position.AllocationPct) {
			return fmt.Errorf("%w: position %q", errInvalidResult, position.Instrument)
		}
		if _, err := time.Parse(time.RFC3339, position.PriceAsOf); err != nil {
			return fmt.Errorf("%w: price_as_of", errInvalidResult)
		}
		if _, exists := seen[position.Instrument]; exists {
			return fmt.Errorf("%w: duplicate instrument", errInvalidResult)
		}
		seen[position.Instrument] = struct{}{}
	}
	return nil
}
