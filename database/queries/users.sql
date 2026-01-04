
-- name: CreateUser :one
INSERT INTO users (email, password, role, google_id, auth_provider)
VALUES ($1, $2, $3, sqlc.narg(google_id), sqlc.narg(auth_provider))
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: UpdateUser :exec
UPDATE users
SET
    email_verified = COALESCE(sqlc.narg(email_verified), email_verified),
    password = COALESCE(sqlc.narg(password), password),
    refresh_token = COALESCE(sqlc.narg(refresh_token), refresh_token)
WHERE id = sqlc.arg(id);

-- name: RotateRefreshToken :exec
UPDATE users
SET refresh_token = sqlc.arg(new_token)
WHERE refresh_token = sqlc.arg(old_token);

-- name: DeleteRefreshToken :exec
UPDATE users SET refresh_token = ''
WHERE refresh_token = $1;

-- name: GetUserWithRefreshToken :one
SELECT * FROM users
WHERE refresh_token = $1;