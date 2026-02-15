
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

-- name: GetWalletWithTransactions :many
SELECT
    w.id,
    w.user_id,
    w.balance,
    w.created_at,
    w.updated_at,
    t.id AS transaction_id,
    t.amount AS transaction_amount,
    t.balance_before AS transaction_balance_before,
    t.balance_after AS transaction_balance_after,
    t.reference AS transaction_reference,
    t.status AS transaction_status,
    t.wallet_id AS transaction_wallet_id,
    t.created_at AS transaction_created_at,
    t.updated_at AS transaction_updated_at
FROM wallets w
LEFT JOIN transactions t ON w.id = t.wallet_id
WHERE w.user_id = $1
ORDER BY t.created_at DESC;

-- name: CreateTransaction :exec
INSERT INTO transactions (amount, balance_before, balance_after, reference, status, wallet_id, type)
VALUES (sqlc.arg(amount), sqlc.arg(balance_before), sqlc.arg(balance_after), sqlc.arg(reference), sqlc.arg(status), sqlc.arg(wallet_id), sqlc.arg(type));