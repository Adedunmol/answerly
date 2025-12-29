package profiles

import (
	"context"
	"fmt"
	"github.com/Adedunmol/answerly/database"
	"time"
)

type Store interface {
	CreateProfile(ctx context.Context, userID int64) error
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
