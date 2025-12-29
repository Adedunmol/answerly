package database

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Transactor defines the interface for managing transactions
type Transactor interface {
	// WithTransaction executes fn within a transaction
	WithTransaction(ctx context.Context, fn func(context.Context) error) error
}

// DBTransactor implements Transactor using *sql.DB
type DBTransactor struct {
	db *pgxpool.Pool
}

func NewDBTransactor(db *pgxpool.Pool) *DBTransactor {
	return &DBTransactor{db: db}
}

// WithTransaction executes the given function within a transaction
func (t *DBTransactor) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	tx, err := t.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	// Store transaction in context
	ctx = context.WithValue(ctx, txKey{}, tx)

	// Execute function
	if err := fn(ctx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
		}
		return err
	}

	return tx.Commit(ctx)
}

// txKey is used to store transaction in context
type txKey struct{}

// GetTx retrieves transaction from context, or starts new transaction if no transaction
func GetTx(ctx context.Context, db *pgxpool.Pool) pgx.Tx {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		panic(err)
	}
	return tx
}
