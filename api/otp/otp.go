package otp

import (
	"context"
	"errors"
	"fmt"
	"github.com/Adedunmol/answerly/api/tokens"
	"github.com/Adedunmol/answerly/database"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

type Store interface {
	CreateOTP(ctx context.Context, userID int64, code string, expiration time.Time, domain string) error
	GetOTP(ctx context.Context, userID int64, domain string) (string, error)
	DeleteOTP(ctx context.Context, userID int64, domain string) error
}

type Repository struct {
	queries      *database.Queries
	tokenService tokens.TokenService
}

var ErrInvalidOtp = errors.New("invalid OTP")

func NewOTPStore(queries *database.Queries, tokenService tokens.TokenService) *Repository {

	return &Repository{queries: queries, tokenService: tokenService}
}

func (r *Repository) CreateOTP(ctx context.Context, userID int64, code string, expiration time.Time, domain string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	expires := pgtype.Timestamp{}
	err := expires.Scan(expiration)
	if err != nil {
		return fmt.Errorf("error while scanning expiration: %v", err)
	}

	err = r.queries.CreateOTP(ctx, database.CreateOTPParams{
		UserID:    userID,
		Code:      code,
		ExpiresAt: expires,
		Domain:    pgtype.Text{String: domain, Valid: len(domain) > 0},
	})

	if err != nil {
		return fmt.Errorf("error creating OTP: %v", err)
	}

	return nil
}

func (r *Repository) GetOTP(ctx context.Context, userID int64, domain string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	code, err := r.queries.GetOTP(ctx, database.GetOTPParams{
		UserID: userID,
		Domain: pgtype.Text{String: domain, Valid: len(domain) > 0},
	})

	if err != nil {
		return "", fmt.Errorf("error getting OTP: %v", err)
	}

	return code, nil
}

func (r *Repository) DeleteOTP(ctx context.Context, userID int64, domain string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := r.queries.DeleteOTP(ctx, database.DeleteOTPParams{
		UserID: userID,
		Domain: pgtype.Text{String: domain, Valid: len(domain) > 0},
	})
	if err != nil {
		return fmt.Errorf("error deleting OTP: %v", err)
	}

	return nil
}
