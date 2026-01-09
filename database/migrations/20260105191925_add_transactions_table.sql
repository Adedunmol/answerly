-- +goose Up
-- +goose StatementBegin
CREATE TABLE transactions (
    id BIGSERIAL PRIMARY KEY,
    amount DECIMAL,
    balance_before DECIMAL,
    balance_after DECIMAL,
    reference VARCHAR(255),
    status VARCHAR,
    wallet_id BIGSERIAL references wallets(id),
    type VARCHAR,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE transactions;
-- +goose StatementEnd
