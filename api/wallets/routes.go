package wallets

import (
	"github.com/Adedunmol/answerly/api/middlewares"
	"github.com/Adedunmol/answerly/api/tokens"
	"github.com/Adedunmol/answerly/database"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupRoutes(r *chi.Mux, db *pgxpool.Pool, queries *database.Queries) {

	jobRouter := chi.NewRouter()
	tokenService := tokens.NewTokenService()

	handler := Handler{
		Store: NewWalletStore(db, queries),
	}

	jobRouter.Use(middlewares.AuthMiddleware(tokenService))

	jobRouter.Get("/", handler.GetWalletHandler)
	jobRouter.Patch("/", handler.TopUpWalletHandler)

	r.Mount("/wallets", jobRouter)
}
