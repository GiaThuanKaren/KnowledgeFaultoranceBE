package config_test

import (
	"os"
	"testing"

	"github.com/feaziest/kfdesktopbe/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any environment variables that might interfere
	os.Unsetenv("PORT")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("REDIS_URL")
	os.Unsetenv("FIREBASE_CREDENTIALS_JSON")
	os.Unsetenv("ENVIRONMENT")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("expected default Port '8080', got %q", cfg.Port)
	}

	expectedDB := "postgres://postgres:postgres@localhost:5432/kfdesktop?sslmode=disable"
	if cfg.DatabaseURL != expectedDB {
		t.Errorf("expected default DatabaseURL %q, got %q", expectedDB, cfg.DatabaseURL)
	}

	expectedRedis := "redis://localhost:6379/0"
	if cfg.RedisURL != expectedRedis {
		t.Errorf("expected default RedisURL %q, got %q", expectedRedis, cfg.RedisURL)
	}

	if cfg.Environment != "development" {
		t.Errorf("expected default Environment 'development', got %q", cfg.Environment)
	}
}

func TestLoad_CustomEnv(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("DATABASE_URL", "postgres://custom:secret@dbhost:5432/proddb?sslmode=require")
	t.Setenv("REDIS_URL", "redis://redishost:6380/1")
	t.Setenv("FIREBASE_CREDENTIALS_JSON", `{"type":"service_account","project_id":"test-proj"}`)
	t.Setenv("ENVIRONMENT", "production")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Port != "9000" {
		t.Errorf("expected Port '9000', got %q", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://custom:secret@dbhost:5432/proddb?sslmode=require" {
		t.Errorf("expected custom DatabaseURL, got %q", cfg.DatabaseURL)
	}
	if cfg.RedisURL != "redis://redishost:6380/1" {
		t.Errorf("expected custom RedisURL, got %q", cfg.RedisURL)
	}
	if cfg.FirebaseCredentialsJSON != `{"type":"service_account","project_id":"test-proj"}` {
		t.Errorf("expected custom FirebaseCredentialsJSON, got %q", cfg.FirebaseCredentialsJSON)
	}
	if cfg.Environment != "production" {
		t.Errorf("expected Environment 'production', got %q", cfg.Environment)
	}
}

func TestLoad_PartialEnv(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("REDIS_URL")
	t.Setenv("PORT", "7070")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Port != "7070" {
		t.Errorf("expected Port '7070', got %q", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://postgres:postgres@localhost:5432/kfdesktop?sslmode=disable" {
		t.Errorf("expected default DatabaseURL, got %q", cfg.DatabaseURL)
	}
}

func TestLoad_IndividualEnvVars(t *testing.T) {
	// Ensure full URLs are not set
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("REDIS_URL")

	// Set individual DB variables
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "appuser")
	t.Setenv("DB_PASSWORD", "appsecret")
	t.Setenv("DB_NAME", "flowdb")
	t.Setenv("DB_SSLMODE", "require")

	// Set individual Redis variables
	t.Setenv("REDIS_HOST", "redis.example.com")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("REDIS_PASSWORD", "redissecret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedDB := "postgres://appuser:appsecret@db.example.com:5433/flowdb?sslmode=require"
	if cfg.DatabaseURL != expectedDB {
		t.Errorf("expected built DatabaseURL %q, got %q", expectedDB, cfg.DatabaseURL)
	}

	expectedRedis := "redis://:redissecret@redis.example.com:6380/0"
	if cfg.RedisURL != expectedRedis {
		t.Errorf("expected built RedisURL %q, got %q", expectedRedis, cfg.RedisURL)
	}

	if cfg.DBHost != "db.example.com" || cfg.DBPort != "5433" || cfg.DBUser != "appuser" || cfg.DBName != "flowdb" {
		t.Errorf("expected individual DB fields to match environment, got %+v", cfg)
	}
	if cfg.RedisHost != "redis.example.com" || cfg.RedisPort != "6380" || cfg.RedisPassword != "redissecret" {
		t.Errorf("expected individual Redis fields to match environment, got %+v", cfg)
	}
}

