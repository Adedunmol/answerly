package surveys

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Adedunmol/answerly/database"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
	"time"
)

type Store interface {
	// Survey Management
	CreateSurvey(ctx context.Context, params CreateSurveyParams) (database.Survey, error)
	GetSurvey(ctx context.Context, surveyID int64) (database.Survey, error)
	GetSurveyWithDetails(ctx context.Context, surveyID int64) (SurveyDetail, error)
	ListSurveys(ctx context.Context, params ListSurveysParams) ([]database.Survey, error)
	UpdateSurvey(ctx context.Context, params UpdateSurveyParams) (database.Survey, error)
	DeleteSurvey(ctx context.Context, surveyID int64) error

	// Question Management
	CreateQuestion(ctx context.Context, params CreateQuestionParams) (database.SurveyQuestion, error)
	GetQuestion(ctx context.Context, questionID int64) (database.SurveyQuestion, error)
	GetQuestionsBySurveyID(ctx context.Context, surveyID int64) ([]database.SurveyQuestion, error)
	UpdateQuestion(ctx context.Context, params UpdateQuestionParams) (database.SurveyQuestion, error)
	DeleteQuestion(ctx context.Context, questionID int64) error

	// Question Options Management
	CreateQuestionOption(ctx context.Context, params CreateQuestionOptionParams) (database.QuestionOption, error)
	GetOptionsByQuestionID(ctx context.Context, questionID int64) ([]database.QuestionOption, error)
	UpdateQuestionOption(ctx context.Context, params UpdateQuestionOptionParams) (database.QuestionOption, error)
	DeleteQuestionOption(ctx context.Context, optionID int64) error

	// User Survey Response Management
	StartSurvey(ctx context.Context, userID, surveyID int64) (database.UserSurveyResponse, error)
	GetUserSurveyResponse(ctx context.Context, userID, surveyID int64) (database.UserSurveyResponse, error)
	GetUserSurveyProgress(ctx context.Context, userID, surveyID int64) (SurveyProgress, error)
	ListUserSurveys(ctx context.Context, userID int64, status string) ([]UserSurveyWithDetails, error)
	CompleteSurvey(ctx context.Context, userID, surveyID int64) (database.UserSurveyResponse, error)

	// Answer Management
	SaveAnswer(ctx context.Context, params SaveAnswerParams) (database.AnswerResponse, error)
	SaveAnswers(ctx context.Context, params []SaveAnswerParams) ([]database.AnswerResponse, error)
	GetAnswer(ctx context.Context, userSurveyResponseID, questionID int64) (database.AnswerResponse, error)
	GetAnswersByUserSurveyResponse(ctx context.Context, userSurveyResponseID int64) ([]database.AnswerResponse, error)
	UpdateAnswer(ctx context.Context, params UpdateAnswerParams) (database.AnswerResponse, error)
}

const UniqueViolationCode = "23505"

type Repository struct {
	queries *database.Queries
}

func NewSurveyStore(queries *database.Queries) *Repository {
	return &Repository{queries: queries}
}

// ==================== Survey Management ====================

func (r *Repository) CreateSurvey(ctx context.Context, params CreateSurveyParams) (database.Survey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reward := pgtype.Numeric{}
	err := reward.Scan(params.Reward.String())
	if err != nil {
		return database.Survey{}, fmt.Errorf("error scanning reward: %v", err)
	}

	eligibilityJSON, err := json.Marshal(params.Eligibility)
	if err != nil {
		return database.Survey{}, fmt.Errorf("error marshaling eligibility: %v", err)
	}

	survey, err := r.queries.CreateSurvey(ctx, database.CreateSurveyParams{
		Title:                params.Title,
		Description:          pgtype.Text{String: params.Description, Valid: params.Description != ""},
		Category:             params.Category,
		EstimatedTimeMinutes: int32(params.EstimatedTimeMinutes),
		Reward:               reward,
		Eligibility:          eligibilityJSON,
		CreatedBy:            params.CreatedBy,
	})
	if err != nil {
		return database.Survey{}, fmt.Errorf("error creating survey: %v", err)
	}

	return survey, nil
}

func (r *Repository) GetSurvey(ctx context.Context, surveyID int64) (database.Survey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	survey, err := r.queries.GetSurvey(ctx, surveyID)
	if err != nil {
		return database.Survey{}, fmt.Errorf("error getting survey: %v", err)
	}

	return survey, nil
}

