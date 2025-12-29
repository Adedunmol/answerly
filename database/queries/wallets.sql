
-- name: CreateWallet :one
INSERT INTO wallets (balance, user_id)
VALUES ($1, $2)
    RETURNING id, balance, user_id, created_at, updated_at;

-- name: GetWallet :one
SELECT * FROM wallets WHERE user_id = $1;

-- name: TopUpWallet :one
UPDATE wallets
SET balance = balance + sqlc.arg(amount)
WHERE user_id = sqlc.arg(user_id)
RETURNING *;

-- name: ChargeWallet :one
UPDATE wallets
SET balance = balance - sqlc.arg(amount)
WHERE user_id = sqlc.arg(user_id) AND balance >= sqlc.arg(amount)
RETURNING *;