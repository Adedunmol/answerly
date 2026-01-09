package wallets

import (
	"github.com/Adedunmol/answerly/database"
	"github.com/shopspring/decimal"
	"time"
)

type CreateWalletBody struct {
	CompanyID int `json:"company_id"`
}

type TopUpWalletBody struct {
	Amount decimal.Decimal `json:"amount" validate:"required"`
}

type Wallet struct {
	ID        int64
	Balance   int64
	CompanyID int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Transaction struct {
	ID        int64
	Amount    int64
	Type      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WalletWithTransactions struct {
	Wallet       database.Wallet        `json:"wallet"`
	Transactions []database.Transaction `json:"transactions"`
}

type WithdrawBody struct {
	Amount        decimal.Decimal `json:"amount" validate:"required"`
	Method        string          `json:"method" validate:"required,oneof=airtime bank_account"`
	AccountNumber string          `json:"account_number"`
	Bank          string          `json:"bank"`
	PhoneNumber   string          `json:"phone_number"`
	Network       string          `json:"network"`
	Save          bool            `json:"save"`
}

type PaymentMethodBody struct {
	UserID        int64
	Type          string
	Provider      string
	AccountName   string
	AccountNumber string
	PhoneNumber   string
}
