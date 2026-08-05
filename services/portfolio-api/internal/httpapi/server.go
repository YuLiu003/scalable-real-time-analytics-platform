package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/services/portfolio-api/internal/projection"
	"github.com/YuLiu003/real-time-analytics-platform/services/portfolio-api/internal/result"
)

const maximumProjectionRequestBytes = 8 << 10

type ResultReader interface {
	Latest(context.Context, string) ([]byte, error)
}

type Server struct {
	reader              ResultReader
	portfolio           string
	dashboard           []byte
	authorizationHash   [sha256.Size]byte
	accessTokenRequired bool
}

func New(reader ResultReader, portfolio string, dashboard []byte, accessToken string) *Server {
	return &Server{
		reader:              reader,
		portfolio:           portfolio,
		dashboard:           dashboard,
		authorizationHash:   sha256.Sum256([]byte("Bearer " + accessToken)),
		accessTokenRequired: accessToken != "",
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /api/v1/config", s.config)
	mux.HandleFunc("GET /api/v1/portfolios/{portfolio}/allocation", s.allocation)
	mux.HandleFunc("POST /api/v1/projections/contributions", s.contributionProjection)
	mux.HandleFunc("GET /", s.index)
	return mux
}

func (s *Server) config(writer http.ResponseWriter, _ *http.Request) {
	dataMode := "synthetic"
	if s.accessTokenRequired {
		dataMode = "private"
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(writer).Encode(struct {
		SchemaVersion       int    `json:"schema_version"`
		PortfolioID         string `json:"portfolio_id"`
		DataMode            string `json:"data_mode"`
		AccessTokenRequired bool   `json:"access_token_required"`
	}{
		SchemaVersion:       1,
		PortfolioID:         s.portfolio,
		DataMode:            dataMode,
		AccessTokenRequired: s.accessTokenRequired,
	})
}

func (s *Server) health(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte("ok\n"))
}

func (s *Server) ready(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
	defer cancel()
	if _, err := s.readAndValidate(ctx); err != nil {
		http.Error(writer, "portfolio result unavailable", http.StatusServiceUnavailable)
		return
	}
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte("ready\n"))
}

func (s *Server) allocation(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.PathValue("portfolio") != s.portfolio {
		http.NotFound(writer, request)
		return
	}
	if s.accessTokenRequired && !authorizationMatches(request.Header.Values("Authorization"), s.authorizationHash) {
		writer.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	data, err := s.readAndValidate(ctx)
	if err != nil {
		http.Error(writer, "portfolio result unavailable", http.StatusServiceUnavailable)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(data)
}

func authorizationMatches(values []string, expected [sha256.Size]byte) bool {
	if len(values) != 1 {
		return false
	}
	provided := sha256.Sum256([]byte(values[0]))
	return subtle.ConstantTimeCompare(provided[:], expected[:]) == 1
}

func (s *Server) contributionProjection(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, maximumProjectionRequestBytes)
	input, err := projection.Decode(request.Body)
	if err != nil {
		http.Error(writer, "invalid projection assumptions", http.StatusBadRequest)
		return
	}
	output, err := projection.Calculate(input)
	if err != nil {
		http.Error(writer, "invalid projection assumptions", http.StatusBadRequest)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(writer).Encode(output)
}

func (s *Server) index(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(writer, request)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(s.dashboard)
}

func (s *Server) readAndValidate(ctx context.Context) ([]byte, error) {
	data, err := s.reader.Latest(ctx, s.portfolio)
	if err != nil {
		return nil, err
	}
	snapshot, err := result.DecodeStrict(data)
	if err != nil {
		return nil, err
	}
	if snapshot.PortfolioID != s.portfolio {
		return nil, fmt.Errorf("result portfolio_id %q does not match %q", snapshot.PortfolioID, s.portfolio)
	}
	return data, nil
}
