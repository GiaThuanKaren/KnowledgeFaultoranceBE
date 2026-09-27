package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	firebase "firebase.google.com/go/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/api/option"

	"github.com/feaziest/kfdesktopbe/internal/config"
	"github.com/feaziest/kfdesktopbe/internal/database"
	"github.com/feaziest/kfdesktopbe/internal/db"
	"github.com/feaziest/kfdesktopbe/internal/handler"
	"github.com/feaziest/kfdesktopbe/internal/middleware"
	"github.com/feaziest/kfdesktopbe/internal/service"
)

// @title           Knowledge Faultorance (FeaziestFlow) API
// @version         1.0
// @description     High-performance REST API backend for Knowledge Faultorance (FeaziestFlow) desktop knowledge base.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    https://github.com/feaziest/kfdesktopbe

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @BasePath  /

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Type "Bearer" followed by a space and your Firebase ID Token (or mock-token:<uid> in development).

func main() {
	// Initialize structured slog logging
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

	// Initialize database connection pool
	var dbPool *pgxpool.Pool
	dbPool, err = database.NewPostgresPool(ctx, cfg)
	if err != nil {
		slog.Warn("database connection failed or pending", "error", err)
	} else {
		defer dbPool.Close()
		slog.Info("database pool initialized successfully")
	}

	// Initialize Redis client
	var redisClient *redis.Client
	redisClient, err = database.NewRedisClient(cfg)
	if err != nil {
		slog.Warn("redis connection failed or pending", "error", err)
	} else {
		defer redisClient.Close()
		slog.Info("redis client initialized successfully")
	}

	// Initialize Firebase Admin SDK or fallback to DevTokenVerifier
	var verifier middleware.TokenVerifier
	if cfg.FirebaseCredentialsJSON != "" {
		opt := option.WithCredentialsJSON([]byte(cfg.FirebaseCredentialsJSON))
		app, err := firebase.NewApp(ctx, nil, opt)
		if err != nil {
			slog.Warn("failed to initialize firebase app, falling back to dev verifier", "error", err)
			verifier = middleware.NewDevTokenVerifier()
		} else {
			authClient, err := app.Auth(ctx)
			if err != nil {
				slog.Warn("failed to create firebase auth client, falling back to dev verifier", "error", err)
				verifier = middleware.NewDevTokenVerifier()
			} else {
				slog.Info("firebase admin auth client initialized successfully")
				verifier = authClient
			}
		}
	} else {
		slog.Warn("FIREBASE_CREDENTIALS_JSON not provided, using dev token verifier")
		verifier = middleware.NewDevTokenVerifier()
	}

	// Initialize db querier and services
	var queries db.Querier
	if dbPool != nil {
		queries = db.New(dbPool)
	}

	userSvc := service.NewUserService(queries, dbPool)
	projectSvc := service.NewProjectService(queries, dbPool)
	noteSvc := service.NewNoteService(queries, dbPool)

	// Mount router
	router := handler.NewRouter(handler.RouterConfig{
		UserService:    userSvc,
		ProjectService: projectSvc,
		NoteService:    noteSvc,
		DBPool:         dbPool,
		RedisClient:    redisClient,
		TokenVerifier:  verifier,
		Logger:         logger,
		RateLimit:      100,
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
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