func (r *Repository) GetSurveyWithDetails(ctx context.Context, surveyID int64) (SurveyDetail, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := r.queries.GetSurveyDetailWithQuestionsAndOptions(ctx, surveyID)
	if err != nil {
		return SurveyDetail{}, fmt.Errorf("error getting survey details: %v", err)
	}

	if len(rows) == 0 {
		return SurveyDetail{}, sql.ErrNoRows
	}

	// Build the survey detail structure
	surveyDetail := SurveyDetail{
		Survey: database.Survey{
			ID:                   rows[0].SurveyID,
			Title:                rows[0].SurveyTitle,
			Description:          rows[0].SurveyDescription,
			Category:             rows[0].SurveyCategory,
			EstimatedTimeMinutes: rows[0].EstimatedTimeMinutes,
			Reward:               rows[0].Reward,
			Eligibility:          rows[0].Eligibility,
			Status:               rows[0].SurveyStatus,
			CreatedBy:            rows[0].CreatedBy,
			CreatedAt:            rows[0].SurveyCreatedAt,
		},
		Questions: []QuestionWithOptions{},
	}

	// Group questions and options
	questionMap := make(map[int64]*QuestionWithOptions)
	for _, row := range rows {
		if !row.QuestionID.Valid {
			continue
		}

		questionID := row.QuestionID.Int64
		if _, exists := questionMap[questionID]; !exists {
			questionMap[questionID] = &QuestionWithOptions{
				Question: database.SurveyQuestion{
					ID:           questionID,
					SurveyID:     surveyID,
					QuestionText: row.QuestionText.String,
					QuestionType: row.QuestionType.String,
					IsRequired:   row.IsRequired,
					OrderIndex:   row.QuestionOrder.Int32,
				},
				Options: []database.QuestionOption{},
			}
		}

		if row.OptionID.Valid {
			questionMap[questionID].Options = append(questionMap[questionID].Options, database.QuestionOption{
				ID:         row.OptionID.Int64,
				QuestionID: questionID,
				OptionText: row.OptionText.String,
				OrderIndex: row.OptionOrder.Int32,
			})
		}
	}

	// Convert map to slice and sort by question order
	for _, question := range questionMap {
		surveyDetail.Questions = append(surveyDetail.Questions, *question)
	}

	return surveyDetail, nil
}

