-- +goose Up
-- +goose StatementBegin

CREATE TYPE gender AS ENUM (
  'male',
  'female',
  'prefer_not_to_say'
);

CREATE TYPE auth_provider AS ENUM (
  'email',
  'google',
);

CREATE TABLE users (
                       id BIGSERIAL PRIMARY KEY,
                       username VARCHAR(255) UNIQUE,
                       email VARCHAR(255) UNIQUE NOT NULL,
                       email_verified BOOLEAN DEFAULT false,
                       password_hash VARCHAR(255),
                       google_id VARCHAR(255) UNIQUE,
                       auth_provider auth_provider DEFAULT 'email',
                       created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                       updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                       deleted_at TIMESTAMP
);

CREATE TABLE wallets (
                         id BIGSERIAL PRIMARY KEY,
                         balance DECIMAL(15,2) DEFAULT 0.00,
                         user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
                         created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                         updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE profiles (
                          id BIGSERIAL PRIMARY KEY,
                          first_name VARCHAR(255),
                          last_name VARCHAR(255),
                          date_of_birth DATE,
                          gender gender,
                          university VARCHAR(255),
                          faculty VARCHAR(255),
                          location VARCHAR(255),
                          user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,  -- One profile per user
                          created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                          updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE otp_verification (
                                  id BIGSERIAL PRIMARY KEY,
                                  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                  code VARCHAR(255) NOT NULL,
                                  expires_at TIMESTAMP NOT NULL,
                                  is_used BOOLEAN DEFAULT false,
                                  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                                  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE fields (
                        id BIGSERIAL PRIMARY KEY,
                        name VARCHAR(255) NOT NULL UNIQUE,
                        created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE interest_areas (
                                id BIGSERIAL PRIMARY KEY,
                                user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                field_id BIGINT NOT NULL REFERENCES fields(id) ON DELETE CASCADE,
                                created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

                                UNIQUE(user_id, field_id)
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_google_id ON users(google_id);
CREATE INDEX idx_users_deleted_at ON users(deleted_at);
CREATE INDEX idx_wallets_user_id ON wallets(user_id);
CREATE INDEX idx_profiles_user_id ON profiles(user_id);
CREATE INDEX idx_otp_code ON otp_verification(user_id, expires_at) WHERE is_used = false;  -- For fast OTP lookup
CREATE INDEX idx_interest_areas_user_id ON interest_areas(user_id);
CREATE INDEX idx_interest_areas_field_id ON interest_areas(field_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS interest_areas;
DROP TABLE IF EXISTS fields;
DROP TABLE IF EXISTS otp_verification;
DROP TABLE IF EXISTS profiles;
DROP TABLE IF EXISTS wallets;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS gender;
-- +goose StatementEnd
