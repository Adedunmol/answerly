package custom_errors

import "errors"

var (
	ErrConflict          = errors.New("record already exists")
	ErrNotFound          = errors.New("resource not found")
	ErrUnauthorized      = errors.New("invalid credentials")
	ErrInternalServer    = errors.New("internal server error")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrInvalidOTP        = errors.New("invalid OTP")
)
