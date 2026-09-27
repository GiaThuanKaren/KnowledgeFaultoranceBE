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
// @Summary      Sync user profile
// @Description  Create or update user profile synchronized from Firebase Auth into PostgreSQL
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      domain.SyncUserParams  true  "User sync payload"
// @Success      200      {object}  domain.User
// @Failure      400      {object}  ErrorEnvelope
// @Failure      401      {object}  ErrorEnvelope
// @Router       /api/v1/users/sync [post]
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
// @Summary      Get current user profile
// @Description  Retrieve the authenticated user's profile information
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  domain.User
// @Failure      401  {object}  ErrorEnvelope
// @Failure      404  {object}  ErrorEnvelope
// @Router       /api/v1/users/me [get]
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
// @Summary      Get user daily activity contributions
// @Description  Retrieve 365-day contribution activity stats for heatmaps
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        year  query     int  false  "Year for stats (defaults to current year)"
// @Success      200   {array}   domain.DailyStat
// @Failure      401   {object}  ErrorEnvelope
// @Router       /api/v1/users/contributions [get]
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
