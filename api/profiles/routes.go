package profiles

import (
	"github.com/Adedunmol/answerly/api/middlewares"
	"github.com/Adedunmol/answerly/api/tokens"
	"github.com/Adedunmol/answerly/database"
	"github.com/Adedunmol/answerly/queue"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupRoutes(r *chi.Mux, queue queue.Queue, db *pgxpool.Pool, queries *database.Queries) {

	profilesRouter := chi.NewRouter()

	store := NewProfileStore(queries)
	tokenService := tokens.NewTokenService()

	handler := Handler{
		Store: store,
	}

	profilesRouter.Use(middlewares.AuthMiddleware(tokenService))

	profilesRouter.Get("/", handler.GetProfileHandler)
	profilesRouter.Patch("/", handler.UpdateProfileHandler)

	r.Mount("/profiles", profilesRouter)

	return
}
