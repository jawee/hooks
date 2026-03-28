-- name: GetListenersByUser :many
SELECT * FROM listeners WHERE user_id = $1;

-- name: CreateListener :one
INSERT INTO listeners (uuid, user_id, name) VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateListenerName :exec
UPDATE listeners SET name = $2 WHERE uuid = $1;
