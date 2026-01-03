package profiles

import (
	"context"
	"fmt"
	"github.com/Adedunmol/answerly/database"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

type Store interface {
	CreateProfile(ctx context.Context, userID int64) error
	GetProfile(ctx context.Context, userID int64) (database.Profile, error)
	UpdateProfile(ctx context.Context, userID int64, profile UpdateProfileBody) (database.Profile, error)
}

type Repository struct {
	queries *database.Queries
}

func NewProfileStore(queries *database.Queries) *Repository {

	return &Repository{queries: queries}
}

func (r *Repository) CreateProfile(ctx context.Context, userID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := r.queries.CreateProfile(ctx, userID)
	if err != nil {
		return fmt.Errorf("error creating profile: %v", err)
	}

	return nil
}

func (r *Repository) GetProfile(ctx context.Context, userID int64) (database.Profile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	profile, err := r.queries.GetProfile(ctx, userID)
	if err != nil {
		return database.Profile{}, fmt.Errorf("error getting profile: %v", err)
	}

	return profile, nil
}

func (r *Repository) UpdateProfile(ctx context.Context, userID int64, profile UpdateProfileBody) (database.Profile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var gender database.NullGender

	if profile.Gender != "" {
		gender = database.NullGender{
			Gender: database.Gender(profile.Gender),
			Valid:  true,
		}
	}

	data, err := r.queries.UpdateProfile(ctx, database.UpdateProfileParams{
		FirstName:   pgtype.Text{String: profile.FirstName, Valid: len(profile.FirstName) > 0},
		LastName:    pgtype.Text{String: profile.LastName, Valid: len(profile.LastName) > 0},
		DateOfBirth: pgtype.Date{Time: profile.DateOfBirth, Valid: true},
		Gender:      gender,
		University:  pgtype.Text{String: profile.University, Valid: len(profile.University) > 0},
		Location:    pgtype.Text{String: profile.Location, Valid: len(profile.Location) > 0},
		UserID:      userID,
	})

	if err != nil {
		return database.Profile{}, fmt.Errorf("error updating profile: %v", err)
	}

	return data, nil
}
