package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/feaziest/kfdesktopbe/internal/db"
	"github.com/feaziest/kfdesktopbe/internal/domain"
)

// NoteService manages notes, quicknote capture, project notes, and note link graph
type NoteService struct {
	queries db.Querier
	pool    *pgxpool.Pool
}

// NewNoteService creates a new NoteService
func NewNoteService(queries db.Querier, pool *pgxpool.Pool) *NoteService {
	return &NoteService{
		queries: queries,
		pool:    pool,
	}
}

// CreateQuicknote captures a note immediately with null project_id and updates daily_stats
func (s *NoteService) CreateQuicknote(ctx context.Context, uid string, content string) (*domain.Note, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("%w: quicknote content cannot be empty", domain.ErrValidation)
	}

	title := "Quick Note"
	trimmed := strings.TrimSpace(content)
	lines := strings.Split(trimmed, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		firstLine = strings.TrimPrefix(firstLine, "# ")
		firstLine = strings.TrimSpace(firstLine)
		runes := []rune(firstLine)
		if len(runes) > 50 {
			firstLine = string(runes[:50])
		}
		if firstLine != "" {
			title = firstLine
		}
	}

	n, err := s.queries.CreateQuicknote(ctx, db.CreateQuicknoteParams{
		UserID:  uid,
		Title:   title,
		Content: content,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create quicknote: %w", err)
	}

	// Increment daily stats for heatmap (non-blocking for latency optimization)
	_, _ = s.queries.IncrementDailyStat(ctx, db.IncrementDailyStatParams{
		UserID:   uid,
		StatDate: pgtype.Date{Time: time.Now().UTC(), Valid: true},
	})

	res := mapNoteToDomain(n)
	return &res, nil
}

// ListQuicknotes returns all unassigned notes (project_id IS NULL) for the user
func (s *NoteService) ListQuicknotes(ctx context.Context, uid string) ([]domain.Note, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}

	dbNotes, err := s.queries.ListQuicknotes(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to list quicknotes: %w", err)
	}

	notes := make([]domain.Note, 0, len(dbNotes))
	for _, n := range dbNotes {
		notes = append(notes, mapNoteToDomain(n))
	}

	return notes, nil
}

// CreateProjectNote validates project ownership, inserts the note, and manages link graph relations
func (s *NoteService) CreateProjectNote(ctx context.Context, uid string, projectID string, req domain.CreateNoteRequest) (*domain.Note, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}

	pUUID, err := parseUUID(projectID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid project id format", domain.ErrValidation)
	}

	project, err := s.queries.GetProjectByID(ctx, pUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to fetch project: %w", err)
	}

	if project.UserID != uid {
		return nil, domain.ErrForbidden
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Untitled"
	}

	n, err := s.queries.CreateProjectNote(ctx, db.CreateProjectNoteParams{
		UserID:    uid,
		ProjectID: pUUID,
		Title:     title,
		Content:   req.Content,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create project note: %w", err)
	}

	// Link relations management: prevent self-links and duplicates
	sourceIDStr := uuidToString(n.ID)
	seen := make(map[string]struct{})
	for _, targetIDStr := range req.LinkedNoteIDs {
		targetIDStr = strings.TrimSpace(targetIDStr)
		if targetIDStr == "" || targetIDStr == sourceIDStr {
			continue // skip empty or self-links
		}
		if _, exists := seen[targetIDStr]; exists {
			continue // skip duplicates
		}
		seen[targetIDStr] = struct{}{}

		targetUUID, err := parseUUID(targetIDStr)
		if err != nil {
			continue // skip invalid UUIDs
		}

		// Cross-tenant check: ensure target note belongs to the same user
		targetNote, err := s.queries.GetNoteByID(ctx, targetUUID)
		if err != nil || targetNote.UserID != uid {
			continue // ignore notes of other users or non-existent notes
		}

		_, _ = s.queries.InsertNoteLink(ctx, db.InsertNoteLinkParams{
			SourceNoteID: n.ID,
			TargetNoteID: targetUUID,
		})
	}

	res := mapNoteToDomain(n)
	return &res, nil
}

// ListProjectNotes retrieves all active notes associated with the specified project
func (s *NoteService) ListProjectNotes(ctx context.Context, uid string, projectID string) ([]domain.Note, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}

	pUUID, err := parseUUID(projectID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid project id format", domain.ErrValidation)
	}

	project, err := s.queries.GetProjectByID(ctx, pUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to fetch project: %w", err)
	}

	if project.UserID != uid {
		return nil, domain.ErrForbidden
	}

	dbNotes, err := s.queries.ListNotesByProject(ctx, db.ListNotesByProjectParams{
		UserID:    uid,
		ProjectID: pUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list project notes: %w", err)
	}

	notes := make([]domain.Note, 0, len(dbNotes))
	for _, n := range dbNotes {
		notes = append(notes, mapNoteToDomain(n))
	}

	return notes, nil
}

