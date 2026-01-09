package auth

import (
	"github.com/Adedunmol/answerly/api/middlewares"
	"github.com/Adedunmol/answerly/api/otp"
	"github.com/Adedunmol/answerly/api/profiles"
	"github.com/Adedunmol/answerly/api/tokens"
	"github.com/Adedunmol/answerly/api/wallets"
	"github.com/Adedunmol/answerly/database"
	"github.com/Adedunmol/answerly/queue"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupRoutes(r *chi.Mux, queue queue.Queue, db *pgxpool.Pool, queries *database.Queries) {

	authRouter := chi.NewRouter()

	store := NewUserStore(queries, db)
	tokenService := tokens.NewTokenService()
	otpStore := otp.NewOTPStore(queries, tokenService)
	walletService := wallets.NewWalletStore(db, queries)
	profileService := profiles.NewProfileStore(queries)

	handler := Handler{
		Store:        store,
		Queue:        queue,
		OTPStore:     otpStore,
		Token:        tokenService,
		WalletStore:  walletService,
		ProfileStore: profileService,
	}

	authRouter.Route("/auth", func(authRouter chi.Router) {
		authRouter.Post("/register", handler.CreateUserHandler)
		authRouter.Post("/login", handler.LoginUserHandler)
		authRouter.Post("/logout", handler.LogoutUserHandler)
		authRouter.Post("/verify", handler.VerifyOTPHandler)
		authRouter.Get("/refresh-token", handler.RefreshTokenHandler)
		authRouter.Post("/request-code", handler.RequestCodeHandler)
		authRouter.Post("/forgot-password", handler.ForgotPasswordHandler)
		authRouter.Post("/forgot-password-request", handler.ForgotPasswordRequestHandler)
		authRouter.With(middlewares.AuthMiddleware(tokenService)).Post("/reset-password", handler.ResetPasswordHandler)
	})

	r.Mount("/users", authRouter)

	return
}
