-- name: GetListenerByUUID :one
SELECT * FROM listeners WHERE uuid = $1;