// GetNote retrieves a note by ID with ownership verification and its bidirectional linked notes
func (s *NoteService) GetNote(ctx context.Context, uid string, noteID string) (*domain.NoteDetail, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}

	nUUID, err := parseUUID(noteID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid note id format", domain.ErrValidation)
	}

	note, err := s.queries.GetNoteByID(ctx, nUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to fetch note: %w", err)
	}

	if note.UserID != uid {
		return nil, domain.ErrForbidden
	}

	dbLinked, err := s.queries.GetLinkedNotesByNoteID(ctx, nUUID)
	if err != nil {
		dbLinked = []db.Note{}
	}

	linkedNotes := make([]domain.Note, 0, len(dbLinked))
	for _, l := range dbLinked {
		if l.UserID == uid {
			linkedNotes = append(linkedNotes, mapNoteToDomain(l))
		}
	}

	return &domain.NoteDetail{
		Note:        mapNoteToDomain(note),
		LinkedNotes: linkedNotes,
	}, nil
}

// UpdateNote updates note content and title, maintaining link graph integrity
func (s *NoteService) UpdateNote(ctx context.Context, uid string, noteID string, req domain.UpdateNoteRequest) (*domain.Note, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}

	nUUID, err := parseUUID(noteID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid note id format", domain.ErrValidation)
	}

	existing, err := s.queries.GetNoteByID(ctx, nUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to fetch note: %w", err)
	}

	if existing.UserID != uid {
		return nil, domain.ErrForbidden
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = existing.Title
	}

	updated, err := s.queries.UpdateNote(ctx, db.UpdateNoteParams{
		ID:      nUUID,
		UserID:  uid,
		Title:   title,
		Content: req.Content,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update note: %w", err)
	}

	// Update links if specified
	if req.LinkedNoteIDs != nil {
		_ = s.queries.DeleteNoteLinksByNoteID(ctx, updated.ID)

		sourceIDStr := uuidToString(updated.ID)
		seen := make(map[string]struct{})
		for _, targetIDStr := range req.LinkedNoteIDs {
			targetIDStr = strings.TrimSpace(targetIDStr)
			if targetIDStr == "" || targetIDStr == sourceIDStr {
				continue
			}
			if _, exists := seen[targetIDStr]; exists {
				continue
			}
			seen[targetIDStr] = struct{}{}

			targetUUID, err := parseUUID(targetIDStr)
			if err != nil {
				continue
			}

			// Cross-tenant check: ensure target note belongs to the same user
			targetNote, err := s.queries.GetNoteByID(ctx, targetUUID)
			if err != nil || targetNote.UserID != uid {
				continue
			}

			_, _ = s.queries.InsertNoteLink(ctx, db.InsertNoteLinkParams{
				SourceNoteID: updated.ID,
				TargetNoteID: targetUUID,
			})
		}
	}

	res := mapNoteToDomain(updated)
	return &res, nil
}

// DeleteNote marks a note as soft deleted after ownership check
func (s *NoteService) DeleteNote(ctx context.Context, uid string, noteID string) error {
	if strings.TrimSpace(uid) == "" {
		return domain.ErrUnauthorized
	}

	nUUID, err := parseUUID(noteID)
	if err != nil {
		return fmt.Errorf("%w: invalid note id format", domain.ErrValidation)
	}

	existing, err := s.queries.GetNoteByID(ctx, nUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to fetch note: %w", err)
	}

	if existing.UserID != uid {
		return domain.ErrForbidden
	}

	err = s.queries.SoftDeleteNote(ctx, db.SoftDeleteNoteParams{
		ID:     nUUID,
		UserID: uid,
	})
	if err != nil {
		return fmt.Errorf("failed to soft delete note: %w", err)
	}

	return nil
}

// ListDeletedNotes retrieves soft-deleted notes for a user
func (s *NoteService) ListDeletedNotes(ctx context.Context, uid string) ([]domain.Note, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}

	dbNotes, err := s.queries.ListDeletedNotesByUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to list deleted notes: %w", err)
	}

	notes := make([]domain.Note, 0, len(dbNotes))
	for _, n := range dbNotes {
		notes = append(notes, mapNoteToDomain(n))
	}

	return notes, nil
}

// RestoreNote restores a soft-deleted note by resetting deleted_at to NULL
func (s *NoteService) RestoreNote(ctx context.Context, uid string, noteID string) (*domain.Note, error) {
	if strings.TrimSpace(uid) == "" {
		return nil, domain.ErrUnauthorized
	}

	nUUID, err := parseUUID(noteID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid note id format", domain.ErrValidation)
	}

	n, err := s.queries.RestoreNote(ctx, db.RestoreNoteParams{
		ID:     nUUID,
		UserID: uid,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to restore note: %w", err)
	}

	res := mapNoteToDomain(n)
	return &res, nil
}

// HardDeleteNote permanently deletes a note from database
func (s *NoteService) HardDeleteNote(ctx context.Context, uid string, noteID string) error {
	if strings.TrimSpace(uid) == "" {
		return domain.ErrUnauthorized
	}

	nUUID, err := parseUUID(noteID)
	if err != nil {
		return fmt.Errorf("%w: invalid note id format", domain.ErrValidation)
	}

	err = s.queries.HardDeleteNote(ctx, db.HardDeleteNoteParams{
		ID:     nUUID,
		UserID: uid,
	})
	if err != nil {
		return fmt.Errorf("failed to permanently delete note: %w", err)
	}

	return nil
}
