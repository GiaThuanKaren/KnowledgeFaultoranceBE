package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/feaziest/kfdesktopbe/internal/domain"
	"github.com/feaziest/kfdesktopbe/internal/middleware"
	"github.com/feaziest/kfdesktopbe/internal/service"
)

// ProjectListResponse defines the paginated response for listing projects
type ProjectListResponse struct {
	Projects []domain.Project `json:"projects"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	Limit    int              `json:"limit"`
}

// ProjectHandler handles project-related HTTP requests
type ProjectHandler struct {
	projectService *service.ProjectService
}

// NewProjectHandler creates a new ProjectHandler
func NewProjectHandler(projectService *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{
		projectService: projectService,
	}
}

// CreateProject handles POST /api/v1/projects
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUID(r.Context())
	if !ok || uid == "" {
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
		return
	}

	var req domain.CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "ERR_VALIDATION", "Invalid JSON payload")
		return
	}

	project, err := h.projectService.CreateProject(r.Context(), uid, req)
	if err != nil {
		HandleError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, project)
}

// ListProjects handles GET /api/v1/projects?page=1&limit=20
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUID(r.Context())
	if !ok || uid == "" {
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
		return
	}

	page := 1
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	projects, total, err := h.projectService.ListProjects(r.Context(), uid, page, limit)
	if err != nil {
		HandleError(w, err)
		return
	}

	if projects == nil {
		projects = []domain.Project{}
	}

	WriteJSON(w, http.StatusOK, ProjectListResponse{
		Projects: projects,
		Total:    total,
		Page:     page,
		Limit:    limit,
	})
}

// GetProject handles GET /api/v1/projects/{projectId}
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUID(r.Context())
	if !ok || uid == "" {
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
		return
	}

	projectID := chi.URLParam(r, "projectId")
	project, err := h.projectService.GetProject(r.Context(), uid, projectID)
	if err != nil {
		HandleError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, project)
}

// UpdateProject handles PUT /api/v1/projects/{projectId}
func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUID(r.Context())
	if !ok || uid == "" {
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
		return
	}

	projectID := chi.URLParam(r, "projectId")
	var req domain.UpdateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "ERR_VALIDATION", "Invalid JSON payload")
		return
	}

	project, err := h.projectService.UpdateProject(r.Context(), uid, projectID, req)
	if err != nil {
		HandleError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, project)
}

// DeleteProject handles DELETE /api/v1/projects/{projectId}
func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUID(r.Context())
	if !ok || uid == "" {
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
		return
	}

	projectID := chi.URLParam(r, "projectId")
	if err := h.projectService.DeleteProject(r.Context(), uid, projectID); err != nil {
		HandleError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
