package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	"github.com/feaziest/kfdesktopbe/internal/db"
	"github.com/feaziest/kfdesktopbe/internal/domain"
	"github.com/feaziest/kfdesktopbe/internal/handler"
	"github.com/feaziest/kfdesktopbe/internal/service"
)

// mockQuerier implements db.Querier in-memory for testing handler logic
type mockQuerier struct {
	users       map[string]db.User
	projects    map[string]db.Project
	notes       map[string]db.Note
	noteLinks   []db.NoteLink
	dailyStats  map[string]db.DailyStat
	softDeleted map[string]bool
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
	start := int(arg.Offset)
	if start > len(res) {
		return []db.Project{}, nil
	}
	end := start + int(arg.Limit)
	if end > len(res) {
		end = len(res)
	}
	return res[start:end], nil
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
		ProjectID: pgtype.UUID{Valid: false},
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

var _ db.Querier = (*mockQuerier)(nil)

type mockVerifier struct {
	verifyFunc func(ctx context.Context, idToken string) (*auth.Token, error)
}

func (m *mockVerifier) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	if m.verifyFunc != nil {
		return m.verifyFunc(ctx, idToken)
	}
	if idToken == "valid-token" || strings.HasPrefix(idToken, "uid-") {
		uid := idToken
		if idToken == "valid-token" {
			uid = "test-user-id"
		}
		return &auth.Token{UID: uid}, nil
	}
	return nil, errors.New("invalid token")
}

func setupTestRouter(mock *mockQuerier, verifier *mockVerifier) http.Handler {
	userSvc := service.NewUserService(mock, nil)
	projectSvc := service.NewProjectService(mock, nil)
	noteSvc := service.NewNoteService(mock, nil)

	return handler.NewRouter(handler.RouterConfig{
		UserService:    userSvc,
		ProjectService: projectSvc,
		NoteService:    noteSvc,
		TokenVerifier:  verifier,
	})
}

