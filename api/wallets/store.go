package wallets

import (
	"context"
	"fmt"
	"github.com/Adedunmol/answerly/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"time"
)

type Store interface {
	CreateWallet(ctx context.Context, userID int64) (database.Wallet, error)
	GetWallet(ctx context.Context, userID int64) (WalletWithTransactions, error)
	TopUpWallet(ctx context.Context, userID int64, amount decimal.Decimal) (database.Wallet, error)
	ChargeWallet(ctx context.Context, companyID int64, amount decimal.Decimal) (database.Wallet, error)
	CreateTransaction(ctx context.Context, walletID int64, amount decimal.Decimal, txType string) error
	CreatePaymentMethod(ctx context.Context, body PaymentMethodBody) error
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

func (r *Repository) GetWallet(ctx context.Context, userID int64) (WalletWithTransactions, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.queries.GetWalletWithTransactions(ctx, userID)
	if err != nil {
		return WalletWithTransactions{}, fmt.Errorf("error getting wallet with transactions: %v", err)
	}

	if len(rows) == 0 {
		return WalletWithTransactions{}, fmt.Errorf("wallet not found")
	}

	// First row contains wallet data
	firstRow := rows[0]
	result := WalletWithTransactions{
		Wallet: database.Wallet{
			ID:        firstRow.ID,
			Balance:   firstRow.Balance,
			UserID:    firstRow.UserID,
			CreatedAt: firstRow.CreatedAt,
			UpdatedAt: firstRow.UpdatedAt,
		},
		Transactions: make([]database.Transaction, 0),
	}

	// Collect all transactions
	for _, row := range rows {
		// Only add transaction if it exists (LEFT JOIN might return NULL)
		if row.TransactionID.Valid {
			transaction := database.Transaction{
				ID:            row.TransactionID.Int64,
				Amount:        row.TransactionAmount,
				BalanceBefore: row.TransactionBalanceBefore,
				BalanceAfter:  row.TransactionBalanceAfter,
				Reference:     row.TransactionReference,
				Status:        row.TransactionStatus,
				WalletID:      row.TransactionWalletID,
				CreatedAt:     row.TransactionCreatedAt,
				UpdatedAt:     row.TransactionUpdatedAt,
			}
			result.Transactions = append(result.Transactions, transaction)
		}
	}

	return result, nil
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

func (r *Repository) CreateTransaction(ctx context.Context, walletID int64, amount decimal.Decimal, txType string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	wallet, err := r.queries.GetWallet(ctx, walletID)
	if err != nil {
		return fmt.Errorf("error getting wallet: %v", err)
	}

	var walletBalance decimal.Decimal
	err = walletBalance.Scan(wallet.Balance)
	if err != nil {
		return fmt.Errorf("error scanning wallet balance: %v", err)
	}

	balanceAfter := walletBalance.Sub(amount)

	balanceAfterCast := pgtype.Numeric{}
	err = balanceAfterCast.Scan(balanceAfter.String())
	if err != nil {
		return fmt.Errorf("error while scanning balance after: %v", err)
	}

	amountCast := pgtype.Numeric{}
	err = amountCast.Scan(amount.String())
	if err != nil {
		return fmt.Errorf("error while scanning amount: %v", err)
	}

	err = r.queries.CreateTransaction(ctx, database.CreateTransactionParams{
		Amount:        amountCast,
		BalanceBefore: wallet.Balance,
		BalanceAfter:  balanceAfterCast,
		Reference:     pgtype.Text{String: uuid.New().String(), Valid: true},
		Status:        pgtype.Text{String: "pending", Valid: true},
		WalletID:      pgtype.Int8{Int64: walletID, Valid: true},
		Type:          pgtype.Text{String: txType, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("error creating transaction: %v", err)
	}
	return nil
}

func (r *Repository) CreatePaymentMethod(ctx context.Context, body PaymentMethodBody) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := r.queries.CreatePaymentMethod(ctx, database.CreatePaymentMethodParams{
		UserID:        body.UserID,
		Type:          body.Type,
		Provider:      pgtype.Text{String: body.Provider, Valid: len(body.Provider) > 0},
		AccountName:   pgtype.Text{String: body.AccountName, Valid: len(body.AccountName) > 0},
		AccountNumber: pgtype.Text{String: body.AccountNumber, Valid: len(body.AccountNumber) > 0},
		PhoneNumber:   pgtype.Text{String: body.PhoneNumber, Valid: len(body.PhoneNumber) > 0},
	})
	if err != nil {
		return fmt.Errorf("error creating payment method: %v", err)
	}

	return nil
}
