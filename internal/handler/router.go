package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/feaziest/kfdesktopbe/internal/middleware"
	"github.com/feaziest/kfdesktopbe/internal/service"
)

// RouterConfig holds all dependencies required to wire the Chi router
type RouterConfig struct {
	UserService    *service.UserService
	ProjectService *service.ProjectService
	NoteService    *service.NoteService
	DBPool         *pgxpool.Pool
	RedisClient    *redis.Client
	TokenVerifier  middleware.TokenVerifier
	Logger         *slog.Logger
	RateLimit      int // Requests per minute, defaults to 100
}

// NewRouter initializes and wires the Chi router with middleware and handlers
func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	// Global middlewares
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.LoggingMiddleware(cfg.Logger))
	r.Use(middleware.TimeoutMiddleware(10 * time.Second))

	// System handlers
	systemHandler := NewSystemHandler(cfg.DBPool, cfg.RedisClient)
	r.Get("/health", systemHandler.Health)
	r.Get("/ready", systemHandler.Ready)

	// API v1 handlers
	userHandler := NewUserHandler(cfg.UserService)
	projectHandler := NewProjectHandler(cfg.ProjectService)
	noteHandler := NewNoteHandler(cfg.NoteService)

	// Protected routes under /api/v1
	r.Route("/api/v1", func(api chi.Router) {
		api.Use(middleware.FirebaseAuthMiddleware(cfg.TokenVerifier))

		rateLimit := cfg.RateLimit
		if rateLimit <= 0 {
			rateLimit = 100
		}
		if cfg.RedisClient != nil {
			api.Use(middleware.RedisRateLimiter(cfg.RedisClient, rateLimit, time.Minute))
		}

		// User endpoints
		api.Post("/users/sync", userHandler.SyncUser)
		api.Get("/users/me", userHandler.GetMe)
		api.Get("/users/contributions", userHandler.GetContributions)

		// Project endpoints
		api.Post("/projects", projectHandler.CreateProject)
		api.Get("/projects", projectHandler.ListProjects)
		api.Get("/projects/{projectId}", projectHandler.GetProject)
		api.Put("/projects/{projectId}", projectHandler.UpdateProject)
		api.Delete("/projects/{projectId}", projectHandler.DeleteProject)

		// Project-scoped note endpoints
		api.Post("/projects/{projectId}/notes", noteHandler.CreateProjectNote)
		api.Get("/projects/{projectId}/notes", noteHandler.ListProjectNotes)

		// Note endpoints
		api.Post("/notes/quick", noteHandler.CreateQuicknote)
		api.Get("/notes/quick", noteHandler.ListQuicknotes)
		api.Get("/notes/{noteId}", noteHandler.GetNote)
		api.Put("/notes/{noteId}", noteHandler.UpdateNote)
		api.Delete("/notes/{noteId}", noteHandler.DeleteNote)
	})

	return r
}
