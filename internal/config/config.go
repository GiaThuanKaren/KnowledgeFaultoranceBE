package config

import (
	"fmt"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

// Config represents the application configuration.
type Config struct {
	Port                    string
	DatabaseURL             string
	DBHost                  string
	DBPort                  string
	DBUser                  string
	DBPassword              string
	DBName                  string
	DBSSLMode               string
	RedisURL                string
	RedisHost               string
	RedisPort               string
	RedisPassword           string
	FirebaseCredentialsJSON string
	Environment             string
}

// Load loads the configuration from environment variables and an optional .env file.
func Load() (*Config, error) {
	// Try loading .env if it exists, ignore error if file not present
	_ = godotenv.Load()
	
	port := getEnv("PORT", "8080")

	// Individual DB configs
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "kfdesktop")
	dbSSLMode := getEnv("DB_SSLMODE", "disable")

	// If DATABASE_URL is set, use it; otherwise build connection string from individual variables
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		var userPart *url.Userinfo
		if dbPassword != "" {
			userPart = url.UserPassword(dbUser, dbPassword)
		} else if dbUser != "" {
			userPart = url.User(dbUser)
		}

		u := &url.URL{
			Scheme: "postgres",
			User:   userPart,
			Host:   fmt.Sprintf("%s:%s", dbHost, dbPort),
			Path:   "/" + dbName,
		}
		q := u.Query()
		if dbSSLMode != "" {
			q.Set("sslmode", dbSSLMode)
		}
		u.RawQuery = q.Encode()
		dbURL = u.String()
	}

	// Individual Redis configs
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")

	// If REDIS_URL is set, use it; otherwise build from individual variables
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		if redisPassword != "" {
			redisURL = fmt.Sprintf("redis://:%s@%s:%s/0", url.QueryEscape(redisPassword), redisHost, redisPort)
		} else {
			redisURL = fmt.Sprintf("redis://%s:%s/0", redisHost, redisPort)
		}
	}

	firebaseCreds := getEnv("FIREBASE_CREDENTIALS_JSON", "")
	env := getEnv("ENVIRONMENT", "development")

	return &Config{
		Port:                    port,
		DatabaseURL:             dbURL,
		DBHost:                  dbHost,
		DBPort:                  dbPort,
		DBUser:                  dbUser,
		DBPassword:              dbPassword,
		DBName:                  dbName,
		DBSSLMode:               dbSSLMode,
		RedisURL:                redisURL,
		RedisHost:               redisHost,
		RedisPort:               redisPort,
		RedisPassword:           redisPassword,
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
