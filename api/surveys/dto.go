package surveys

import (
	"github.com/Adedunmol/answerly/database"
	"github.com/shopspring/decimal"
)

// Parameter structs
type CreateSurveyParams struct {
	Title                string          `json:"title" validate:"required"`
	Description          string          `json:"description" validate:"required"`
	Category             string          `json:"category" validate:"required"`
	EstimatedTimeMinutes int             `json:"estimated_time_minutes" validate:"required"`
	Reward               decimal.Decimal `json:"reward" validate:"required"`
	Eligibility          string          // Will be stored as JSONB
	CreatedBy            int64           `json:"created_by"`
}

type UpdateSurveyParams struct {
	ID                   int64
	Title                *string
	Description          *string
	Category             *string
	EstimatedTimeMinutes *int
	Reward               *decimal.Decimal
	Eligibility          *string
	Status               *string
}

type ListSurveysParams struct {
	Category string
	Status   string
	Limit    int
	Offset   int
}

type CreateQuestionParams struct {
	SurveyID     int64
	QuestionText string
	QuestionType string
	IsRequired   bool
	OrderIndex   int
}

type UpdateQuestionParams struct {
	ID           int64
	QuestionText *string
	QuestionType *string
	IsRequired   *bool
	OrderIndex   *int
}

type CreateQuestionOptionParams struct {
	QuestionID int64
	OptionText string
	OrderIndex int
}

type UpdateQuestionOptionParams struct {
	ID         int64
	OptionText *string
	OrderIndex *int
}

type SaveAnswerParams struct {
	UserSurveyResponseID int64
	QuestionID           int64
	AnswerText           *string
	SelectedOptionIDs    []int64
}

type UpdateAnswerParams struct {
	ID                int64
	AnswerText        *string
	SelectedOptionIDs []int64
}

// Response structs
type SurveyDetail struct {
	Survey    database.Survey
	Questions []QuestionWithOptions
}

type QuestionWithOptions struct {
	Question database.SurveyQuestion
	Options  []database.QuestionOption
}

type SurveyProgress struct {
	UserSurveyResponse  database.UserSurveyResponse
	TotalQuestions      int
	AnsweredQuestions   int
	PercentageCompleted decimal.Decimal
}

type UserSurveyWithDetails struct {
	Survey              database.Survey
	UserSurveyResponse  database.UserSurveyResponse
	PercentageCompleted decimal.Decimal
}

type SaveAnswersParams struct {
	Answers []SaveAnswerParams `json:"answers"`
}