// 1. GET /health -> 200 OK {"status":"ok"}
func TestHealthCheck(t *testing.T) {
	mock := newMockQuerier()
	r := setupTestRouter(mock, &mockVerifier{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var res map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res["status"] != "ok" {
		t.Errorf("expected status ok, got %v", res["status"])
	}
}

// 2. POST /api/v1/notes/quick with empty content -> 400 ERR_VALIDATION
func TestQuicknote_ValidationFailure(t *testing.T) {
	mock := newMockQuerier()
	r := setupTestRouter(mock, &mockVerifier{})

	body := `{"content": "   "}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notes/quick", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var res handler.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if res.Error.Code != "ERR_VALIDATION" {
		t.Errorf("expected error code ERR_VALIDATION, got %v", res.Error.Code)
	}
}

// 3. POST /api/v1/notes/quick with valid content -> 201 Created
func TestQuicknote_Success(t *testing.T) {
	mock := newMockQuerier()
	r := setupTestRouter(mock, &mockVerifier{})

	body := `{"content": "Meeting notes with actionable points"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notes/quick", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var note domain.Note
	if err := json.Unmarshal(rec.Body.Bytes(), &note); err != nil {
		t.Fatalf("failed to decode note body: %v", err)
	}
	if note.ID == "" {
		t.Error("expected note ID to be populated")
	}
	if note.Content != "Meeting notes with actionable points" {
		t.Errorf("unexpected content: %s", note.Content)
	}
	if note.ProjectID != nil {
		t.Errorf("expected quicknote project_id to be nil, got %v", *note.ProjectID)
	}
}

// 4. GET /api/v1/projects pagination
func TestProjects_Pagination(t *testing.T) {
	mock := newMockQuerier()
	r := setupTestRouter(mock, &mockVerifier{})

	// Pre-create 5 projects for test-user-id
	for i := 1; i <= 5; i++ {
		_, _ = mock.CreateProject(context.Background(), db.CreateProjectParams{
			UserID: "test-user-id",
			Title:  "Project " + string(rune('0'+i)),
			Color:  "#0D9488",
			Status: "nextup",
		})
	}

	// Request page 1 with limit 2
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects?page=1&limit=2", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res struct {
		Projects []domain.Project `json:"projects"`
		Total    int              `json:"total"`
		Page     int              `json:"page"`
		Limit    int              `json:"limit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.Total != 5 {
		t.Errorf("expected total 5, got %d", res.Total)
	}
	if len(res.Projects) != 2 {
		t.Errorf("expected 2 projects on page 1, got %d", len(res.Projects))
	}
	if res.Page != 1 || res.Limit != 2 {
		t.Errorf("expected page 1 limit 2, got page %d limit %d", res.Page, res.Limit)
	}
}

// 5. Unauthorized access returns 401 ERR_UNAUTHORIZED
func TestUnauthorizedAccess(t *testing.T) {
	mock := newMockQuerier()
	r := setupTestRouter(mock, &mockVerifier{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	// Missing Authorization header
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}

	var errRes handler.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &errRes); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if errRes.Error.Code != "ERR_UNAUTHORIZED" {
		t.Errorf("expected ERR_UNAUTHORIZED, got %s", errRes.Error.Code)
	}
}

// 6. User sync and GetMe
func TestUserSyncAndGetMe(t *testing.T) {
	mock := newMockQuerier()
	r := setupTestRouter(mock, &mockVerifier{})

	body := `{"id":"test-user-id","email":"user@test.com","display_name":"Test User","avatar_url":"https://pic.png"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/sync", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var synced domain.User
	if err := json.Unmarshal(rec.Body.Bytes(), &synced); err != nil {
		t.Fatalf("failed to unmarshal user: %v", err)
	}
	if synced.Email != "user@test.com" || synced.DisplayName != "Test User" {
		t.Errorf("unexpected user data: %+v", synced)
	}

	// GET /api/v1/users/me
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	meReq.Header.Set("Authorization", "Bearer valid-token")
	meRec := httptest.NewRecorder()
	r.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for me, got %d: %s", meRec.Code, meRec.Body.String())
	}
}

// 7. Cross-tenant project update returns 403 ERR_FORBIDDEN
func TestProjectCrossTenant_Forbidden(t *testing.T) {
	mock := newMockQuerier()
	r := setupTestRouter(mock, &mockVerifier{})

	// Project created by user-alice
	p, err := mock.CreateProject(context.Background(), db.CreateProjectParams{
		UserID: "uid-alice",
		Title:  "Alice's Secret Project",
		Color:  "#0D9488",
		Status: "nextup",
	})
	if err != nil {
		t.Fatalf("failed to create alice project: %v", err)
	}
	pID := uuid.UUID(p.ID.Bytes).String()

	// Bob attempts to update Alice's project
	body := `{"title":"Bob Hacked It"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+pID, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer uid-bob")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", rec.Code, rec.Body.String())
	}

	var res handler.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if res.Error.Code != "ERR_FORBIDDEN" {
		t.Errorf("expected ERR_FORBIDDEN, got %s", res.Error.Code)
	}
}

// 8. GET /ready probe
func TestReadyProbe(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	// When DB pool is nil -> should report unhealthy 503
	h := handler.NewSystemHandler(nil, rdb)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	h.Ready(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503 when db is down, got %d", rec.Code)
	}

	var res map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if res["status"] != "unhealthy" || res["database"] != "down" || res["redis"] != "up" {
		t.Errorf("unexpected ready response: %+v", res)
	}
}

// 9. GET /api/v1/users/contributions
func TestUserContributions(t *testing.T) {
	mock := newMockQuerier()
	mock.dailyStats["test-user-id:2026-09-26"] = db.DailyStat{
		UserID:        "test-user-id",
		StatDate:      pgtype.Date{Time: time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC), Valid: true},
		ActivityCount: 5,
	}

	r := setupTestRouter(mock, &mockVerifier{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/contributions?year=2026", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var stats []domain.DailyStat
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to unmarshal contributions: %v", err)
	}
	if len(stats) != 1 || stats[0].ActivityCount != 5 {
		t.Errorf("unexpected contributions: %+v", stats)
	}
}

// 10. Project CRUD Endpoints
func TestProjectCRUD(t *testing.T) {
	mock := newMockQuerier()
	r := setupTestRouter(mock, &mockVerifier{})

	// Create
	createBody := `{"title":"Integration Project","description":"Testing project CRUD","color":"#0D9488","status":"inprocess"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(createBody))
	createReq.Header.Set("Authorization", "Bearer valid-token")
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", createRec.Code, createRec.Body.String())
	}

	var created domain.Project
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode created project: %v", err)
	}
	if created.Title != "Integration Project" {
		t.Errorf("expected title Integration Project, got %s", created.Title)
	}

	// Get
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+created.ID, nil)
	getReq.Header.Set("Authorization", "Bearer valid-token")
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", getRec.Code, getRec.Body.String())
	}

	// Update
	updateBody := `{"title":"Updated Project Name","description":"Updated desc","color":"#EF4444","status":"completed"}`
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+created.ID, bytes.NewBufferString(updateBody))
	updateReq.Header.Set("Authorization", "Bearer valid-token")
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	r.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	var updated domain.Project
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to decode updated project: %v", err)
	}
	if updated.Title != "Updated Project Name" || updated.Status != "completed" {
		t.Errorf("unexpected updated project: %+v", updated)
	}

	// Delete
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+created.ID, nil)
	delReq.Header.Set("Authorization", "Bearer valid-token")
	delRec := httptest.NewRecorder()
	r.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for delete, got %d: %s", delRec.Code, delRec.Body.String())
	}

	// Verify Get returns 404 after delete
	getAfterDelReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+created.ID, nil)
	getAfterDelReq.Header.Set("Authorization", "Bearer valid-token")
	getAfterDelRec := httptest.NewRecorder()
	r.ServeHTTP(getAfterDelRec, getAfterDelReq)

	if getAfterDelRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found after delete, got %d", getAfterDelRec.Code)
	}
}

