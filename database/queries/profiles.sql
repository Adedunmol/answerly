-- name: CreateProfile :exec
INSERT INTO profiles (user_id)
VALUES (sqlc.arg(user_id));