-- name: GetSessionByID :one
SELECT * FROM sessions WHERE session_id = $1;

-- name: CreateSession :one
INSERT INTO sessions (session_id, user_id) VALUES ($1, $2)
RETURNING *;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE session_id = $1;
