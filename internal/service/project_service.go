package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/feaziest/kfdesktopbe/internal/db"
	"github.com/feaziest/kfdesktopbe/internal/domain"
)

// ProjectService manages workspace projects, pagination, ownership enforcement and soft deletes
type ProjectService struct {
	queries db.Querier
	pool    *pgxpool.Pool
}

// NewProjectService creates a new ProjectService
func NewProjectService(queries db.Querier, pool *pgxpool.Pool) *ProjectService {
	return &ProjectService{
		queries: queries,
		pool:    pool,
	}
}

// CreateProject validates input, applies defaults, and persists a new project
func (s *ProjectService) CreateProject(ctx context.Context, uid string, req domain.CreateProjectRequest) (*domain.Project, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}
	if strings.TrimSpace(req.Title) == "" {
		return nil, fmt.Errorf("%w: project title cannot be empty", domain.ErrValidation)
	}

	color := req.Color
	if strings.TrimSpace(color) == "" {
		color = "#0D9488"
	}

	status := req.Status
	if strings.TrimSpace(status) == "" {
		status = "nextup"
	}

	p, err := s.queries.CreateProject(ctx, db.CreateProjectParams{
		UserID:      uid,
		Title:       req.Title,
		Description: req.Description,
		Color:       color,
		Status:      status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	res := mapProjectToDomain(p)
	return &res, nil
}

// ListProjects retrieves paginated projects for a user along with total count
func (s *ProjectService) ListProjects(ctx context.Context, uid string, page, limit int) ([]domain.Project, int, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, 0, domain.ErrUnauthorized
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	totalCount, err := s.queries.CountProjectsByUser(ctx, uid)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count projects: %w", err)
	}

	dbProjects, err := s.queries.ListProjectsByUser(ctx, db.ListProjectsByUserParams{
		UserID: uid,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list projects: %w", err)
	}

	projects := make([]domain.Project, 0, len(dbProjects))
	for _, p := range dbProjects {
		projects = append(projects, mapProjectToDomain(p))
	}

	return projects, int(totalCount), nil
}

// GetProject checks existence and ownership, returning project details
func (s *ProjectService) GetProject(ctx context.Context, uid string, projectID string) (*domain.Project, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}

	pUUID, err := parseUUID(projectID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid project id format", domain.ErrValidation)
	}

	p, err := s.queries.GetProjectByID(ctx, pUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	if p.UserID != uid {
		return nil, domain.ErrForbidden
	}

	res := mapProjectToDomain(p)
	return &res, nil
}

// UpdateProject verifies project ownership and updates title, description, color, or status
func (s *ProjectService) UpdateProject(ctx context.Context, uid string, projectID string, req domain.UpdateProjectRequest) (*domain.Project, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}

	pUUID, err := parseUUID(projectID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid project id format", domain.ErrValidation)
	}

	existing, err := s.queries.GetProjectByID(ctx, pUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to fetch project: %w", err)
	}

	if existing.UserID != uid {
		return nil, domain.ErrForbidden
	}

	if strings.TrimSpace(req.Title) == "" {
		return nil, fmt.Errorf("%w: project title cannot be empty", domain.ErrValidation)
	}

	color := req.Color
	if strings.TrimSpace(color) == "" {
		color = existing.Color
	}

	status := req.Status
	if strings.TrimSpace(status) == "" {
		status = existing.Status
	}

	updated, err := s.queries.UpdateProject(ctx, db.UpdateProjectParams{
		ID:          pUUID,
		UserID:      uid,
		Title:       req.Title,
		Description: req.Description,
		Color:       color,
		Status:      status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	res := mapProjectToDomain(updated)
	return &res, nil
}

// DeleteProject performs a soft delete after verifying user ownership
func (s *ProjectService) DeleteProject(ctx context.Context, uid string, projectID string) error {
	if strings.TrimSpace(uid) == "" {
		return domain.ErrUnauthorized
	}

	pUUID, err := parseUUID(projectID)
	if err != nil {
		return fmt.Errorf("%w: invalid project id format", domain.ErrValidation)
	}

	existing, err := s.queries.GetProjectByID(ctx, pUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to fetch project: %w", err)
	}

	if existing.UserID != uid {
		return domain.ErrForbidden
	}

	err = s.queries.SoftDeleteProject(ctx, db.SoftDeleteProjectParams{
		ID:     pUUID,
		UserID: uid,
	})
	if err != nil {
		return fmt.Errorf("failed to soft delete project: %w", err)
	}

	return nil
}

// ListDeletedProjects retrieves soft-deleted projects for a user
func (s *ProjectService) ListDeletedProjects(ctx context.Context, uid string) ([]domain.Project, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}

	dbProjects, err := s.queries.ListDeletedProjectsByUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to list deleted projects: %w", err)
	}

	projects := make([]domain.Project, 0, len(dbProjects))
	for _, p := range dbProjects {
		projects = append(projects, mapProjectToDomain(p))
	}

	return projects, nil
}

// RestoreProject restores a soft-deleted project by resetting deleted_at to NULL
func (s *ProjectService) RestoreProject(ctx context.Context, uid string, projectID string) (*domain.Project, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}

	pUUID, err := parseUUID(projectID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid project id format", domain.ErrValidation)
	}

	p, err := s.queries.RestoreProject(ctx, db.RestoreProjectParams{
		ID:     pUUID,
		UserID: uid,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to restore project: %w", err)
	}

	res := mapProjectToDomain(p)
	return &res, nil
}

// HardDeleteProject permanently deletes a project from database
func (s *ProjectService) HardDeleteProject(ctx context.Context, uid string, projectID string) error {
	if strings.TrimSpace(uid) == "" {
		return domain.ErrUnauthorized
	}

	pUUID, err := parseUUID(projectID)
	if err != nil {
		return fmt.Errorf("%w: invalid project id format", domain.ErrValidation)
	}

	err = s.queries.HardDeleteProject(ctx, db.HardDeleteProjectParams{
		ID:     pUUID,
		UserID: uid,
	})
	if err != nil {
		return fmt.Errorf("failed to permanently delete project: %w", err)
	}

	return nil
}
