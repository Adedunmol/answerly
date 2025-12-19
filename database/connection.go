package database

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"os"
	"time"
)

func ConnectDB(ctx context.Context) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	connectionStr, exists := os.LookupEnv("DATABASE_URL")

	if !exists {
		return nil, errors.New("DATABASE_URL environment variable not set")
	}

	pool, err := pgxpool.New(context.Background(), connectionStr)

	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %v", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("error pinging database: %v", err)
	}

	log.Print("connected to the database")

	return pool, nil
}
