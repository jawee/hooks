-- name: GetRequestsByListener :many
SELECT * FROM requests WHERE listener_id = $1 ORDER BY timestamp ASC;

-- name: CreateRequest :one
INSERT INTO requests (listener_id, headers, body) VALUES ($1, $2, $3)
RETURNING *;
