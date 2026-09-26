package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feaziest/kfdesktopbe/internal/domain"
	"github.com/feaziest/kfdesktopbe/internal/middleware"
	"github.com/feaziest/kfdesktopbe/internal/service"
)

// UserHandler handles user-related HTTP endpoints
type UserHandler struct {
	userService *service.UserService
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// SyncUser handles POST /api/v1/users/sync
func (h *UserHandler) SyncUser(w http.ResponseWriter, r *http.Request) {
	var params domain.SyncUserParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		WriteError(w, http.StatusBadRequest, "ERR_VALIDATION", "Invalid JSON payload")
		return
	}

	if uid, ok := middleware.GetUID(r.Context()); ok && uid != "" {
		params.ID = uid
	}

	user, err := h.userService.SyncUser(r.Context(), params)
	if err != nil {
		HandleError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, user)
}

// GetMe handles GET /api/v1/users/me
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUID(r.Context())
	if !ok || uid == "" {
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
		return
	}

	user, err := h.userService.GetMe(r.Context(), uid)
	if err != nil {
		HandleError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, user)
}

// GetContributions handles GET /api/v1/users/contributions?year=YYYY
func (h *UserHandler) GetContributions(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUID(r.Context())
	if !ok || uid == "" {
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
		return
	}

	var year int
	if yearStr := r.URL.Query().Get("year"); yearStr != "" {
		year, _ = strconv.Atoi(yearStr)
	}

	stats, err := h.userService.GetContributions(r.Context(), uid, year)
	if err != nil {
		HandleError(w, err)
		return
	}

	if stats == nil {
		stats = []domain.DailyStat{}
	}

	WriteJSON(w, http.StatusOK, stats)
}
