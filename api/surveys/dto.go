package surveys

import "github.com/shopspring/decimal"

// Parameter structs
type CreateSurveyParams struct {
	Title                string
	Description          string
	Category             string
	EstimatedTimeMinutes int
	Reward               decimal.Decimal
	Eligibility          map[string]interface{} // Will be stored as JSONB
	CreatedBy            int64
}

type UpdateSurveyParams struct {
	ID                   int64
	Title                *string
	Description          *string
	Category             *string
	EstimatedTimeMinutes *int
	Reward               *decimal.Decimal
	Eligibility          map[string]interface{}
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
