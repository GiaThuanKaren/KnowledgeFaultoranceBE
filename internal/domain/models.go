package domain

import "time"

// User represents a synced user entity
type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SyncUserParams represents the input to sync a Firebase user into Postgres
type SyncUserParams struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
}

// Project represents a user workspace project
type Project struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	Status      string    `json:"status"` // 'secondary' | 'nextup' | 'inprocess' | 'completed'
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateProjectRequest contains payload for creating a project
type CreateProjectRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Status      string `json:"status"`
}

// UpdateProjectRequest contains payload for updating an existing project
type UpdateProjectRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Status      string `json:"status"`
}

// Note represents a markdown note entity, either quicknote (ProjectID == nil) or project note
type Note struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ProjectID *string   `json:"project_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateNoteRequest contains payload for creating a note within a project
type CreateNoteRequest struct {
	Title         string   `json:"title"`
	Content       string   `json:"content"`
	LinkedNoteIDs []string `json:"linked_note_ids,omitempty"`
}

// UpdateNoteRequest contains payload for updating a note
type UpdateNoteRequest struct {
	Title         string   `json:"title"`
	Content       string   `json:"content"`
	LinkedNoteIDs []string `json:"linked_note_ids,omitempty"`
}

// NoteDetail includes note content along with its bidirectional linked notes
type NoteDetail struct {
	Note
	LinkedNotes []Note `json:"linked_notes"`
}

// DailyStat represents activity count on a specific date for contribution heatmaps
type DailyStat struct {
	UserID        string `json:"user_id"`
	StatDate      string `json:"stat_date"` // YYYY-MM-DD
	ActivityCount int32  `json:"activity_count"`
}