// 11. Notes CRUD, Project Notes, and Linked Notes
func TestNotesCRUD_And_Links(t *testing.T) {
	mock := newMockQuerier()
	r := setupTestRouter(mock, &mockVerifier{})

	// 1. Create a project
	p, _ := mock.CreateProject(context.Background(), db.CreateProjectParams{
		UserID: "test-user-id",
		Title:  "Note Hub",
	})
	pID := uuid.UUID(p.ID.Bytes).String()

	// 2. Create Quicknote
	qnBody := `{"content":"First Quicknote Target"}`
	qnReq := httptest.NewRequest(http.MethodPost, "/api/v1/notes/quick", bytes.NewBufferString(qnBody))
	qnReq.Header.Set("Authorization", "Bearer valid-token")
	qnReq.Header.Set("Content-Type", "application/json")
	qnRec := httptest.NewRecorder()
	r.ServeHTTP(qnRec, qnReq)

	var qn domain.Note
	_ = json.Unmarshal(qnRec.Body.Bytes(), &qn)

	// 3. Create Project Note linking to qn
	pnBody := `{"title":"Project Note 1","content":"Details linking to qn","linked_note_ids":["` + qn.ID + `"]}`
	pnReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pID+"/notes", bytes.NewBufferString(pnBody))
	pnReq.Header.Set("Authorization", "Bearer valid-token")
	pnReq.Header.Set("Content-Type", "application/json")
	pnRec := httptest.NewRecorder()
	r.ServeHTTP(pnRec, pnReq)

	if pnRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for project note, got %d: %s", pnRec.Code, pnRec.Body.String())
	}

	var pn domain.Note
	_ = json.Unmarshal(pnRec.Body.Bytes(), &pn)
	if pn.ProjectID == nil || *pn.ProjectID != pID {
		t.Errorf("expected note project_id to be %s, got %v", pID, pn.ProjectID)
	}

	// 4. List Project Notes
	listPnReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pID+"/notes", nil)
	listPnReq.Header.Set("Authorization", "Bearer valid-token")
	listPnRec := httptest.NewRecorder()
	r.ServeHTTP(listPnRec, listPnReq)

	if listPnRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for list project notes, got %d", listPnRec.Code)
	}
	var pnList []domain.Note
	_ = json.Unmarshal(listPnRec.Body.Bytes(), &pnList)
	if len(pnList) != 1 {
		t.Errorf("expected 1 project note, got %d", len(pnList))
	}

	// 5. List Quicknotes
	listQnReq := httptest.NewRequest(http.MethodGet, "/api/v1/notes/quick", nil)
	listQnReq.Header.Set("Authorization", "Bearer valid-token")
	listQnRec := httptest.NewRecorder()
	r.ServeHTTP(listQnRec, listQnReq)

	if listQnRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for list quicknotes, got %d", listQnRec.Code)
	}
	var qnList []domain.Note
	_ = json.Unmarshal(listQnRec.Body.Bytes(), &qnList)
	if len(qnList) != 1 {
		t.Errorf("expected 1 quicknote, got %d", len(qnList))
	}

	// 6. Get Note Detail with Linked Notes
	detailReq := httptest.NewRequest(http.MethodGet, "/api/v1/notes/"+pn.ID, nil)
	detailReq.Header.Set("Authorization", "Bearer valid-token")
	detailRec := httptest.NewRecorder()
	r.ServeHTTP(detailRec, detailReq)

	if detailRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for note detail, got %d: %s", detailRec.Code, detailRec.Body.String())
	}
	var detail domain.NoteDetail
	_ = json.Unmarshal(detailRec.Body.Bytes(), &detail)
	if len(detail.LinkedNotes) != 1 || detail.LinkedNotes[0].ID != qn.ID {
		t.Errorf("expected 1 linked note matching quicknote, got %+v", detail.LinkedNotes)
	}

	// 7. Update Note
	upNoteBody := `{"title":"Updated Note Title","content":"Updated note content"}`
	upNoteReq := httptest.NewRequest(http.MethodPut, "/api/v1/notes/"+pn.ID, bytes.NewBufferString(upNoteBody))
	upNoteReq.Header.Set("Authorization", "Bearer valid-token")
	upNoteReq.Header.Set("Content-Type", "application/json")
	upNoteRec := httptest.NewRecorder()
	r.ServeHTTP(upNoteRec, upNoteReq)

	if upNoteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for note update, got %d: %s", upNoteRec.Code, upNoteRec.Body.String())
	}

	// 8. Cross-tenant update and delete attempts by uid-bob
	bobUpReq := httptest.NewRequest(http.MethodPut, "/api/v1/notes/"+pn.ID, bytes.NewBufferString(upNoteBody))
	bobUpReq.Header.Set("Authorization", "Bearer uid-bob")
	bobUpReq.Header.Set("Content-Type", "application/json")
	bobUpRec := httptest.NewRecorder()
	r.ServeHTTP(bobUpRec, bobUpReq)

	if bobUpRec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for cross-tenant note update, got %d", bobUpRec.Code)
	}

	bobDelReq := httptest.NewRequest(http.MethodDelete, "/api/v1/notes/"+pn.ID, nil)
	bobDelReq.Header.Set("Authorization", "Bearer uid-bob")
	bobDelRec := httptest.NewRecorder()
	r.ServeHTTP(bobDelRec, bobDelReq)

	if bobDelRec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for cross-tenant note delete, got %d", bobDelRec.Code)
	}

	// 9. Owner deletes note
	ownerDelReq := httptest.NewRequest(http.MethodDelete, "/api/v1/notes/"+pn.ID, nil)
	ownerDelReq.Header.Set("Authorization", "Bearer valid-token")
	ownerDelRec := httptest.NewRecorder()
	r.ServeHTTP(ownerDelRec, ownerDelReq)

	if ownerDelRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for owner note delete, got %d", ownerDelRec.Code)
	}

	// 10. Get Note after delete returns 404
	getDelNoteReq := httptest.NewRequest(http.MethodGet, "/api/v1/notes/"+pn.ID, nil)
	getDelNoteReq.Header.Set("Authorization", "Bearer valid-token")
	getDelNoteRec := httptest.NewRecorder()
	r.ServeHTTP(getDelNoteRec, getDelNoteReq)

	if getDelNoteRec.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for soft deleted note, got %d", getDelNoteRec.Code)
	}
}

