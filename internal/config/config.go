package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config represents the application configuration.
type Config struct {
	Port                    string
	DatabaseURL             string
	RedisURL                string
	FirebaseCredentialsJSON string
	Environment             string
}

// Load loads the configuration from environment variables and an optional .env file.
func Load() (*Config, error) {
	// Try loading .env if it exists, ignore error if file not present
	_ = godotenv.Load()

	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/kfdesktop?sslmode=disable")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379/0")
	firebaseCreds := getEnv("FIREBASE_CREDENTIALS_JSON", "")
	env := getEnv("ENVIRONMENT", "development")

	return &Config{
		Port:                    port,
		DatabaseURL:             dbURL,
		RedisURL:                redisURL,
		FirebaseCredentialsJSON: firebaseCreds,
		Environment:             env,
	}, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
