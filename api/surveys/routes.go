package surveys

import (
	"github.com/Adedunmol/answerly/api/middlewares"
	"github.com/Adedunmol/answerly/api/tokens"
	"github.com/Adedunmol/answerly/database"
	"github.com/Adedunmol/answerly/queue"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupRoutes(r *chi.Mux, queue queue.Queue, db *pgxpool.Pool, queries *database.Queries) {

	surveysRouter := chi.NewRouter()

	store := NewSurveyStore(queries)
	tokenService := tokens.NewTokenService()

	handler := Handler{
		Store: store,
	}

	// Public routes (no authentication required)
	surveysRouter.Group(func(r chi.Router) {
		r.Get("/", handler.ListSurveysHandler)
		r.Get("/{surveyID}", handler.GetSurveyHandler)
		r.Get("/{surveyID}/details", handler.GetSurveyWithDetailsHandler)
		r.Get("/{surveyID}/questions", handler.GetQuestionsBySurveyHandler)
		r.Get("/questions/{questionID}/options", handler.GetOptionsByQuestionHandler)
	})

	// Protected routes (authentication required)
	surveysRouter.Group(func(r chi.Router) {
		r.Use(middlewares.AuthMiddleware(tokenService))

		// Survey management (for survey creators/admins)
		r.Post("/", handler.CreateSurveyHandler)
		r.Patch("/{surveyID}", handler.UpdateSurveyHandler)
		r.Delete("/{surveyID}", handler.DeleteSurveyHandler)

		// Question management
		r.Post("/{surveyID}/questions", handler.CreateQuestionHandler)
		r.Patch("/questions/{questionID}", handler.UpdateQuestionHandler)
		r.Delete("/questions/{questionID}", handler.DeleteQuestionHandler)

		// Question options management
		r.Post("/questions/{questionID}/options", handler.CreateQuestionOptionHandler)
		r.Patch("/options/{optionID}", handler.UpdateQuestionOptionHandler)
		r.Delete("/options/{optionID}", handler.DeleteQuestionOptionHandler)

		// User survey responses
		r.Post("/{surveyID}/start", handler.StartSurveyHandler)
		r.Get("/{surveyID}/progress", handler.GetUserSurveyProgressHandler)
		r.Post("/{surveyID}/complete", handler.CompleteSurveyHandler)
		r.Get("/my-surveys", handler.ListUserSurveysHandler)

		// Answer management
		r.Post("/answers", handler.SaveAnswerHandler)
		r.Post("/answers/bulk", handler.SaveAnswersHandler)
		r.Get("/responses/{userSurveyResponseID}/answers", handler.GetAnswersByUserSurveyHandler)
		r.Patch("/answers/{answerID}", handler.UpdateAnswerHandler)
	})

	r.Mount("/surveys", surveysRouter)

	return
}
