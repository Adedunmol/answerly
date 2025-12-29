package researchers

type Store interface {
	CreateUser(body *CreateAdminBody) (any, error)
	FindUserByEmail(email string) (any, error)
	FindUserByID(id int) (any, error)
	UpdateUser(id int, data any) (any, error)
	UpdateRefreshToken(oldRefreshToken, refreshToken string) error
	DeleteRefreshToken(refreshToken string) error
}
