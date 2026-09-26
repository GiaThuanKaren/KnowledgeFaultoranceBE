package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/feaziest/kfdesktopbe/internal/domain"
	"github.com/feaziest/kfdesktopbe/internal/middleware"
	"github.com/feaziest/kfdesktopbe/internal/service"
)

// QuicknoteRequest defines payload for creating a quicknote
type QuicknoteRequest struct {
	Content string `json:"content"`
}

// NoteHandler handles note-related HTTP endpoints
type NoteHandler struct {
	noteService *service.NoteService
}

// NewNoteHandler creates a new NoteHandler
func NewNoteHandler(noteService *service.NoteService) *NoteHandler {
	return &NoteHandler{
		noteService: noteService,
	}
}

// CreateQuicknote handles POST /api/v1/notes/quick
func (h *NoteHandler) CreateQuicknote(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUID(r.Context())
	if !ok || uid == "" {
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
		return
	}

	var req QuicknoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "ERR_VALIDATION", "Invalid JSON payload")
		return
	}

	if strings.TrimSpace(req.Content) == "" {
		WriteError(w, http.StatusBadRequest, "ERR_VALIDATION", "Quicknote content cannot be empty")
		return
	}

	note, err := h.noteService.CreateQuicknote(r.Context(), uid, req.Content)
	if err != nil {
		HandleError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, note)
}

// ListQuicknotes handles GET /api/v1/notes/quick
func (h *NoteHandler) ListQuicknotes(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUID(r.Context())
	if !ok || uid == "" {
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
		return
	}

	notes, err := h.noteService.ListQuicknotes(r.Context(), uid)
	if err != nil {
		HandleError(w, err)
		return
	}

	if notes == nil {
		notes = []domain.Note{}
	}

	WriteJSON(w, http.StatusOK, notes)
}

// CreateProjectNote handles POST /api/v1/projects/{projectId}/notes
func (h *NoteHandler) CreateProjectNote(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUID(r.Context())
	if !ok || uid == "" {
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
		return
	}

	projectID := chi.URLParam(r, "projectId")
	var req domain.CreateNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "ERR_VALIDATION", "Invalid JSON payload")
		return
	}

	note, err := h.noteService.CreateProjectNote(r.Context(), uid, projectID, req)
	if err != nil {
		HandleError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, note)
}

// ListProjectNotes handles GET /api/v1/projects/{projectId}/notes
func (h *NoteHandler) ListProjectNotes(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUID(r.Context())
	if !ok || uid == "" {
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
		return
	}

	projectID := chi.URLParam(r, "projectId")
	notes, err := h.noteService.ListProjectNotes(r.Context(), uid, projectID)
	if err != nil {
		HandleError(w, err)
		return
	}

	if notes == nil {
		notes = []domain.Note{}
	}

	WriteJSON(w, http.StatusOK, notes)
}

// GetNote handles GET /api/v1/notes/{noteId}
func (h *NoteHandler) GetNote(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUID(r.Context())
	if !ok || uid == "" {
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
		return
	}

	noteID := chi.URLParam(r, "noteId")
	noteDetail, err := h.noteService.GetNote(r.Context(), uid, noteID)
	if err != nil {
		HandleError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, noteDetail)
}

// UpdateNote handles PUT /api/v1/notes/{noteId}
func (h *NoteHandler) UpdateNote(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUID(r.Context())
	if !ok || uid == "" {
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
		return
	}

	noteID := chi.URLParam(r, "noteId")
	var req domain.UpdateNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "ERR_VALIDATION", "Invalid JSON payload")
		return
	}

	note, err := h.noteService.UpdateNote(r.Context(), uid, noteID, req)
	if err != nil {
		HandleError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, note)
}

// DeleteNote handles DELETE /api/v1/notes/{noteId}
func (h *NoteHandler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUID(r.Context())
	if !ok || uid == "" {
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
		return
	}

	noteID := chi.URLParam(r, "noteId")
	if err := h.noteService.DeleteNote(r.Context(), uid, noteID); err != nil {
		HandleError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
