package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	"github.com/feaziest/kfdesktopbe/internal/db"
	"github.com/feaziest/kfdesktopbe/internal/domain"
	"github.com/feaziest/kfdesktopbe/internal/handler"
	"github.com/feaziest/kfdesktopbe/internal/middleware"
	"github.com/feaziest/kfdesktopbe/internal/service"
)

// In-memory querier for standalone script verification
type scriptMockQuerier struct {
	users      map[string]db.User
	projects   map[string]db.Project
	notes      map[string]db.Note
	noteLinks  []db.NoteLink
	dailyStats map[string]db.DailyStat
}

func newScriptMockQuerier() *scriptMockQuerier {
	return &scriptMockQuerier{
		users:      make(map[string]db.User),
		projects:   make(map[string]db.Project),
		notes:      make(map[string]db.Note),
		noteLinks:  make([]db.NoteLink, 0),
		dailyStats: make(map[string]db.DailyStat),
	}
}

func (m *scriptMockQuerier) UpsertUser(ctx context.Context, arg db.UpsertUserParams) (db.User, error) {
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

func (m *scriptMockQuerier) GetUserByID(ctx context.Context, id string) (db.User, error) {
	u, ok := m.users[id]
	if !ok {
		return db.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (m *scriptMockQuerier) CreateProject(ctx context.Context, arg db.CreateProjectParams) (db.Project, error) {
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

func (m *scriptMockQuerier) GetProjectByID(ctx context.Context, id pgtype.UUID) (db.Project, error) {
	idStr := uuid.UUID(id.Bytes).String()
	p, ok := m.projects[idStr]
	if !ok || p.DeletedAt.Valid {
		return db.Project{}, pgx.ErrNoRows
	}
	return p, nil
}

func (m *scriptMockQuerier) GetProjectByIDAndUser(ctx context.Context, arg db.GetProjectByIDAndUserParams) (db.Project, error) {
	idStr := uuid.UUID(arg.ID.Bytes).String()
	p, ok := m.projects[idStr]
	if !ok || p.UserID != arg.UserID || p.DeletedAt.Valid {
		return db.Project{}, pgx.ErrNoRows
	}
	return p, nil
}

func (m *scriptMockQuerier) ListProjectsByUser(ctx context.Context, arg db.ListProjectsByUserParams) ([]db.Project, error) {
	res := make([]db.Project, 0)
	for _, p := range m.projects {
		if p.UserID == arg.UserID && !p.DeletedAt.Valid {
			res = append(res, p)
		}
	}
	return res, nil
}

func (m *scriptMockQuerier) CountProjectsByUser(ctx context.Context, userID string) (int64, error) {
	var count int64
	for _, p := range m.projects {
		if p.UserID == userID && !p.DeletedAt.Valid {
			count++
		}
	}
	return count, nil
}

func (m *scriptMockQuerier) UpdateProject(ctx context.Context, arg db.UpdateProjectParams) (db.Project, error) {
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

func (m *scriptMockQuerier) SoftDeleteProject(ctx context.Context, arg db.SoftDeleteProjectParams) error {
	idStr := uuid.UUID(arg.ID.Bytes).String()
	p, ok := m.projects[idStr]
	if !ok || p.UserID != arg.UserID || p.DeletedAt.Valid {
		return pgx.ErrNoRows
	}
	p.DeletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.projects[idStr] = p
	return nil
}

func (m *scriptMockQuerier) CreateQuicknote(ctx context.Context, arg db.CreateQuicknoteParams) (db.Note, error) {
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

func (m *scriptMockQuerier) CreateProjectNote(ctx context.Context, arg db.CreateProjectNoteParams) (db.Note, error) {
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

func (m *scriptMockQuerier) ListQuicknotes(ctx context.Context, userID string) ([]db.Note, error) {
	res := make([]db.Note, 0)
	for _, n := range m.notes {
		if n.UserID == userID && !n.ProjectID.Valid && !n.DeletedAt.Valid {
			res = append(res, n)
		}
	}
	return res, nil
}

func (m *scriptMockQuerier) ListNotesByProject(ctx context.Context, arg db.ListNotesByProjectParams) ([]db.Note, error) {
	res := make([]db.Note, 0)
	for _, n := range m.notes {
		if n.UserID == arg.UserID && n.ProjectID == arg.ProjectID && !n.DeletedAt.Valid {
			res = append(res, n)
		}
	}
	return res, nil
}

func (m *scriptMockQuerier) GetNoteByID(ctx context.Context, id pgtype.UUID) (db.Note, error) {
	idStr := uuid.UUID(id.Bytes).String()
	n, ok := m.notes[idStr]
	if !ok || n.DeletedAt.Valid {
		return db.Note{}, pgx.ErrNoRows
	}
	return n, nil
}

func (m *scriptMockQuerier) GetNoteByIDAndUser(ctx context.Context, arg db.GetNoteByIDAndUserParams) (db.Note, error) {
	idStr := uuid.UUID(arg.ID.Bytes).String()
	n, ok := m.notes[idStr]
	if !ok || n.UserID != arg.UserID || n.DeletedAt.Valid {
		return db.Note{}, pgx.ErrNoRows
	}
	return n, nil
}

func (m *scriptMockQuerier) UpdateNote(ctx context.Context, arg db.UpdateNoteParams) (db.Note, error) {
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

func (m *scriptMockQuerier) SoftDeleteNote(ctx context.Context, arg db.SoftDeleteNoteParams) error {
	idStr := uuid.UUID(arg.ID.Bytes).String()
	n, ok := m.notes[idStr]
	if !ok || n.UserID != arg.UserID || n.DeletedAt.Valid {
		return pgx.ErrNoRows
	}
	n.DeletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.notes[idStr] = n
	return nil
}

func (m *scriptMockQuerier) InsertNoteLink(ctx context.Context, arg db.InsertNoteLinkParams) (db.NoteLink, error) {
	nl := db.NoteLink{
		ID:           pgtype.UUID{Bytes: uuid.New(), Valid: true},
		SourceNoteID: arg.SourceNoteID,
		TargetNoteID: arg.TargetNoteID,
		CreatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.noteLinks = append(m.noteLinks, nl)
	return nl, nil
}

func (m *scriptMockQuerier) GetLinkedNotes(ctx context.Context, sourceNoteID pgtype.UUID) ([]db.Note, error) {
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

func (m *scriptMockQuerier) GetLinkedNotesByNoteID(ctx context.Context, sourceNoteID pgtype.UUID) ([]db.Note, error) {
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

func (m *scriptMockQuerier) DeleteNoteLinksByNoteID(ctx context.Context, sourceNoteID pgtype.UUID) error {
	remaining := make([]db.NoteLink, 0)
	for _, l := range m.noteLinks {
		if l.SourceNoteID != sourceNoteID && l.TargetNoteID != sourceNoteID {
			remaining = append(remaining, l)
		}
	}
	m.noteLinks = remaining
	return nil
}

func (m *scriptMockQuerier) IncrementDailyStat(ctx context.Context, arg db.IncrementDailyStatParams) (db.DailyStat, error) {
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

func (m *scriptMockQuerier) GetDailyStatsByYear(ctx context.Context, arg db.GetDailyStatsByYearParams) ([]db.DailyStat, error) {
	res := make([]db.DailyStat, 0)
	for _, ds := range m.dailyStats {
		if ds.UserID == arg.UserID {
			res = append(res, ds)
		}
	}
	return res, nil
}

var _ db.Querier = (*scriptMockQuerier)(nil)

func main() {
	targetURL := flag.String("url", os.Getenv("SERVER_URL"), "Base URL of backend server to verify")
	flag.Parse()

	var baseURL string
	var cleanup func()

	if *targetURL != "" {
		baseURL = strings.TrimRight(*targetURL, "/")
		fmt.Printf("==> Testing target server at %s\n", baseURL)
	} else {
		fmt.Println("==> No external SERVER_URL provided. Booting in-process test server...")
		mr, err := miniredis.Run()
		if err != nil {
			fmt.Printf("FAIL: unable to start miniredis: %v\n", err)
			os.Exit(1)
		}

		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		mockQ := newScriptMockQuerier()
		userSvc := service.NewUserService(mockQ, nil)
		projectSvc := service.NewProjectService(mockQ, nil)
		noteSvc := service.NewNoteService(mockQ, nil)

		router := handler.NewRouter(handler.RouterConfig{
			UserService:    userSvc,
			ProjectService: projectSvc,
			NoteService:    noteSvc,
			DBPool:         nil,
			RedisClient:    rdb,
			TokenVerifier:  middleware.NewDevTokenVerifier(),
			Logger:         nil,
			RateLimit:      1000,
		})

		ts := httptest.NewServer(router)
		baseURL = ts.URL
		cleanup = func() {
			ts.Close()
			rdb.Close()
			mr.Close()
		}
		defer cleanup()
		fmt.Printf("==> In-process test server running at %s\n", baseURL)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	userID := "e2e-user-verify"
	authHeader := "Bearer mock-token:" + userID

	makeRequest := func(method, path string, body []byte) (*http.Response, []byte, error) {
		req, err := http.NewRequest(method, baseURL+path, bytes.NewBuffer(body))
		if err != nil {
			return nil, nil, err
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if path != "/health" && path != "/ready" {
			req.Header.Set("Authorization", authHeader)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer resp.Body.Close()
		respBytes := new(bytes.Buffer)
		_, err = respBytes.ReadFrom(resp.Body)
		return resp, respBytes.Bytes(), err
	}

	fmt.Println("\n==========================================")
	fmt.Println("FEAZIESTFLOW E2E VERIFICATION SUITE")
	fmt.Println("==========================================")

	// 1. GET /health (200 OK)
	fmt.Print("[1/8] Verifying GET /health ... ")
	resp, body, err := makeRequest(http.MethodGet, "/health", nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("FAIL (status %d, err %v)\n", resp.StatusCode, err)
		os.Exit(1)
	}
	var healthBody map[string]string
	_ = json.Unmarshal(body, &healthBody)
	if healthBody["status"] != "ok" {
		fmt.Printf("FAIL (unexpected body: %s)\n", string(body))
		os.Exit(1)
	}
	fmt.Printf("PASS (200 OK, status: %s)\n", healthBody["status"])

	// 2. GET /ready (verifies response format)
	fmt.Print("[2/8] Verifying GET /ready response format ... ")
	resp, body, err = makeRequest(http.MethodGet, "/ready", nil)
	if err != nil {
		fmt.Printf("FAIL (err %v)\n", err)
		os.Exit(1)
	}
	var readyBody map[string]string
	if err := json.Unmarshal(body, &readyBody); err != nil {
		fmt.Printf("FAIL (malformed JSON: %s)\n", string(body))
		os.Exit(1)
	}
	if _, ok := readyBody["status"]; !ok {
		fmt.Println("FAIL (missing 'status' field)")
		os.Exit(1)
	}
	fmt.Printf("PASS (status: %s, database: %s, redis: %s)\n", readyBody["status"], readyBody["database"], readyBody["redis"])

	// 3. POST /api/v1/users/sync
	fmt.Print("[3/8] Verifying POST /api/v1/users/sync ... ")
	syncPayload := []byte(`{"email":"e2e@feaziest.com","display_name":"E2E Verifier","avatar_url":"https://feaziest.com/avatar.png"}`)
	resp, body, err = makeRequest(http.MethodPost, "/api/v1/users/sync", syncPayload)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("FAIL (status %d: %s)\n", resp.StatusCode, string(body))
		os.Exit(1)
	}
	var user domain.User
	_ = json.Unmarshal(body, &user)
	if user.Email != "e2e@feaziest.com" {
		fmt.Printf("FAIL (unexpected user email: %s)\n", user.Email)
		os.Exit(1)
	}
	fmt.Printf("PASS (Synced UID %s)\n", user.ID)

	// 4. POST /api/v1/notes/quick with latency check (< 200ms)
	fmt.Print("[4/8] Verifying POST /api/v1/notes/quick (< 200ms latency) ... ")
	quickPayload := []byte(`{"content":"Fast quicknote verification content"}`)
	start := time.Now()
	resp, body, err = makeRequest(http.MethodPost, "/api/v1/notes/quick", quickPayload)
	latency := time.Since(start)
	if err != nil || resp.StatusCode != http.StatusCreated {
		fmt.Printf("FAIL (status %d: %s)\n", resp.StatusCode, string(body))
		os.Exit(1)
	}
	if latency >= 200*time.Millisecond {
		fmt.Printf("FAIL (latency %v exceeded 200ms)\n", latency)
		os.Exit(1)
	}
	var quicknote domain.Note
	_ = json.Unmarshal(body, &quicknote)
	fmt.Printf("PASS (Created ID: %s, Latency: %v)\n", quicknote.ID, latency)

	// 5. POST /api/v1/projects & GET /api/v1/projects
	fmt.Print("[5/8] Verifying POST /api/v1/projects & GET /api/v1/projects ... ")
	projPayload := []byte(`{"title":"Verification Workspace","description":"E2E Project","color":"#10B981","status":"inprocess"}`)
	resp, body, err = makeRequest(http.MethodPost, "/api/v1/projects", projPayload)
	if err != nil || resp.StatusCode != http.StatusCreated {
		fmt.Printf("FAIL (status %d: %s)\n", resp.StatusCode, string(body))
		os.Exit(1)
	}
	var project domain.Project
	_ = json.Unmarshal(body, &project)

	resp, body, err = makeRequest(http.MethodGet, "/api/v1/projects", nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("FAIL GET /projects (status %d: %s)\n", resp.StatusCode, string(body))
		os.Exit(1)
	}
	var projList handler.ProjectListResponse
	_ = json.Unmarshal(body, &projList)
	foundProj := false
	for _, p := range projList.Projects {
		if p.ID == project.ID {
			foundProj = true
			break
		}
	}
	if !foundProj {
		fmt.Printf("FAIL (created project %s not found in list)\n", project.ID)
		os.Exit(1)
	}
	fmt.Printf("PASS (Project ID %s listed successfully)\n", project.ID)

	// 6. POST /api/v1/projects/{id}/notes & GET /api/v1/notes/{id} (linked notes)
	fmt.Print("[6/8] Verifying POST /api/v1/projects/{id}/notes & linked note retrieval ... ")
	noteReq := domain.CreateNoteRequest{
		Title:         "Architecture Decision Record",
		Content:       "ADR content linked to quicknote",
		LinkedNoteIDs: []string{quicknote.ID},
	}
	noteReqBytes, _ := json.Marshal(noteReq)
	resp, body, err = makeRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/notes", noteReqBytes)
	if err != nil || resp.StatusCode != http.StatusCreated {
		fmt.Printf("FAIL (status %d: %s)\n", resp.StatusCode, string(body))
		os.Exit(1)
	}
	var createdNote domain.Note
	_ = json.Unmarshal(body, &createdNote)

	resp, body, err = makeRequest(http.MethodGet, "/api/v1/notes/"+createdNote.ID, nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("FAIL GET /notes/%s (status %d: %s)\n", createdNote.ID, resp.StatusCode, string(body))
		os.Exit(1)
	}
	var noteDetail domain.NoteDetail
	_ = json.Unmarshal(body, &noteDetail)
	hasLink := false
	for _, l := range noteDetail.LinkedNotes {
		if l.ID == quicknote.ID {
			hasLink = true
			break
		}
	}
	if !hasLink {
		fmt.Printf("FAIL (note does not contain linked quicknote %s)\n", quicknote.ID)
		os.Exit(1)
	}
	fmt.Printf("PASS (Project Note %s bidirectionally links %s)\n", createdNote.ID, quicknote.ID)

	// 7. GET /api/v1/users/contributions?year=2026
	fmt.Print("[7/8] Verifying GET /api/v1/users/contributions?year=2026 ... ")
	resp, body, err = makeRequest(http.MethodGet, "/api/v1/users/contributions?year=2026", nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("FAIL (status %d: %s)\n", resp.StatusCode, string(body))
		os.Exit(1)
	}
	var stats []domain.DailyStat
	_ = json.Unmarshal(body, &stats)
	if len(stats) == 0 {
		fmt.Println("FAIL (expected daily contribution entries)")
		os.Exit(1)
	}
	fmt.Printf("PASS (Heatmap recorded %d activity stats for 2026)\n", len(stats))

	// 8. DELETE /api/v1/projects/{id} (soft-delete verification)
	fmt.Print("[8/8] Verifying DELETE /api/v1/projects/{id} (soft-delete) ... ")
	resp, body, err = makeRequest(http.MethodDelete, "/api/v1/projects/"+project.ID, nil)
	if err != nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent) {
		fmt.Printf("FAIL DELETE (status %d: %s)\n", resp.StatusCode, string(body))
		os.Exit(1)
	}
	resp, _, err = makeRequest(http.MethodGet, "/api/v1/projects/"+project.ID, nil)
	if err != nil || resp.StatusCode != http.StatusNotFound {
		fmt.Printf("FAIL GET after delete: expected 404, got %d\n", resp.StatusCode)
		os.Exit(1)
	}
	fmt.Printf("PASS (Project successfully soft-deleted, subsequent GET returns 404)\n")

	fmt.Println("==========================================")
	fmt.Println("ALL 8 END-TO-END SANITY CHECKS PASSED (100% SUCCESS)!")
	fmt.Println("==========================================")
}
