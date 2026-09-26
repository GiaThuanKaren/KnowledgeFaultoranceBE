-- name: CreateQuicknote :one
INSERT INTO notes (user_id, project_id, title, content)
VALUES ($1, NULL, $2, $3)
RETURNING *;

-- name: CreateProjectNote :one
INSERT INTO notes (user_id, project_id, title, content)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListQuicknotes :many
SELECT * FROM notes
WHERE user_id = $1 AND project_id IS NULL AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: ListNotesByProject :many
SELECT * FROM notes
WHERE user_id = $1 AND project_id = $2 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetNoteByID :one
SELECT * FROM notes
WHERE id = $1 AND deleted_at IS NULL
LIMIT 1;

-- name: GetNoteByIDAndUser :one
SELECT * FROM notes
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
LIMIT 1;

-- name: UpdateNote :one
UPDATE notes
SET title = $2,
    content = $3,
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteNote :exec
UPDATE notes
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: InsertNoteLink :one
INSERT INTO note_links (source_note_id, target_note_id)
VALUES ($1, $2)
ON CONFLICT (source_note_id, target_note_id) DO UPDATE
SET created_at = note_links.created_at
RETURNING *;

-- name: GetLinkedNotes :many
SELECT n.* FROM notes n
INNER JOIN note_links nl ON n.id = nl.target_note_id
WHERE nl.source_note_id = $1 AND n.deleted_at IS NULL
ORDER BY n.created_at DESC;

-- name: GetLinkedNotesByNoteID :many
SELECT DISTINCT n.* FROM notes n
INNER JOIN note_links nl ON (n.id = nl.target_note_id AND nl.source_note_id = $1)
                         OR (n.id = nl.source_note_id AND nl.target_note_id = $1)
WHERE n.deleted_at IS NULL
ORDER BY n.created_at DESC;

-- name: DeleteNoteLinksByNoteID :exec
DELETE FROM note_links
WHERE source_note_id = $1 OR target_note_id = $1;
