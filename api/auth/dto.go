package auth

import "time"

type User struct {
	ID           int        `json:"id"`
	FirstName    string     `json:"first_name"`
	LastName     string     `json:"last_name"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	Password     string     `json:"-"`
	DateOfBirth  *time.Time `json:"date_of_birth"`
	Verified     bool       `json:"verified"`
	RefreshToken string     `json:"-"`
}

type CreateUserBody struct {
	Password        string `json:"password" validate:"required"`
	ConfirmPassword string `json:"password_confirmation" validate:"required,eqfield=Password"`
	Email           string `json:"email" validate:"required,email"`
}

type LoginUserBody struct {
	Password string `json:"password" validate:"required"`
	Email    string `json:"email" validate:"required"`
}

type CreateUserResponse struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Email     string `json:"email"`
}

type OTP struct {
	ID        int        `json:"id"`
	Email     string     `json:"email"`
	OTP       string     `json:"otp"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt *time.Time `json:"created_at"`
}

type UpdateUserBody struct {
	Verified     bool   `json:"verified"`
	Password     string `json:"password"`
	RefreshToken string `json:"refresh_token"`
}

type VerifyOTPBody struct {
	Email string `json:"email" validate:"required,email"`
	Code  string `json:"code" validate:"required"`
}

type RequestOTPBody struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPasswordBody struct {
	OldPassword        string `json:"old_password" validate:"required"`
	NewPassword        string `json:"new_password" validate:"required"`
	NewPasswordConfirm string `json:"new_password_confirm" validate:"required"`
}

type ForgotPasswordBody struct {
	Email              string `json:"email" validate:"required,email"`
	Code               string `json:"code" validate:"required"`
	NewPassword        string `json:"new_password" validate:"required"`
	NewPasswordConfirm string `json:"new_password_confirm" validate:"required,eqfield=NewPassword"`
}
