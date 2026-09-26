package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/feaziest/kfdesktopbe/internal/config"
)

// NewPostgresPool initializes a new PostgreSQL connection pool using pgxpool.
func NewPostgresPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	// Recommended connection pool settings
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	return pool, nil
}

// NewRedisClient initializes a new Redis client.
func NewRedisClient(cfg *config.Config) (*redis.Client, error) {
	// If RedisURL is formatted as redis://... parse with ParseURL
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		// Fallback for simple "host:port" strings
		opt = &redis.Options{
			Addr: cfg.RedisURL,
		}
	}

	client := redis.NewClient(opt)
	return client, nil
}
