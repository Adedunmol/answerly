package auth

import (
	"context"
	"errors"
	"fmt"
	"github.com/Adedunmol/answerly/api/custom_errors"
	"github.com/Adedunmol/answerly/database"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Store interface {
	CreateUser(ctx context.Context, body *CreateUserBody) (database.User, error)
	FindUserByEmail(ctx context.Context, email string) (database.User, error)
	FindUserByID(ctx context.Context, id int) (database.User, error)
	UpdateUser(ctx context.Context, id int, data UpdateUserBody) error
	FindUserWithRefreshToken(ctx context.Context, refreshToken string) (database.User, error)
	UpdateRefreshToken(ctx context.Context, oldRefreshToken, refreshToken string) error
	DeleteRefreshToken(ctx context.Context, refreshToken string) error
}

type Repository struct {
	queries *database.Queries
	db      *pgxpool.Pool
}

func NewUserStore(queries *database.Queries, db *pgxpool.Pool) *Repository {

	return &Repository{queries: queries, db: db}
}

const UniqueViolation = "23505"

func (r *Repository) CreateUser(ctx context.Context, body *CreateUserBody) (database.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	//executor := database.GetTx(ctx, r.db)
	//q := r.queries.WithTx(executor)
	//

	var provider database.NullAuthProvider

	if body.GoogleID != "" {
		provider = database.NullAuthProvider{
			AuthProvider: database.AuthProviderGoogle,
			Valid:        true,
		}
	} else {
		provider = database.NullAuthProvider{
			AuthProvider: database.AuthProviderEmail,
			Valid:        true,
		}
	}

	data, err := r.queries.CreateUser(ctx, database.CreateUserParams{
		Email:        body.Email,
		Password:     pgtype.Text{String: body.Password, Valid: len(body.Password) > 0},
		Role:         body.Role,
		GoogleID:     pgtype.Text{String: body.GoogleID, Valid: len(body.GoogleID) > 0},
		AuthProvider: provider,
	})

	if err != nil {
		var e *pgconn.PgError
		if errors.As(err, &e) && e.Code == UniqueViolation {
			return database.User{}, custom_errors.ErrConflict
		}
		return database.User{}, fmt.Errorf("error creating user:  %v", err)
	}

	return data, nil
}

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (database.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	//
	//executor := database.GetTx(ctx, r.db)
	//q := r.queries.WithTx(executor)

	data, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return database.User{}, fmt.Errorf("error getting user by email:  %v", err)
	}

	return data, nil
}

func (r *Repository) FindUserByID(ctx context.Context, id int) (database.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	//executor := database.GetTx(ctx, r.db)
	//q := r.queries.WithTx(executor)

	data, err := r.queries.GetUserByID(ctx, int64(id))
	if err != nil {
		return database.User{}, fmt.Errorf("error getting user by id:  %v", err)
	}

	return data, nil
}

func (r *Repository) UpdateUser(ctx context.Context, id int, data UpdateUserBody) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	//executor := database.GetTx(ctx, r.db)
	//q := r.queries.WithTx(executor)

	err := r.queries.UpdateUser(ctx, database.UpdateUserParams{
		EmailVerified: pgtype.Bool{Bool: data.Verified, Valid: true},
		Password:      pgtype.Text{String: data.Password, Valid: len(data.Password) > 0},
		RefreshToken:  pgtype.Text{String: data.RefreshToken, Valid: len(data.RefreshToken) > 0},
		ID:            int64(id),
	})

	if err != nil {
		return fmt.Errorf("error updating user:  %v", err)
	}

	return nil
}

func (r *Repository) UpdateRefreshToken(ctx context.Context, oldRefreshToken, refreshToken string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	//executor := database.GetTx(ctx, r.db)
	//q := r.queries.WithTx(executor)

	err := r.queries.RotateRefreshToken(ctx, database.RotateRefreshTokenParams{
		OldToken: pgtype.Text{String: oldRefreshToken, Valid: true},
		NewToken: pgtype.Text{String: refreshToken, Valid: true},
	})

	if err != nil {
		return fmt.Errorf("error updating refresh token:  %v", err)
	}

	return nil
}

func (r *Repository) DeleteRefreshToken(ctx context.Context, refreshToken string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	//executor := database.GetTx(ctx, r.db)
	//q := r.queries.WithTx(executor)

	err := r.queries.DeleteRefreshToken(ctx, pgtype.Text{String: refreshToken, Valid: len(refreshToken) > 0})
	if err != nil {
		return fmt.Errorf("error deleting refresh token:  %v", err)
	}

	return nil
}

func (r *Repository) FindUserWithRefreshToken(ctx context.Context, refreshToken string) (database.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	user, err := r.queries.GetUserWithRefreshToken(ctx, pgtype.Text{String: refreshToken, Valid: len(refreshToken) > 0})
	if err != nil {
		return database.User{}, fmt.Errorf("error getting user with refresh token:  %v", err)
	}
	return user, nil
}
