-- +goose Up
-- +goose StatementBegin
CREATE TABLE payment_methods (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    type VARCHAR(30) NOT NULL, -- e.g. 'bank', 'mobile_money'
    provider VARCHAR(50), -- e.g. 'GTBank', 'MTN', 'Airtel'
    account_name VARCHAR(100),
    account_number VARCHAR(30),  -- encrypted
    phone_number VARCHAR(20),    -- encrypted
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP
);-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE payment_methods;
-- +goose StatementEnd
