package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/feaziest/kfdesktopbe/internal/config"
	"github.com/feaziest/kfdesktopbe/internal/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("starting FeaziestFlow Backend Server",
		"port", cfg.Port,
		"env", cfg.Environment,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize database pool
	dbPool, err := database.NewPostgresPool(ctx, cfg)
	if err != nil {
		slog.Warn("database connection failed or pending", "error", err)
	} else {
		defer dbPool.Close()
		slog.Info("database pool initialized successfully")
	}

	// Initialize redis client
	redisClient, err := database.NewRedisClient(cfg)
	if err != nil {
		slog.Warn("redis connection failed or pending", "error", err)
	} else {
		defer redisClient.Close()
		slog.Info("redis client initialized successfully")
	}

	// Basic HTTP server mux for initial liveness
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server exited cleanly")
}