func (r *Repository) ListSurveys(ctx context.Context, params ListSurveysParams) ([]database.Survey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	category := pgtype.Text{}
	if params.Category != "" {
		category = pgtype.Text{String: params.Category, Valid: true}
	}

	status := pgtype.Text{}
	if params.Status != "" {
		status = pgtype.Text{String: params.Status, Valid: true}
	}

	surveys, err := r.queries.ListSurveys(ctx, database.ListSurveysParams{
		Category: category,
		Status:   status,
		Limit:    int32(params.Limit),
		Offset:   int32(params.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("error listing surveys: %v", err)
	}

	return surveys, nil
}

func (r *Repository) UpdateSurvey(ctx context.Context, params UpdateSurveyParams) (database.Survey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updateParams := database.UpdateSurveyParams{
		ID: params.ID,
	}

	if params.Title != nil {
		updateParams.Title = pgtype.Text{String: *params.Title, Valid: true}
	}

	if params.Description != nil {
		updateParams.Description = pgtype.Text{String: *params.Description, Valid: true}
	}

	if params.Category != nil {
		updateParams.Category = pgtype.Text{String: *params.Category, Valid: true}
	}

	if params.EstimatedTimeMinutes != nil {
		updateParams.EstimatedTimeMinutes = pgtype.Int4{Int32: int32(*params.EstimatedTimeMinutes), Valid: true}
	}

	if params.Reward != nil {
		reward := pgtype.Numeric{}
		err := reward.Scan(params.Reward.String())
		if err != nil {
			return database.Survey{}, fmt.Errorf("error scanning reward: %v", err)
		}
		updateParams.Reward = reward
	}

	if params.Eligibility != nil {
		eligibilityJSON, err := json.Marshal(params.Eligibility)
		if err != nil {
			return database.Survey{}, fmt.Errorf("error marshaling eligibility: %v", err)
		}
		updateParams.Eligibility = eligibilityJSON
	}

	if params.Status != nil {
		updateParams.Status = pgtype.Text{String: *params.Status, Valid: true}
	}

	survey, err := r.queries.UpdateSurvey(ctx, updateParams)
	if err != nil {
		return database.Survey{}, fmt.Errorf("error updating survey: %v", err)
	}

	return survey, nil
}

func (r *Repository) DeleteSurvey(ctx context.Context, surveyID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := r.queries.DeleteSurvey(ctx, surveyID)
	if err != nil {
		return fmt.Errorf("error deleting survey: %v", err)
	}

	return nil
}

// ==================== Question Management ====================

func (r *Repository) CreateQuestion(ctx context.Context, params CreateQuestionParams) (database.SurveyQuestion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	question, err := r.queries.CreateQuestion(ctx, database.CreateQuestionParams{
		SurveyID:     params.SurveyID,
		QuestionText: params.QuestionText,
		QuestionType: params.QuestionType,
		IsRequired:   pgtype.Bool{Bool: params.IsRequired},
		OrderIndex:   int32(params.OrderIndex),
	})
	if err != nil {
		return database.SurveyQuestion{}, fmt.Errorf("error creating question: %v", err)
	}

	return question, nil
}

func (r *Repository) GetQuestion(ctx context.Context, questionID int64) (database.SurveyQuestion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	question, err := r.queries.GetQuestion(ctx, questionID)
	if err != nil {
		return database.SurveyQuestion{}, fmt.Errorf("error getting question: %v", err)
	}

	return question, nil
}

func (r *Repository) GetQuestionsBySurveyID(ctx context.Context, surveyID int64) ([]database.SurveyQuestion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	questions, err := r.queries.GetQuestionsBySurveyID(ctx, surveyID)
	if err != nil {
		return nil, fmt.Errorf("error getting questions: %v", err)
	}

	return questions, nil
}

func (r *Repository) UpdateQuestion(ctx context.Context, params UpdateQuestionParams) (database.SurveyQuestion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updateParams := database.UpdateQuestionParams{
		ID: params.ID,
	}

	if params.QuestionText != nil {
		updateParams.QuestionText = pgtype.Text{String: *params.QuestionText, Valid: true}
	}

	if params.QuestionType != nil {
		updateParams.QuestionType = pgtype.Text{String: *params.QuestionType, Valid: true}
	}

	if params.IsRequired != nil {
		updateParams.IsRequired = pgtype.Bool{Bool: *params.IsRequired, Valid: true}
	}

	if params.OrderIndex != nil {
		updateParams.OrderIndex = pgtype.Int4{Int32: int32(*params.OrderIndex), Valid: true}
	}

	question, err := r.queries.UpdateQuestion(ctx, updateParams)
	if err != nil {
		return database.SurveyQuestion{}, fmt.Errorf("error updating question: %v", err)
	}

	return question, nil
}

func (r *Repository) DeleteQuestion(ctx context.Context, questionID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := r.queries.DeleteQuestion(ctx, questionID)
	if err != nil {
		return fmt.Errorf("error deleting question: %v", err)
	}

	return nil
}

// ==================== Question Options Management ====================

func (r *Repository) CreateQuestionOption(ctx context.Context, params CreateQuestionOptionParams) (database.QuestionOption, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	option, err := r.queries.CreateQuestionOption(ctx, database.CreateQuestionOptionParams{
		QuestionID: params.QuestionID,
		OptionText: params.OptionText,
		OrderIndex: int32(params.OrderIndex),
	})
	if err != nil {
		return database.QuestionOption{}, fmt.Errorf("error creating question option: %v", err)
	}

	return option, nil
}

func (r *Repository) GetOptionsByQuestionID(ctx context.Context, questionID int64) ([]database.QuestionOption, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	options, err := r.queries.GetOptionsByQuestionID(ctx, questionID)
	if err != nil {
		return nil, fmt.Errorf("error getting options: %v", err)
	}

	return options, nil
}

func (r *Repository) UpdateQuestionOption(ctx context.Context, params UpdateQuestionOptionParams) (database.QuestionOption, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updateParams := database.UpdateQuestionOptionParams{
		ID: params.ID,
	}

	if params.OptionText != nil {
		updateParams.OptionText = pgtype.Text{String: *params.OptionText, Valid: true}
	}

	if params.OrderIndex != nil {
		updateParams.OrderIndex = pgtype.Int4{Int32: int32(*params.OrderIndex), Valid: true}
	}

	option, err := r.queries.UpdateQuestionOption(ctx, updateParams)
	if err != nil {
		return database.QuestionOption{}, fmt.Errorf("error updating question option: %v", err)
	}

	return option, nil
}

func (r *Repository) DeleteQuestionOption(ctx context.Context, optionID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := r.queries.DeleteQuestionOption(ctx, optionID)
	if err != nil {
		return fmt.Errorf("error deleting question option: %v", err)
	}

	return nil
}

// ==================== User Survey Response Management ====================

func (r *Repository) StartSurvey(ctx context.Context, userID, surveyID int64) (database.UserSurveyResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := r.queries.StartSurvey(ctx, database.StartSurveyParams{
		UserID:   userID,
		SurveyID: surveyID,
	})
	if err != nil {
		return database.UserSurveyResponse{}, fmt.Errorf("error starting survey: %v", err)
	}

	return response, nil
}

func (r *Repository) GetUserSurveyResponse(ctx context.Context, userID, surveyID int64) (database.UserSurveyResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := r.queries.GetUserSurveyResponse(ctx, database.GetUserSurveyResponseParams{
		UserID:   userID,
		SurveyID: surveyID,
	})
	if err != nil {
		return database.UserSurveyResponse{}, fmt.Errorf("error getting user survey response: %v", err)
	}

	return response, nil
}

func (r *Repository) GetUserSurveyProgress(ctx context.Context, userID, surveyID int64) (SurveyProgress, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	progress, err := r.queries.GetUserSurveyProgress(ctx, database.GetUserSurveyProgressParams{
		UserID:   userID,
		SurveyID: surveyID,
	})
	if err != nil {
		return SurveyProgress{}, fmt.Errorf("error getting survey progress: %v", err)
	}

	var percentage decimal.Decimal
	val, _ := progress.PercentageCompleted.Float64Value()
	err = percentage.Scan(val)
	if err != nil {
		return SurveyProgress{}, err
	}

	return SurveyProgress{
		UserSurveyResponse: database.UserSurveyResponse{
			ID:                  progress.ID,
			UserID:              progress.UserID,
			SurveyID:            progress.SurveyID,
			Status:              progress.Status,
			PercentageCompleted: progress.PercentageCompleted,
			StartedAt:           progress.StartedAt,
			CompletedAt:         progress.CompletedAt,
			UpdatedAt:           progress.UpdatedAt,
		},
		TotalQuestions:      int(progress.TotalQuestions),
		AnsweredQuestions:   int(progress.AnsweredQuestions),
		PercentageCompleted: percentage,
	}, nil
}

func (r *Repository) ListUserSurveys(ctx context.Context, userID int64, status string) ([]UserSurveyWithDetails, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	statusParam := pgtype.Text{}
	if status != "" {
		statusParam = pgtype.Text{String: status, Valid: true}
	}

	rows, err := r.queries.ListUserSurveys(ctx, database.ListUserSurveysParams{
		UserID: userID,
		Status: statusParam,
	})
	if err != nil {
		return nil, fmt.Errorf("error listing user surveys: %v", err)
	}

	var surveys []UserSurveyWithDetails
	for _, row := range rows {
		var percentage decimal.Decimal
		var percentageFloat float64
		if err := row.PercentageCompleted.Scan(&percentageFloat); err == nil {
			percentage = decimal.NewFromFloat(percentageFloat)
		}

		surveys = append(surveys, UserSurveyWithDetails{
			Survey: database.Survey{
				ID:                   row.ID,
				Title:                row.Title,
				Description:          row.Description,
				Category:             row.Category,
				EstimatedTimeMinutes: row.EstimatedTimeMinutes,
				Reward:               row.Reward,
				Eligibility:          row.Eligibility,
				Status:               row.Status,
				CreatedBy:            row.CreatedBy,
				CreatedAt:            row.CreatedAt,
				UpdatedAt:            row.UpdatedAt,
			},
			UserSurveyResponse: database.UserSurveyResponse{
				ID:                  row.UserSurveyResponseID,
				UserID:              userID,
				SurveyID:            row.ID,
				Status:              row.ResponseStatus,
				PercentageCompleted: row.PercentageCompleted,
				StartedAt:           row.StartedAt,
				CompletedAt:         row.CompletedAt,
				UpdatedAt:           row.ResponseUpdatedAt,
			},
			PercentageCompleted: percentage,
		})
	}

	return surveys, nil
}

func (r *Repository) CompleteSurvey(ctx context.Context, userID, surveyID int64) (database.UserSurveyResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := r.queries.CompleteSurvey(ctx, database.CompleteSurveyParams{
		UserID:   userID,
		SurveyID: surveyID,
	})
	if err != nil {
		return database.UserSurveyResponse{}, fmt.Errorf("error completing survey: %v", err)
	}

	return response, nil
}

// ==================== Answer Management ====================

func (r *Repository) SaveAnswer(ctx context.Context, params SaveAnswerParams) (database.AnswerResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	answerText := pgtype.Text{}
	if params.AnswerText != nil {
		answerText = pgtype.Text{String: *params.AnswerText, Valid: true}
	}

	answer, err := r.queries.SaveAnswer(ctx, database.SaveAnswerParams{
		UserSurveyResponseID: params.UserSurveyResponseID,
		QuestionID:           params.QuestionID,
		AnswerText:           answerText,
		SelectedOptionIds:    params.SelectedOptionIDs,
	})
	if err != nil {
		return database.AnswerResponse{}, fmt.Errorf("error saving answer: %v", err)
	}

	// Update survey progress
	count, err := r.queries.CountAnsweredQuestions(ctx, params.UserSurveyResponseID)
	if err == nil {
		userResponse, err := r.queries.GetUserSurveyResponseByID(ctx, params.UserSurveyResponseID)
		if err == nil {
			questions, err := r.queries.GetQuestionsBySurveyID(ctx, userResponse.SurveyID)
			if err == nil && len(questions) > 0 {
				percentage := (float64(count) / float64(len(questions))) * 100
				percentageNumeric := pgtype.Numeric{}
				percentageNumeric.Scan(fmt.Sprintf("%.2f", percentage))

				r.queries.UpdateSurveyProgress(ctx, database.UpdateSurveyProgressParams{
					ID:                  params.UserSurveyResponseID,
					PercentageCompleted: percentageNumeric,
				})
			}
		}
	}

	return answer, nil
}

func (r *Repository) SaveAnswers(ctx context.Context, params []SaveAnswerParams) ([]database.AnswerResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var answers []database.AnswerResponse

	for _, param := range params {
		answer, err := r.SaveAnswer(ctx, param)
		if err != nil {
			return nil, fmt.Errorf("error saving answers: %v", err)
		}
		answers = append(answers, answer)
	}

	return answers, nil
}

func (r *Repository) GetAnswer(ctx context.Context, userSurveyResponseID, questionID int64) (database.AnswerResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	answer, err := r.queries.GetAnswer(ctx, database.GetAnswerParams{
		UserSurveyResponseID: userSurveyResponseID,
		QuestionID:           questionID,
	})
	if err != nil {
		return database.AnswerResponse{}, fmt.Errorf("error getting answer: %v", err)
	}

	return answer, nil
}

func (r *Repository) GetAnswersByUserSurveyResponse(ctx context.Context, userSurveyResponseID int64) ([]database.AnswerResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	answers, err := r.queries.GetAnswersByUserSurveyResponse(ctx, userSurveyResponseID)
	if err != nil {
		return nil, fmt.Errorf("error getting answers: %v", err)
	}

	return answers, nil
}

func (r *Repository) UpdateAnswer(ctx context.Context, params UpdateAnswerParams) (database.AnswerResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updateParams := database.UpdateAnswerParams{
		ID: params.ID,
	}

	if params.AnswerText != nil {
		updateParams.AnswerText = pgtype.Text{String: *params.AnswerText, Valid: true}
	}

	if params.SelectedOptionIDs != nil {
		updateParams.SelectedOptionIds = params.SelectedOptionIDs
	}

	answer, err := r.queries.UpdateAnswer(ctx, updateParams)
	if err != nil {
		return database.AnswerResponse{}, fmt.Errorf("error updating answer: %v", err)
	}

	return answer, nil
}
