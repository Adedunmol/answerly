-- name: CreatePaymentMethod :exec
INSERT INTO payment_methods (user_id, type, provider, account_name, account_number, phone_number)
VALUES (sqlc.arg(user_id), sqlc.arg(type), sqlc.narg(provider), sqlc.arg(account_name), sqlc.narg(account_number), sqlc.narg(phone_number));