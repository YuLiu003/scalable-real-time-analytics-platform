package alpaca

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	historyPageLimit    = 10_000
	maximumHistoryBody  = 16 << 20
	maximumHistoryPages = 1_000
)

type History interface {
	Bars(context.Context, []string, time.Time, time.Time) ([]Bar, error)
}

type HistoryClient struct {
	URL       string
	Feed      string
	KeyID     string
	SecretKey string
	HTTP      *http.Client
}

type historyResponse struct {
	Bars          map[string][]historyBar `json:"bars"`
	NextPageToken *string                 `json:"next_page_token"`
}

type historyBar struct {
	Close      json.Number `json:"c"`
	TradeCount int64       `json:"n"`
	Timestamp  string      `json:"t"`
}

func (client HistoryClient) Bars(ctx context.Context, watchlist []string, start, end time.Time) ([]Bar, error) {
	if !start.Before(end) {
		return nil, errors.New("historical market-data interval must have a start before its end")
	}
	allowed := make(map[string]struct{}, len(watchlist))
	for _, instrument := range watchlist {
		allowed[instrument] = struct{}{}
	}
	pageToken := ""
	seenTokens := make(map[string]struct{})
	var bars []Bar
	for page := 0; page < maximumHistoryPages; page++ {
		response, err := client.page(ctx, watchlist, start, end, pageToken)
		if err != nil {
			return nil, err
		}
		for instrument, values := range response.Bars {
			if _, ok := allowed[instrument]; !ok {
				return nil, errors.New("historical provider returned an unrequested instrument")
			}
			for _, value := range values {
				timestamp, err := time.Parse(time.RFC3339Nano, value.Timestamp)
				if err != nil {
					return nil, errors.New("historical provider returned an invalid timestamp")
				}
				bar := Bar{Instrument: instrument, Timestamp: timestamp, Close: value.Close.String(), TradeCount: value.TradeCount}
				if err := bar.Validate(); err != nil {
					return nil, err
				}
				bars = append(bars, bar)
			}
		}
		if response.NextPageToken == nil || *response.NextPageToken == "" {
			sortBars(bars)
			return bars, nil
		}
		next := *response.NextPageToken
		if _, exists := seenTokens[next]; exists {
			return nil, errors.New("historical provider repeated a pagination token")
		}
		seenTokens[next] = struct{}{}
		pageToken = next
	}
	return nil, errors.New("historical provider exceeded the pagination bound")
}

func (client HistoryClient) page(ctx context.Context, watchlist []string, start, end time.Time, pageToken string) (historyResponse, error) {
	endpoint, err := url.Parse(client.URL)
	if err != nil {
		return historyResponse{}, errors.New("historical provider endpoint is invalid")
	}
	query := endpoint.Query()
	query.Set("symbols", strings.Join(watchlist, ","))
	query.Set("timeframe", "1Min")
	query.Set("start", start.UTC().Format(time.RFC3339Nano))
	query.Set("end", end.UTC().Format(time.RFC3339Nano))
	query.Set("limit", fmt.Sprintf("%d", historyPageLimit))
	query.Set("adjustment", "raw")
	query.Set("feed", client.Feed)
	query.Set("sort", "asc")
	if pageToken != "" {
		query.Set("page_token", pageToken)
	}
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return historyResponse{}, errors.New("build historical provider request")
	}
	request.Header.Set("APCA-API-KEY-ID", client.KeyID)
	request.Header.Set("APCA-API-SECRET-KEY", client.SecretKey)
	response, err := client.HTTP.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return historyResponse{}, ctx.Err()
		}
		return historyResponse{}, errors.New("request historical market data")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return historyResponse{}, providerError(response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maximumHistoryBody+1))
	if err != nil {
		return historyResponse{}, errors.New("read historical provider response")
	}
	if len(data) > maximumHistoryBody {
		return historyResponse{}, errors.New("historical provider response exceeded the size bound")
	}
	var result historyResponse
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil || result.Bars == nil {
		return historyResponse{}, errors.New("historical provider returned invalid JSON")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return historyResponse{}, errors.New("historical provider returned multiple JSON values")
	}
	return result, nil
}
