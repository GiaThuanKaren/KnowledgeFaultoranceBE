package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/feaziest/kfdesktopbe/internal/config"
	"github.com/feaziest/kfdesktopbe/internal/database"
)

func TestNewPostgresPool_InvalidURL(t *testing.T) {
	cfg := &config.Config{
		DatabaseURL: "invalid://bad url with spaces",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pool, err := database.NewPostgresPool(ctx, cfg)
	if err == nil {
		if pool != nil {
			pool.Close()
		}
		t.Fatal("expected error for invalid database URL, got nil")
	}
}

func TestNewRedisClient(t *testing.T) {
	cfg := &config.Config{
		RedisURL: "redis://localhost:6379/0",
	}

	client, err := database.NewRedisClient(cfg)
	if err != nil {
		t.Fatalf("expected no error creating redis client, got %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil redis client")
	}
	_ = client.Close()
}

func TestNewRedisClient_AddrFallback(t *testing.T) {
	cfg := &config.Config{
		RedisURL: "localhost:6379",
	}

	client, err := database.NewRedisClient(cfg)
	if err != nil {
		t.Fatalf("expected no error creating redis client with raw address, got %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil redis client")
	}
	_ = client.Close()
}
