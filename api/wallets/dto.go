package wallets

import "github.com/shopspring/decimal"

type CreateWalletBody struct {
	CompanyID int `json:"company_id"`
}

type TopUpWalletBody struct {
	Amount decimal.Decimal `json:"amount" validate:"required"`
}
