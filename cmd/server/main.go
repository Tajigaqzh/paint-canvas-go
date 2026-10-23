package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"paint-canvas-go/internal/config"
	"paint-canvas-go/internal/httpapi"
	"paint-canvas-go/internal/store"
)

func main() {
	cfg := config.Load()
	if cfg.Environment == "production" && os.Getenv("JWT_SECRET") == "" {
		panic("JWT_SECRET must be set in production")
	}
	logger, err := zap.NewProduction()
	if cfg.Environment == "development" {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		panic(err)
	}
	defer func() { _ = logger.Sync() }()

	db, err := store.Open(cfg.DatabaseDSN)
	if err != nil {
		logger.Fatal("open database", zap.Error(err))
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.New(db, logger, cfg.CORSOrigins, cfg.JWTSecret),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("HTTP server listening", zap.String("addr", cfg.HTTPAddr))
		serveErr <- server.ListenAndServe()
	}()

	stop, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("HTTP server failed", zap.Error(err))
		}
	case <-stop.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown failed", zap.Error(err))
		}
	}
}