// 12. Rate Limiting via Redis in Router
func TestRouter_RateLimiting(t *testing.T) {
	mock := newMockQuerier()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	userSvc := service.NewUserService(mock, nil)
	projectSvc := service.NewProjectService(mock, nil)
	noteSvc := service.NewNoteService(mock, nil)

	r := handler.NewRouter(handler.RouterConfig{
		UserService:    userSvc,
		ProjectService: projectSvc,
		NoteService:    noteSvc,
		RedisClient:    rdb,
		TokenVerifier:  &mockVerifier{},
		RateLimit:      3, // Limit to 3 requests
	})

	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound && rec.Code != http.StatusOK {
			t.Fatalf("expected request %d to pass rate limiter, got status %d", i, rec.Code)
		}
	}

	// 4th request should be rate limited (429)
	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req4.Header.Set("Authorization", "Bearer valid-token")
	rec4 := httptest.NewRecorder()
	r.ServeHTTP(rec4, req4)

	if rec4.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 4th request to be 429 ERR_RATE_LIMITED, got %d: %s", rec4.Code, rec4.Body.String())
	}

	var res handler.ErrorEnvelope
	if err := json.Unmarshal(rec4.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode error json: %v", err)
	}
	if res.Error.Code != "ERR_RATE_LIMITED" {
		t.Errorf("expected ERR_RATE_LIMITED, got %s", res.Error.Code)
	}
}

// 13. Swagger Documentation Route
func TestRouter_SwaggerDocumentation(t *testing.T) {
	mock := newMockQuerier()
	userSvc := service.NewUserService(mock, nil)
	projectSvc := service.NewProjectService(mock, nil)
	noteSvc := service.NewNoteService(mock, nil)

	r := handler.NewRouter(handler.RouterConfig{
		UserService:    userSvc,
		ProjectService: projectSvc,
		NoteService:    noteSvc,
		TokenVerifier:  &mockVerifier{},
	})

	// Test GET /api/docs redirects to /api/docs/index.html
	req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("expected 301 Moved Permanently, got %d", rec.Code)
	}

	// Test GET /api/docs/doc.json returns 200 OK with valid JSON
	reqDoc := httptest.NewRequest(http.MethodGet, "/api/docs/doc.json", nil)
	recDoc := httptest.NewRecorder()
	r.ServeHTTP(recDoc, reqDoc)

	if recDoc.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/docs/doc.json, got %d", recDoc.Code)
	}

	var swaggerDoc map[string]any
	if err := json.Unmarshal(recDoc.Body.Bytes(), &swaggerDoc); err != nil {
		t.Fatalf("failed to parse swagger JSON: %v", err)
	}
	if swaggerDoc["swagger"] != "2.0" && swaggerDoc["openapi"] == nil {
		t.Errorf("expected valid swagger/openapi schema, got: %v", swaggerDoc["swagger"])
	}
}

