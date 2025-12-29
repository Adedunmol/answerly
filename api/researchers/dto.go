package researchers

type CreateAdminBody struct {
	FirstName       string `json:"first_name" validate:"required"`
	LastName        string `json:"last_name" validate:"required"`
	Password        string `json:"password" validate:"required"`
	ConfirmPassword string `validate:"required,eqfield=Password"`
	Email           string `json:"email" validate:"required,email"`
}
