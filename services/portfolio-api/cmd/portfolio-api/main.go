package main

import (
	"context"
	_ "embed"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	portfolioaccess "github.com/YuLiu003/real-time-analytics-platform/services/portfolio-api/internal/access"
	"github.com/YuLiu003/real-time-analytics-platform/services/portfolio-api/internal/httpapi"
	"github.com/YuLiu003/real-time-analytics-platform/services/portfolio-api/internal/store"
)

//go:embed web/index.html
var dashboard []byte

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	settings, err := store.FromEnvironment()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	portfolio := envOrDefault("PORTFOLIO_ID", "demo")
	accessToken, err := portfolioaccess.Load(portfolio, os.Getenv("PORTFOLIO_ACCESS_TOKEN_FILE"))
	if err != nil {
		logger.Error("invalid portfolio access configuration", "error", err)
		os.Exit(1)
	}
	objectStore, err := store.New(ctx, settings)
	if err != nil {
		logger.Error("create object store client", "error", err)
		os.Exit(1)
	}
	server := &http.Server{
		Addr:              envOrDefault("HTTP_ADDRESS", ":8080"),
		Handler:           httpapi.New(objectStore, portfolio, dashboard, accessToken).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownContext, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_ = server.Shutdown(shutdownContext)
	}()
	logger.Info("portfolio API started", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("portfolio API stopped", "error", err)
		os.Exit(1)
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
