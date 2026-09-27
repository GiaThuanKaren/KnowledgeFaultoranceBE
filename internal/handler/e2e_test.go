package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/feaziest/kfdesktopbe/internal/domain"
	"github.com/feaziest/kfdesktopbe/internal/handler"
	"github.com/feaziest/kfdesktopbe/internal/middleware"
	"github.com/feaziest/kfdesktopbe/internal/service"
)

func TestE2E_KeyAPIOperations(t *testing.T) {
	// Initialize miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	mockQ := newMockQuerier()
	userSvc := service.NewUserService(mockQ, nil)
	projectSvc := service.NewProjectService(mockQ, nil)
	noteSvc := service.NewNoteService(mockQ, nil)

	r := handler.NewRouter(handler.RouterConfig{
		UserService:    userSvc,
		ProjectService: projectSvc,
		NoteService:    noteSvc,
		DBPool:         nil,
		RedisClient:    rdb,
		TokenVerifier:  middleware.NewDevTokenVerifier(),
		Logger:         nil,
		RateLimit:      1000,
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	client := ts.Client()
	userID := "e2e-user-123"
	authHeader := "Bearer mock-token:" + userID

	// Helper for authenticated requests
	makeReq := func(method, path string, body []byte) *http.Response {
		req, err := http.NewRequest(method, ts.URL+path, bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("failed to create request %s %s: %v", method, path, err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if path != "/health" && path != "/ready" {
			req.Header.Set("Authorization", authHeader)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request %s %s failed: %v", method, path, err)
		}
		return resp
	}

	// 1. GET /health (200 OK)
	t.Run("1. GET /health", func(t *testing.T) {
		resp := makeReq(http.MethodGet, "/health", nil)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}

		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode /health response: %v", err)
		}
		if body["status"] != "ok" {
			t.Fatalf("expected status=ok, got %s", body["status"])
		}
	})

	// 2. GET /ready (verifies response format)
	t.Run("2. GET /ready", func(t *testing.T) {
		resp := makeReq(http.MethodGet, "/ready", nil)
		defer resp.Body.Close()

		// Since dbPool is nil, expected 503 Service Unavailable, but format must contain status, database, redis
		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode /ready response: %v", err)
		}
		if _, ok := body["status"]; !ok {
			t.Errorf("expected 'status' key in /ready response")
		}
		if _, ok := body["database"]; !ok {
			t.Errorf("expected 'database' key in /ready response")
		}
		if _, ok := body["redis"]; !ok {
			t.Errorf("expected 'redis' key in /ready response")
		}
	})

	// 3. POST /api/v1/users/sync
	t.Run("3. POST /api/v1/users/sync", func(t *testing.T) {
		syncPayload := []byte(`{
			"email": "e2e@example.com",
			"display_name": "E2E Tester",
			"avatar_url": "https://example.com/avatar.png"
		}`)
		resp := makeReq(http.MethodPost, "/api/v1/users/sync", syncPayload)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}

		var user domain.User
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			t.Fatalf("failed to decode sync response: %v", err)
		}
		if user.Email != "e2e@example.com" {
			t.Fatalf("expected email e2e@example.com, got %s", user.Email)
		}
		if user.ID != userID {
			t.Fatalf("expected user ID %s, got %s", userID, user.ID)
		}
	})

	// 4. POST /api/v1/notes/quick with latency check (< 200ms)
	var quicknoteID string
	t.Run("4. POST /api/v1/notes/quick (< 200ms)", func(t *testing.T) {
		quickPayload := []byte(`{
			"content": "E2E test quicknote capture"
		}`)

		start := time.Now()
		resp := makeReq(http.MethodPost, "/api/v1/notes/quick", quickPayload)
		latency := time.Since(start)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
		}

		if latency >= 200*time.Millisecond {
			t.Fatalf("quicknote latency %v exceeded threshold of 200ms", latency)
		}

		var note domain.Note
		if err := json.NewDecoder(resp.Body).Decode(&note); err != nil {
			t.Fatalf("failed to decode quicknote response: %v", err)
		}
		if note.ID == "" {
			t.Fatalf("expected non-empty note ID")
		}
		quicknoteID = note.ID
		t.Logf("Quicknote created with ID: %s, latency: %v (< 200ms threshold)", quicknoteID, latency)
	})

	// 5. POST /api/v1/projects & GET /api/v1/projects
	var projectID string
	t.Run("5. POST /api/v1/projects & GET /api/v1/projects", func(t *testing.T) {
		projectPayload := []byte(`{
			"title": "E2E Automated Project",
			"description": "Integration testing project",
			"color": "#3B82F6",
			"status": "inprocess"
		}`)
		resp := makeReq(http.MethodPost, "/api/v1/projects", projectPayload)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
		}

		var proj domain.Project
		if err := json.NewDecoder(resp.Body).Decode(&proj); err != nil {
			t.Fatalf("failed to decode project response: %v", err)
		}
		if proj.ID == "" {
			t.Fatalf("expected non-empty project ID")
		}
		projectID = proj.ID

		// GET /api/v1/projects
		getResp := makeReq(http.MethodGet, "/api/v1/projects", nil)
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", getResp.StatusCode)
		}

		var listResp handler.ProjectListResponse
		if err := json.NewDecoder(getResp.Body).Decode(&listResp); err != nil {
			t.Fatalf("failed to decode project list: %v", err)
		}
		found := false
		for _, p := range listResp.Projects {
			if p.ID == projectID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("created project %s not found in GET /api/v1/projects", projectID)
		}
	})

	// 6. POST /api/v1/projects/{id}/notes & GET /api/v1/notes/{id} (linked notes)
	var projectNoteID string
	t.Run("6. POST /api/v1/projects/{id}/notes & GET /api/v1/notes/{id}", func(t *testing.T) {
		notePayload, err := json.Marshal(domain.CreateNoteRequest{
			Title:         "E2E Linked Project Note",
			Content:       "Note referencing the quicknote",
			LinkedNoteIDs: []string{quicknoteID},
		})
		if err != nil {
			t.Fatalf("failed to marshal note request: %v", err)
		}

		resp := makeReq(http.MethodPost, "/api/v1/projects/"+projectID+"/notes", notePayload)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
		}

		var projNote domain.Note
		if err := json.NewDecoder(resp.Body).Decode(&projNote); err != nil {
			t.Fatalf("failed to decode project note response: %v", err)
		}
		projectNoteID = projNote.ID

		// GET /api/v1/notes/{id} and verify linked_notes contains quicknoteID
		getNoteResp := makeReq(http.MethodGet, "/api/v1/notes/"+projectNoteID, nil)
		defer getNoteResp.Body.Close()

		if getNoteResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", getNoteResp.StatusCode)
		}

		var noteDetail domain.NoteDetail
		if err := json.NewDecoder(getNoteResp.Body).Decode(&noteDetail); err != nil {
			t.Fatalf("failed to decode note detail: %v", err)
		}

		foundLink := false
		for _, linked := range noteDetail.LinkedNotes {
			if linked.ID == quicknoteID {
				foundLink = true
				break
			}
		}
		if !foundLink {
			t.Fatalf("expected linked_notes to contain quicknote %s", quicknoteID)
		}
	})

	// 7. GET /api/v1/users/contributions?year=2026
	t.Run("7. GET /api/v1/users/contributions?year=2026", func(t *testing.T) {
		resp := makeReq(http.MethodGet, "/api/v1/users/contributions?year=2026", nil)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}

		var stats []domain.DailyStat
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			t.Fatalf("failed to decode contributions: %v", err)
		}
		// Activity from Quicknote and Project Note creation should be recorded
		if len(stats) == 0 {
			t.Fatalf("expected non-empty contributions list for year 2026")
		}
		t.Logf("Contributions returned %d entries", len(stats))
	})

	// 8. DELETE /api/v1/projects/{id} (soft-delete verification)
	t.Run("8. DELETE /api/v1/projects/{id} soft-delete", func(t *testing.T) {
		delResp := makeReq(http.MethodDelete, "/api/v1/projects/"+projectID, nil)
		defer delResp.Body.Close()

		if delResp.StatusCode != http.StatusOK && delResp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 200 OK or 204 No Content, got %d", delResp.StatusCode)
		}

		// Subsequent GET /api/v1/projects/{id} must return 404
		getResp := makeReq(http.MethodGet, "/api/v1/projects/"+projectID, nil)
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found after deletion, got %d", getResp.StatusCode)
		}
	})
}
