-- name: CreateProject :one
INSERT INTO projects (user_id, title, description, color, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListProjectsByUser :many
SELECT * FROM projects
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountProjectsByUser :one
SELECT COUNT(*) FROM projects
WHERE user_id = $1 AND deleted_at IS NULL;

-- name: GetProjectByID :one
SELECT * FROM projects
WHERE id = $1 AND deleted_at IS NULL
LIMIT 1;

-- name: GetProjectByIDAndUser :one
SELECT * FROM projects
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
LIMIT 1;

-- name: UpdateProject :one
UPDATE projects
SET title = $2,
    description = $3,
    color = $4,
    status = $5,
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteProject :exec
UPDATE projects
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;
