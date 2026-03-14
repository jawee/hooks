-- name: GetListenersByUser :many
SELECT * FROM listeners WHERE user_id = $1;

-- name: CreateListener :one
INSERT INTO listeners (uuid, user_id) VALUES ($1, $2)
RETURNING *;
