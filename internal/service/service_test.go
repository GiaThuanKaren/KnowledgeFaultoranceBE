package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/feaziest/kfdesktopbe/internal/db"
	"github.com/feaziest/kfdesktopbe/internal/domain"
	"github.com/feaziest/kfdesktopbe/internal/service"
)

// mockQuerier implements db.Querier in-memory for testing service logic
type mockQuerier struct {
	users         map[string]db.User
	projects      map[string]db.Project
	notes         map[string]db.Note
	noteLinks     []db.NoteLink
	dailyStats    map[string]db.DailyStat // key: user_id + ":" + date
	softDeleted   map[string]bool
	statIncrement int
}

func newMockQuerier() *mockQuerier {
	return &mockQuerier{
		users:       make(map[string]db.User),
		projects:    make(map[string]db.Project),
		notes:       make(map[string]db.Note),
		noteLinks:   make([]db.NoteLink, 0),
		dailyStats:  make(map[string]db.DailyStat),
		softDeleted: make(map[string]bool),
	}
}

func (m *mockQuerier) UpsertUser(ctx context.Context, arg db.UpsertUserParams) (db.User, error) {
	u := db.User{
		ID:          arg.ID,
		Email:       arg.Email,
		DisplayName: arg.DisplayName,
		AvatarUrl:   arg.AvatarUrl,
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.users[arg.ID] = u
	return u, nil
}

func (m *mockQuerier) GetUserByID(ctx context.Context, id string) (db.User, error) {
	u, ok := m.users[id]
	if !ok {
		return db.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (m *mockQuerier) CreateProject(ctx context.Context, arg db.CreateProjectParams) (db.Project, error) {
	newUUID := uuid.New()
	p := db.Project{
		ID:          pgtype.UUID{Bytes: newUUID, Valid: true},
		UserID:      arg.UserID,
		Title:       arg.Title,
		Description: arg.Description,
		Color:       arg.Color,
		Status:      arg.Status,
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.projects[newUUID.String()] = p
	return p, nil
}

func (m *mockQuerier) GetProjectByID(ctx context.Context, id pgtype.UUID) (db.Project, error) {
	idStr := uuid.UUID(id.Bytes).String()
	p, ok := m.projects[idStr]
	if !ok || p.DeletedAt.Valid {
		return db.Project{}, pgx.ErrNoRows
	}
	return p, nil
}

func (m *mockQuerier) GetProjectByIDAndUser(ctx context.Context, arg db.GetProjectByIDAndUserParams) (db.Project, error) {
	idStr := uuid.UUID(arg.ID.Bytes).String()
	p, ok := m.projects[idStr]
	if !ok || p.UserID != arg.UserID || p.DeletedAt.Valid {
		return db.Project{}, pgx.ErrNoRows
	}
	return p, nil
}

func (m *mockQuerier) ListProjectsByUser(ctx context.Context, arg db.ListProjectsByUserParams) ([]db.Project, error) {
	res := make([]db.Project, 0)
	for _, p := range m.projects {
		if p.UserID == arg.UserID && !p.DeletedAt.Valid {
			res = append(res, p)
		}
	}
	return res, nil
}

func (m *mockQuerier) CountProjectsByUser(ctx context.Context, userID string) (int64, error) {
	var count int64
	for _, p := range m.projects {
		if p.UserID == userID && !p.DeletedAt.Valid {
			count++
		}
	}
	return count, nil
}

func (m *mockQuerier) UpdateProject(ctx context.Context, arg db.UpdateProjectParams) (db.Project, error) {
	idStr := uuid.UUID(arg.ID.Bytes).String()
	p, ok := m.projects[idStr]
	if !ok || p.UserID != arg.UserID || p.DeletedAt.Valid {
		return db.Project{}, pgx.ErrNoRows
	}
	p.Title = arg.Title
	p.Description = arg.Description
	p.Color = arg.Color
	p.Status = arg.Status
	p.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.projects[idStr] = p
	return p, nil
}

func (m *mockQuerier) SoftDeleteProject(ctx context.Context, arg db.SoftDeleteProjectParams) error {
	idStr := uuid.UUID(arg.ID.Bytes).String()
	p, ok := m.projects[idStr]
	if !ok || p.UserID != arg.UserID || p.DeletedAt.Valid {
		return pgx.ErrNoRows
	}
	p.DeletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.projects[idStr] = p
	return nil
}

func (m *mockQuerier) CreateQuicknote(ctx context.Context, arg db.CreateQuicknoteParams) (db.Note, error) {
	newUUID := uuid.New()
	n := db.Note{
		ID:        pgtype.UUID{Bytes: newUUID, Valid: true},
		UserID:    arg.UserID,
		ProjectID: pgtype.UUID{Valid: false}, // NULL project_id for quicknote
		Title:     arg.Title,
		Content:   arg.Content,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.notes[newUUID.String()] = n
	return n, nil
}

func (m *mockQuerier) CreateProjectNote(ctx context.Context, arg db.CreateProjectNoteParams) (db.Note, error) {
	newUUID := uuid.New()
	n := db.Note{
		ID:        pgtype.UUID{Bytes: newUUID, Valid: true},
		UserID:    arg.UserID,
		ProjectID: arg.ProjectID,
		Title:     arg.Title,
		Content:   arg.Content,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.notes[newUUID.String()] = n
	return n, nil
}

func (m *mockQuerier) ListQuicknotes(ctx context.Context, userID string) ([]db.Note, error) {
	res := make([]db.Note, 0)
	for _, n := range m.notes {
		if n.UserID == userID && !n.ProjectID.Valid && !n.DeletedAt.Valid {
			res = append(res, n)
		}
	}
	return res, nil
}

func (m *mockQuerier) ListNotesByProject(ctx context.Context, arg db.ListNotesByProjectParams) ([]db.Note, error) {
	res := make([]db.Note, 0)
	for _, n := range m.notes {
		if n.UserID == arg.UserID && n.ProjectID == arg.ProjectID && !n.DeletedAt.Valid {
			res = append(res, n)
		}
	}
	return res, nil
}

func (m *mockQuerier) GetNoteByID(ctx context.Context, id pgtype.UUID) (db.Note, error) {
	idStr := uuid.UUID(id.Bytes).String()
	n, ok := m.notes[idStr]
	if !ok || n.DeletedAt.Valid {
		return db.Note{}, pgx.ErrNoRows
	}
	return n, nil
}

func (m *mockQuerier) GetNoteByIDAndUser(ctx context.Context, arg db.GetNoteByIDAndUserParams) (db.Note, error) {
	idStr := uuid.UUID(arg.ID.Bytes).String()
	n, ok := m.notes[idStr]
	if !ok || n.UserID != arg.UserID || n.DeletedAt.Valid {
		return db.Note{}, pgx.ErrNoRows
	}
	return n, nil
}

func (m *mockQuerier) UpdateNote(ctx context.Context, arg db.UpdateNoteParams) (db.Note, error) {
	idStr := uuid.UUID(arg.ID.Bytes).String()
	n, ok := m.notes[idStr]
	if !ok || n.UserID != arg.UserID || n.DeletedAt.Valid {
		return db.Note{}, pgx.ErrNoRows
	}
	n.Title = arg.Title
	n.Content = arg.Content
	n.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.notes[idStr] = n
	return n, nil
}

func (m *mockQuerier) SoftDeleteNote(ctx context.Context, arg db.SoftDeleteNoteParams) error {
	idStr := uuid.UUID(arg.ID.Bytes).String()
	n, ok := m.notes[idStr]
	if !ok || n.UserID != arg.UserID || n.DeletedAt.Valid {
		return pgx.ErrNoRows
	}
	n.DeletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.notes[idStr] = n
	m.softDeleted[idStr] = true
	return nil
}

func (m *mockQuerier) InsertNoteLink(ctx context.Context, arg db.InsertNoteLinkParams) (db.NoteLink, error) {
	nl := db.NoteLink{
		ID:           pgtype.UUID{Bytes: uuid.New(), Valid: true},
		SourceNoteID: arg.SourceNoteID,
		TargetNoteID: arg.TargetNoteID,
		CreatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.noteLinks = append(m.noteLinks, nl)
	return nl, nil
}

func (m *mockQuerier) GetLinkedNotes(ctx context.Context, sourceNoteID pgtype.UUID) ([]db.Note, error) {
	res := make([]db.Note, 0)
	for _, l := range m.noteLinks {
		if l.SourceNoteID == sourceNoteID {
			tID := uuid.UUID(l.TargetNoteID.Bytes).String()
			if note, ok := m.notes[tID]; ok && !note.DeletedAt.Valid {
				res = append(res, note)
			}
		}
	}
	return res, nil
}

func (m *mockQuerier) GetLinkedNotesByNoteID(ctx context.Context, sourceNoteID pgtype.UUID) ([]db.Note, error) {
	res := make([]db.Note, 0)
	seen := make(map[string]bool)
	for _, l := range m.noteLinks {
		var targetIDStr string
		if l.SourceNoteID == sourceNoteID {
			targetIDStr = uuid.UUID(l.TargetNoteID.Bytes).String()
		} else if l.TargetNoteID == sourceNoteID {
			targetIDStr = uuid.UUID(l.SourceNoteID.Bytes).String()
		}
		if targetIDStr != "" && !seen[targetIDStr] {
			seen[targetIDStr] = true
			if note, ok := m.notes[targetIDStr]; ok && !note.DeletedAt.Valid {
				res = append(res, note)
			}
		}
	}
	return res, nil
}

func (m *mockQuerier) DeleteNoteLinksByNoteID(ctx context.Context, sourceNoteID pgtype.UUID) error {
	remaining := make([]db.NoteLink, 0)
	for _, l := range m.noteLinks {
		if l.SourceNoteID != sourceNoteID && l.TargetNoteID != sourceNoteID {
			remaining = append(remaining, l)
		}
	}
	m.noteLinks = remaining
	return nil
}

func (m *mockQuerier) IncrementDailyStat(ctx context.Context, arg db.IncrementDailyStatParams) (db.DailyStat, error) {
	m.statIncrement++
	dateStr := arg.StatDate.Time.Format("2006-01-02")
	key := arg.UserID + ":" + dateStr
	ds, ok := m.dailyStats[key]
	if !ok {
		ds = db.DailyStat{
			UserID:        arg.UserID,
			StatDate:      arg.StatDate,
			ActivityCount: 1,
		}
	} else {
		ds.ActivityCount++
	}
	m.dailyStats[key] = ds
	return ds, nil
}

func (m *mockQuerier) GetDailyStatsByYear(ctx context.Context, arg db.GetDailyStatsByYearParams) ([]db.DailyStat, error) {
	res := make([]db.DailyStat, 0)
	for _, ds := range m.dailyStats {
		if ds.UserID == arg.UserID {
			res = append(res, ds)
		}
	}
	return res, nil
}

func (m *mockQuerier) HardDeleteProject(ctx context.Context, arg db.HardDeleteProjectParams) error {
	idStr := uuid.UUID(arg.ID.Bytes).String()
	p, ok := m.projects[idStr]
	if !ok || p.UserID != arg.UserID {
		return pgx.ErrNoRows
	}
	delete(m.projects, idStr)
	return nil
}

func (m *mockQuerier) RestoreProject(ctx context.Context, arg db.RestoreProjectParams) (db.Project, error) {
	idStr := uuid.UUID(arg.ID.Bytes).String()
	p, ok := m.projects[idStr]
	if !ok || p.UserID != arg.UserID || !p.DeletedAt.Valid {
		return db.Project{}, pgx.ErrNoRows
	}
	p.DeletedAt = pgtype.Timestamptz{Valid: false}
	p.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.projects[idStr] = p
	return p, nil
}

func (m *mockQuerier) ListDeletedProjectsByUser(ctx context.Context, userID string) ([]db.Project, error) {
	res := make([]db.Project, 0)
	for _, p := range m.projects {
		if p.UserID == userID && p.DeletedAt.Valid {
			res = append(res, p)
		}
	}
	return res, nil
}

func (m *mockQuerier) HardDeleteNote(ctx context.Context, arg db.HardDeleteNoteParams) error {
	idStr := uuid.UUID(arg.ID.Bytes).String()
	n, ok := m.notes[idStr]
	if !ok || n.UserID != arg.UserID {
		return pgx.ErrNoRows
	}
	delete(m.notes, idStr)
	return nil
}

func (m *mockQuerier) RestoreNote(ctx context.Context, arg db.RestoreNoteParams) (db.Note, error) {
	idStr := uuid.UUID(arg.ID.Bytes).String()
	n, ok := m.notes[idStr]
	if !ok || n.UserID != arg.UserID || !n.DeletedAt.Valid {
		return db.Note{}, pgx.ErrNoRows
	}
	n.DeletedAt = pgtype.Timestamptz{Valid: false}
	n.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.notes[idStr] = n
	return n, nil
}

func (m *mockQuerier) ListDeletedNotesByUser(ctx context.Context, userID string) ([]db.Note, error) {
	res := make([]db.Note, 0)
	for _, n := range m.notes {
		if n.UserID == userID && n.DeletedAt.Valid {
			res = append(res, n)
		}
	}
	return res, nil
}

// Ensure mockQuerier implements db.Querier
var _ db.Querier = (*mockQuerier)(nil)

// Test Case 1: CreateQuicknote creates note with null project_id and increments daily_stats.
func TestCreateQuicknote_Success(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	noteSvc := service.NewNoteService(mock, nil)

	content := "Quick test idea that needs immediate capture"
	note, err := noteSvc.CreateQuicknote(ctx, "user-123", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if note == nil {
		t.Fatal("expected note to not be nil")
	}

	if note.ProjectID != nil {
		t.Errorf("expected quicknote project_id to be nil, got %v", *note.ProjectID)
	}

	if note.Content != content {
		t.Errorf("expected content %q, got %q", content, note.Content)
	}

	if mock.statIncrement != 1 {
		t.Errorf("expected daily_stats to be incremented once, got %d", mock.statIncrement)
	}
}

func TestCreateQuicknote_EmptyContent_Validation(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	noteSvc := service.NewNoteService(mock, nil)

	_, err := noteSvc.CreateQuicknote(ctx, "user-123", "   ")
	if err == nil {
		t.Fatal("expected validation error for empty content, got nil")
	}

	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

// Test Case 2: UpdateProject checks user_id == uid, fails with ErrForbidden if someone else's project.
func TestUpdateProject_OwnershipCheck(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	projectSvc := service.NewProjectService(mock, nil)

	// User 1 creates project
	p, err := projectSvc.CreateProject(ctx, "user-owner", domain.CreateProjectRequest{
		Title:       "Owner Project",
		Description: "Owner Project Description",
		Color:       "#0D9488",
		Status:      "nextup",
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// User 2 attempts to update User 1's project -> should fail with ErrForbidden
	_, err = projectSvc.UpdateProject(ctx, "user-attacker", p.ID, domain.UpdateProjectRequest{
		Title:       "Hacked Title",
		Description: "Hacked Description",
	})
	if err == nil {
		t.Fatal("expected error on cross-tenant update, got nil")
	}

	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}

	// Owner updates -> should succeed
	updated, err := projectSvc.UpdateProject(ctx, "user-owner", p.ID, domain.UpdateProjectRequest{
		Title:       "Updated Title",
		Description: "Updated Description",
		Color:       "#FF0000",
		Status:      "inprocess",
	})
	if err != nil {
		t.Fatalf("owner update should succeed: %v", err)
	}

	if updated.Title != "Updated Title" {
		t.Errorf("expected updated title %q, got %q", "Updated Title", updated.Title)
	}
}

// Test Case 3: DeleteNote calls soft delete, does not delete physical row.
func TestDeleteNote_SoftDelete(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	noteSvc := service.NewNoteService(mock, nil)

	// Create quicknote
	n, err := noteSvc.CreateQuicknote(ctx, "user-123", "Note to delete")
	if err != nil {
		t.Fatalf("failed to create quicknote: %v", err)
	}

	// Attacker cannot delete
	err = noteSvc.DeleteNote(ctx, "user-attacker", n.ID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden for attacker delete, got %v", err)
	}

	// Owner deletes note
	err = noteSvc.DeleteNote(ctx, "user-123", n.ID)
	if err != nil {
		t.Fatalf("failed to delete note: %v", err)
	}

	// Verify the physical row still exists in mock, but is marked deleted
	if !mock.softDeleted[n.ID] {
		t.Error("expected note to be marked as soft deleted")
	}

	if _, exists := mock.notes[n.ID]; !exists {
		t.Error("expected physical note row to still exist in database")
	}

	// GetNote after soft delete should return ErrNotFound
	_, err = noteSvc.GetNote(ctx, "user-123", n.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for soft-deleted note, got %v", err)
	}
}

// Test Case 4: CreateProjectNote with linked_note_ids creates rows in note_links.
func TestCreateProjectNote_LinkedNotes(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	projectSvc := service.NewProjectService(mock, nil)
	noteSvc := service.NewNoteService(mock, nil)

	p, err := projectSvc.CreateProject(ctx, "user-123", domain.CreateProjectRequest{
		Title: "Knowledge Base",
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Create 2 existing notes to link
	n1, err := noteSvc.CreateQuicknote(ctx, "user-123", "Linked note 1")
	if err != nil {
		t.Fatalf("failed to create note 1: %v", err)
	}
	n2, err := noteSvc.CreateQuicknote(ctx, "user-123", "Linked note 2")
	if err != nil {
		t.Fatalf("failed to create note 2: %v", err)
	}

	// Create project note with linked notes, duplicate ID and self-link attempted
	newNote, err := noteSvc.CreateProjectNote(ctx, "user-123", p.ID, domain.CreateNoteRequest{
		Title:   "Main Topic",
		Content: "Topic content linking to n1 and n2",
		LinkedNoteIDs: []string{
			n1.ID,
			n2.ID,
			n1.ID, // duplicate, should be deduplicated
		},
	})
	if err != nil {
		t.Fatalf("failed to create project note: %v", err)
	}

	if newNote.ProjectID == nil || *newNote.ProjectID != p.ID {
		t.Fatalf("expected project_id %v, got %v", p.ID, newNote.ProjectID)
	}

	// Check note detail with linked notes
	detail, err := noteSvc.GetNote(ctx, "user-123", newNote.ID)
	if err != nil {
		t.Fatalf("failed to get note detail: %v", err)
	}

	if len(detail.LinkedNotes) != 2 {
		t.Errorf("expected 2 linked notes (deduplicated), got %d", len(detail.LinkedNotes))
	}
}

func TestUserService_SyncUser_And_GetMe(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	userSvc := service.NewUserService(mock, nil)

	// SyncUser
	u, err := userSvc.SyncUser(ctx, domain.SyncUserParams{
		ID:          "firebase-uid-abc",
		Email:       "test@example.com",
		DisplayName: "Test User",
		AvatarURL:   "https://example.com/pic.png",
	})
	if err != nil {
		t.Fatalf("SyncUser failed: %v", err)
	}

	if u.ID != "firebase-uid-abc" || u.Email != "test@example.com" {
		t.Errorf("unexpected user data: %+v", u)
	}

	// GetMe
	me, err := userSvc.GetMe(ctx, "firebase-uid-abc")
	if err != nil {
		t.Fatalf("GetMe failed: %v", err)
	}
	if me.DisplayName != "Test User" {
		t.Errorf("expected display name %q, got %q", "Test User", me.DisplayName)
	}

	// GetMe Not Found
	_, err = userSvc.GetMe(ctx, "unknown-uid")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectService_ListProjects_Pagination(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	projectSvc := service.NewProjectService(mock, nil)

	for i := 1; i <= 5; i++ {
		_, err := projectSvc.CreateProject(ctx, "user-paginated", domain.CreateProjectRequest{
			Title: "Project " + string(rune('0'+i)),
		})
		if err != nil {
			t.Fatalf("failed to create project: %v", err)
		}
	}

	projects, total, err := projectSvc.ListProjects(ctx, "user-paginated", 1, 10)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total count 5, got %d", total)
	}
	if len(projects) != 5 {
		t.Errorf("expected 5 projects, got %d", len(projects))
	}
}

func TestGetProject_NotFound_And_Forbidden(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	projectSvc := service.NewProjectService(mock, nil)

	p, err := projectSvc.CreateProject(ctx, "user-owner", domain.CreateProjectRequest{
		Title: "Private Project",
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Non-existent project
	_, err = projectSvc.GetProject(ctx, "user-owner", uuid.New().String())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for non-existent project, got %v", err)
	}

	// Other user attempts to get project
	_, err = projectSvc.GetProject(ctx, "user-other", p.ID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden on cross-tenant access, got %v", err)
	}

	// Owner can get project
	got, err := projectSvc.GetProject(ctx, "user-owner", p.ID)
	if err != nil {
		t.Fatalf("owner should be able to get project: %v", err)
	}
	if got.Title != "Private Project" {
		t.Errorf("expected title 'Private Project', got %q", got.Title)
	}
}

func TestDeleteProject_NotFound_And_Forbidden(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	projectSvc := service.NewProjectService(mock, nil)

	p, err := projectSvc.CreateProject(ctx, "user-owner", domain.CreateProjectRequest{
		Title: "To be deleted",
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Attacker attempts delete
	err = projectSvc.DeleteProject(ctx, "user-attacker", p.ID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden on delete by non-owner, got %v", err)
	}

	// Owner deletes
	err = projectSvc.DeleteProject(ctx, "user-owner", p.ID)
	if err != nil {
		t.Fatalf("owner delete should succeed: %v", err)
	}

	// Deleted project should return ErrNotFound
	_, err = projectSvc.GetProject(ctx, "user-owner", p.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for deleted project, got %v", err)
	}
}

func TestGetContributions_YearFilter(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	userSvc := service.NewUserService(mock, nil)

	// Increment stat
	mock.dailyStats["user-contrib:2026-09-26"] = db.DailyStat{
		UserID:        "user-contrib",
		StatDate:      pgtype.Date{Time: time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC), Valid: true},
		ActivityCount: 3,
	}

	stats, err := userSvc.GetContributions(ctx, "user-contrib", 2026)
	if err != nil {
		t.Fatalf("GetContributions failed: %v", err)
	}
	if len(stats) != 1 {
		t.Errorf("expected 1 stat, got %d", len(stats))
	}
	if stats[0].ActivityCount != 3 {
		t.Errorf("expected activity count 3, got %d", stats[0].ActivityCount)
	}
}

func TestListQuicknotes(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	noteSvc := service.NewNoteService(mock, nil)

	_, err := noteSvc.CreateQuicknote(ctx, "user-qn", "Quicknote 1")
	if err != nil {
		t.Fatalf("create quicknote 1 failed: %v", err)
	}
	_, err = noteSvc.CreateQuicknote(ctx, "user-qn", "Quicknote 2")
	if err != nil {
		t.Fatalf("create quicknote 2 failed: %v", err)
	}

	notes, err := noteSvc.ListQuicknotes(ctx, "user-qn")
	if err != nil {
		t.Fatalf("ListQuicknotes failed: %v", err)
	}
	if len(notes) != 2 {
		t.Errorf("expected 2 quicknotes, got %d", len(notes))
	}
}

func TestUpdateNote_OwnershipCheck(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	noteSvc := service.NewNoteService(mock, nil)

	n, err := noteSvc.CreateQuicknote(ctx, "user-owner", "Original content")
	if err != nil {
		t.Fatalf("failed to create quicknote: %v", err)
	}

	// Cross-tenant update attempt
	_, err = noteSvc.UpdateNote(ctx, "user-attacker", n.ID, domain.UpdateNoteRequest{
		Title:   "Hacked",
		Content: "Hacked content",
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden for cross-tenant note update, got %v", err)
	}

	// Owner update
	updated, err := noteSvc.UpdateNote(ctx, "user-owner", n.ID, domain.UpdateNoteRequest{
		Title:   "Updated Title",
		Content: "Updated content",
	})
	if err != nil {
		t.Fatalf("owner note update should succeed: %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Errorf("expected updated title, got %q", updated.Title)
	}
}

func TestCreateProject_Validation(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	projectSvc := service.NewProjectService(mock, nil)

	// Empty title
	_, err := projectSvc.CreateProject(ctx, "user-1", domain.CreateProjectRequest{
		Title: "   ",
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation for empty title, got %v", err)
	}

	// Missing UID
	_, err = projectSvc.CreateProject(ctx, "", domain.CreateProjectRequest{
		Title: "Valid Title",
	})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for empty uid, got %v", err)
	}
}

func TestCreateQuicknote_UTF8_Vietnamese(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	noteSvc := service.NewNoteService(mock, nil)

	// Long Vietnamese sentence with diacritics > 50 runes
	content := "Đây là một ghi chú nhanh bằng tiếng Việt có dấu rất dài hơn năm mươi ký tự để kiểm tra việc cắt chuỗi an toàn không lỗi font."
	note, err := noteSvc.CreateQuicknote(ctx, "user-vn", content)
	if err != nil {
		t.Fatalf("CreateQuicknote failed: %v", err)
	}

	titleRunes := []rune(note.Title)
	if len(titleRunes) != 50 {
		t.Errorf("expected title length 50 runes, got %d", len(titleRunes))
	}

	expectedPrefix := string([]rune(content)[:50])
	if note.Title != expectedPrefix {
		t.Errorf("expected title %q, got %q", expectedPrefix, note.Title)
	}
}

func TestCreateProjectNote_CrossTenantLinkPrevention(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	projectSvc := service.NewProjectService(mock, nil)
	noteSvc := service.NewNoteService(mock, nil)

	p, err := projectSvc.CreateProject(ctx, "user-alice", domain.CreateProjectRequest{
		Title: "Alice's Workspace",
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Alice's note
	aliceNote, err := noteSvc.CreateQuicknote(ctx, "user-alice", "Alice Note 1")
	if err != nil {
		t.Fatalf("failed to create Alice note: %v", err)
	}

	// Bob's note (different user)
	bobNote, err := noteSvc.CreateQuicknote(ctx, "user-bob", "Bob Note Private")
	if err != nil {
		t.Fatalf("failed to create Bob note: %v", err)
	}

	// Alice creates a project note attempting to link to both Alice's note and Bob's note
	newNote, err := noteSvc.CreateProjectNote(ctx, "user-alice", p.ID, domain.CreateNoteRequest{
		Title:         "Alice Linked Note",
		Content:       "Attempting to link cross-tenant",
		LinkedNoteIDs: []string{aliceNote.ID, bobNote.ID},
	})
	if err != nil {
		t.Fatalf("failed to create note: %v", err)
	}

	// Fetch detail - only Alice's note should be in LinkedNotes
	detail, err := noteSvc.GetNote(ctx, "user-alice", newNote.ID)
	if err != nil {
		t.Fatalf("failed to get note detail: %v", err)
	}

	if len(detail.LinkedNotes) != 1 {
		t.Fatalf("expected exactly 1 linked note (cross-tenant pruned), got %d", len(detail.LinkedNotes))
	}
	if detail.LinkedNotes[0].ID != aliceNote.ID {
		t.Errorf("expected linked note to be Alice's note %s, got %s", aliceNote.ID, detail.LinkedNotes[0].ID)
	}
}

func TestUpdateNote_LinkSynchronization(t *testing.T) {
	ctx := context.Background()
	mock := newMockQuerier()
	projectSvc := service.NewProjectService(mock, nil)
	noteSvc := service.NewNoteService(mock, nil)

	p, err := projectSvc.CreateProject(ctx, "user-sync", domain.CreateProjectRequest{
		Title: "Sync Workspace",
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	n1, _ := noteSvc.CreateQuicknote(ctx, "user-sync", "Target Note 1")
	n2, _ := noteSvc.CreateQuicknote(ctx, "user-sync", "Target Note 2")
	n3, _ := noteSvc.CreateQuicknote(ctx, "user-sync", "Target Note 3")

	// Initially link to n1 and n2
	sourceNote, err := noteSvc.CreateProjectNote(ctx, "user-sync", p.ID, domain.CreateNoteRequest{
		Title:         "Source Note",
		Content:       "Initial links",
		LinkedNoteIDs: []string{n1.ID, n2.ID},
	})
	if err != nil {
		t.Fatalf("failed to create note: %v", err)
	}

	detail, err := noteSvc.GetNote(ctx, "user-sync", sourceNote.ID)
	if err != nil {
		t.Fatalf("failed to get note: %v", err)
	}
	if len(detail.LinkedNotes) != 2 {
		t.Fatalf("expected 2 linked notes, got %d", len(detail.LinkedNotes))
	}

	// Update links: remove n1 and n2, link only to n3
	_, err = noteSvc.UpdateNote(ctx, "user-sync", sourceNote.ID, domain.UpdateNoteRequest{
		Title:         "Source Note",
		Content:       "Updated links to n3",
		LinkedNoteIDs: []string{n3.ID},
	})
	if err != nil {
		t.Fatalf("UpdateNote failed: %v", err)
	}

	detailAfterUpdate, err := noteSvc.GetNote(ctx, "user-sync", sourceNote.ID)
	if err != nil {
		t.Fatalf("failed to get note after update: %v", err)
	}
	if len(detailAfterUpdate.LinkedNotes) != 1 {
		t.Fatalf("expected 1 linked note after update, got %d", len(detailAfterUpdate.LinkedNotes))
	}
	if detailAfterUpdate.LinkedNotes[0].ID != n3.ID {
		t.Errorf("expected linked note to be %s, got %s", n3.ID, detailAfterUpdate.LinkedNotes[0].ID)
	}

	// Clear all links
	_, err = noteSvc.UpdateNote(ctx, "user-sync", sourceNote.ID, domain.UpdateNoteRequest{
		Title:         "Source Note",
		Content:       "All links cleared",
		LinkedNoteIDs: []string{},
	})
	if err != nil {
		t.Fatalf("UpdateNote clear links failed: %v", err)
	}

	detailCleared, err := noteSvc.GetNote(ctx, "user-sync", sourceNote.ID)
	if err != nil {
		t.Fatalf("failed to get note after clear: %v", err)
	}
	if len(detailCleared.LinkedNotes) != 0 {
		t.Errorf("expected 0 linked notes after clearing, got %d", len(detailCleared.LinkedNotes))
	}
}

