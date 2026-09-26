package service

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/feaziest/kfdesktopbe/internal/db"
	"github.com/feaziest/kfdesktopbe/internal/domain"
)

func parseUUID(s string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

func timestamptzToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func mapUserToDomain(u db.User) *domain.User {
	return &domain.User{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		AvatarURL:   u.AvatarUrl,
		CreatedAt:   timestamptzToTime(u.CreatedAt),
		UpdatedAt:   timestamptzToTime(u.UpdatedAt),
	}
}

func mapProjectToDomain(p db.Project) domain.Project {
	return domain.Project{
		ID:          uuidToString(p.ID),
		UserID:      p.UserID,
		Title:       p.Title,
		Description: p.Description,
		Color:       p.Color,
		Status:      p.Status,
		CreatedAt:   timestamptzToTime(p.CreatedAt),
		UpdatedAt:   timestamptzToTime(p.UpdatedAt),
	}
}

func mapNoteToDomain(n db.Note) domain.Note {
	var projectID *string
	if n.ProjectID.Valid {
		pid := uuidToString(n.ProjectID)
		projectID = &pid
	}
	return domain.Note{
		ID:        uuidToString(n.ID),
		UserID:    n.UserID,
		ProjectID: projectID,
		Title:     n.Title,
		Content:   n.Content,
		CreatedAt: timestamptzToTime(n.CreatedAt),
		UpdatedAt: timestamptzToTime(n.UpdatedAt),
	}
}
