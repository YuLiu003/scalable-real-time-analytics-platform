package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/services/portfolio-api/internal/result"
)

type ResultReader interface {
	Latest(context.Context, string) ([]byte, error)
}

type Server struct {
	reader    ResultReader
	portfolio string
	dashboard []byte
}

func New(reader ResultReader, portfolio string, dashboard []byte) *Server {
	return &Server{reader: reader, portfolio: portfolio, dashboard: dashboard}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /api/v1/portfolios/{portfolio}/allocation", s.allocation)
	mux.HandleFunc("GET /", s.index)
	return mux
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
	if request.PathValue("portfolio") != s.portfolio {
		http.NotFound(writer, request)
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
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(data)
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
