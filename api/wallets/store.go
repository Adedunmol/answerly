package wallets

import (
	"context"
	"fmt"
	"github.com/Adedunmol/answerly/database"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"time"
)

type Store interface {
	CreateWallet(ctx context.Context, userID int64) (database.Wallet, error)
	GetWallet(ctx context.Context, userID int64) (database.Wallet, error)
	TopUpWallet(ctx context.Context, userID int64, amount decimal.Decimal) (database.Wallet, error)
	ChargeWallet(ctx context.Context, companyID int64, amount decimal.Decimal) (database.Wallet, error)
}

const UniqueViolationCode = "23505"

type Repository struct {
	queries *database.Queries
	db      *pgxpool.Pool
}

func NewWalletStore(pool *pgxpool.Pool, queries *database.Queries) *Repository {

	return &Repository{queries: queries, db: pool}
}

func (r *Repository) CreateWallet(ctx context.Context, userID int64) (database.Wallet, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	balance := pgtype.Numeric{}
	err := balance.Scan("0.0")
	if err != nil {
		return database.Wallet{}, fmt.Errorf("error while scanning amount: %v", err)
	}

	wallet, err := r.queries.CreateWallet(ctx, database.CreateWalletParams{
		Balance: balance,
		UserID:  userID,
	})
	if err != nil {
		return database.Wallet{}, fmt.Errorf("error creating wallet: %v", err)
	}

	return wallet, nil
}

func (r *Repository) GetWallet(ctx context.Context, userID int64) (database.Wallet, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wallet, err := r.queries.GetWallet(ctx, userID)
	if err != nil {
		return database.Wallet{}, fmt.Errorf("error getting wallet: %v", err)
	}

	return wallet, nil
}

func (r *Repository) TopUpWallet(ctx context.Context, userID int64, amount decimal.Decimal) (database.Wallet, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	amountCast := pgtype.Numeric{}
	err := amountCast.Scan(amount.String())
	if err != nil {
		return database.Wallet{}, fmt.Errorf("error while scanning amount: %v", err)
	}

	wallet, err := r.queries.TopUpWallet(ctx, database.TopUpWalletParams{
		UserID: userID,
		Amount: amountCast,
	})
	if err != nil {
		return database.Wallet{}, fmt.Errorf("error topping up wallet: %v", err)
	}

	return wallet, nil
}

func (r *Repository) ChargeWallet(ctx context.Context, userID int64, amount decimal.Decimal) (database.Wallet, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	amountCast := pgtype.Numeric{}
	err := amountCast.Scan(amount.String())
	if err != nil {
		return database.Wallet{}, fmt.Errorf("error while scanning amount: %v", err)
	}

	wallet, err := r.queries.ChargeWallet(ctx, database.ChargeWalletParams{
		Amount: amountCast,
		UserID: userID,
	})
	if err != nil {
		return database.Wallet{}, fmt.Errorf("error charging wallet: %v", err)
	}

	return wallet, nil
}
