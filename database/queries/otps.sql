
-- name: DeleteOTP :exec
DELETE FROM otp_verifications WHERE user_id = $1 AND domain = $2;

-- name: GetOTP :one
SELECT code FROM otp_verifications
WHERE user_id = $1 AND domain = $2
ORDER BY created_at
LIMIT 1;

-- name: CreateOTP :exec
INSERT INTO otp_verifications (user_id, code, expires_at, domain)
VALUES ($1, $2, $3, $4);