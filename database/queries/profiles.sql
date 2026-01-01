-- name: CreateProfile :exec
INSERT INTO profiles (user_id)
VALUES (sqlc.arg(user_id));

-- name: GetProfile :one
SELECT * FROM profiles WHERE user_id = sqlc.arg(user_id);

-- name: UpdateProfile :one
UPDATE profiles
SET
    first_name = COALESCE(sqlc.narg(first_name), first_name),
    last_name = COALESCE(sqlc.narg(last_name), last_name),
    date_of_birth = COALESCE(sqlc.narg(date_of_birth), date_of_birth),
    gender = COALESCE(sqlc.narg(gender), gender),
    university = COALESCE(sqlc.narg(university), university),
    faculty = COALESCE(sqlc.narg(faculty), faculty),
    location = COALESCE(sqlc.narg(location), location)
WHERE user_id = sqlc.arg(user_id)
RETURNING *;