package surveys

import (
	"context"
	"github.com/Adedunmol/answerly/api/jsonutil"
	"github.com/Adedunmol/answerly/api/tokens"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
	"net/http"
	"strconv"
)

type Handler struct {
	Store Store
}

// ==================== Survey Management Handlers ====================

func (h *Handler) CreateSurveyHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	claims := request.Context().Value("claims").(*tokens.Claims)
	userID := claims.UserID

	if userID == 0 {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	data, err := jsonutil.UnmarshalJsonResponse[CreateSurveyParams](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	reward, err := decimal.NewFromString(data.Reward.String())
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid reward amount",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	params := CreateSurveyParams{
		Title:                data.Title,
		Description:          data.Description,
		Category:             data.Category,
		EstimatedTimeMinutes: data.EstimatedTimeMinutes,
		Reward:               reward,
		Eligibility:          data.Eligibility,
		CreatedBy:            int64(userID),
	}

	survey, err := h.Store.CreateSurvey(ctx, params)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "survey created successfully",
		Data:    survey,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusCreated)
}

func (h *Handler) GetSurveyHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	surveyIDStr := chi.URLParam(request, "surveyID")
	surveyID, err := strconv.ParseInt(surveyIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid survey ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	survey, err := h.Store.GetSurvey(ctx, surveyID)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusNotFound)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "survey retrieved successfully",
		Data:    survey,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

func (h *Handler) GetSurveyWithDetailsHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	surveyIDStr := chi.URLParam(request, "surveyID")
	surveyID, err := strconv.ParseInt(surveyIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid survey ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	surveyDetail, err := h.Store.GetSurveyWithDetails(ctx, surveyID)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusNotFound)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "survey details retrieved successfully",
		Data:    surveyDetail,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

func (h *Handler) ListSurveysHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	category := request.URL.Query().Get("category")
	status := request.URL.Query().Get("status")

	limitStr := request.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	offsetStr := request.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	params := ListSurveysParams{
		Category: category,
		Status:   status,
		Limit:    limit,
		Offset:   offset,
	}

	surveys, err := h.Store.ListSurveys(ctx, params)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "surveys retrieved successfully",
		Data:    surveys,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

func (h *Handler) UpdateSurveyHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	claims := request.Context().Value("claims").(*tokens.Claims)
	userID := claims.UserID

	if userID == 0 {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	surveyIDStr := chi.URLParam(request, "surveyID")
	surveyID, err := strconv.ParseInt(surveyIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid survey ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	data, err := jsonutil.UnmarshalJsonResponse[UpdateSurveyParams](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	params := UpdateSurveyParams{
		ID:                   surveyID,
		Title:                data.Title,
		Description:          data.Description,
		Category:             data.Category,
		EstimatedTimeMinutes: data.EstimatedTimeMinutes,
		Status:               data.Status,
	}

	if data.Reward != nil {
		reward, err := decimal.NewFromString(data.Reward.String())
		if err != nil {
			response := jsonutil.Response{
				Status:  "error",
				Message: "invalid reward amount",
			}
			jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
			return
		}
		params.Reward = &reward
	}

	if data.Eligibility != nil {
		params.Eligibility = data.Eligibility
	}

	survey, err := h.Store.UpdateSurvey(ctx, params)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "survey updated successfully",
		Data:    survey,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

func (h *Handler) DeleteSurveyHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	claims := request.Context().Value("claims").(*tokens.Claims)
	userID := claims.UserID

	if userID == 0 {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	surveyIDStr := chi.URLParam(request, "surveyID")
	surveyID, err := strconv.ParseInt(surveyIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid survey ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	err = h.Store.DeleteSurvey(ctx, surveyID)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "survey deleted successfully",
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

// ==================== Question Management Handlers ====================

func (h *Handler) CreateQuestionHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	claims := request.Context().Value("claims").(*tokens.Claims)
	userID := claims.UserID

	if userID == 0 {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	surveyIDStr := chi.URLParam(request, "surveyID")
	surveyID, err := strconv.ParseInt(surveyIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid survey ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	data, err := jsonutil.UnmarshalJsonResponse[CreateQuestionParams](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	params := CreateQuestionParams{
		SurveyID:     surveyID,
		QuestionText: data.QuestionText,
		QuestionType: data.QuestionType,
		IsRequired:   data.IsRequired,
		OrderIndex:   data.OrderIndex,
	}

	question, err := h.Store.CreateQuestion(ctx, params)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "question created successfully",
		Data:    question,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusCreated)
}

func (h *Handler) GetQuestionsBySurveyHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	surveyIDStr := chi.URLParam(request, "surveyID")
	surveyID, err := strconv.ParseInt(surveyIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid survey ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	questions, err := h.Store.GetQuestionsBySurveyID(ctx, surveyID)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "questions retrieved successfully",
		Data:    questions,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

func (h *Handler) UpdateQuestionHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	claims := request.Context().Value("claims").(*tokens.Claims)
	userID := claims.UserID

	if userID == 0 {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	questionIDStr := chi.URLParam(request, "questionID")
	questionID, err := strconv.ParseInt(questionIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid question ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	data, err := jsonutil.UnmarshalJsonResponse[UpdateQuestionParams](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	params := UpdateQuestionParams{
		ID:           questionID,
		QuestionText: data.QuestionText,
		QuestionType: data.QuestionType,
		IsRequired:   data.IsRequired,
		OrderIndex:   data.OrderIndex,
	}

	question, err := h.Store.UpdateQuestion(ctx, params)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "question updated successfully",
		Data:    question,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

func (h *Handler) DeleteQuestionHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	claims := request.Context().Value("claims").(*tokens.Claims)
	userID := claims.UserID

	if userID == 0 {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	questionIDStr := chi.URLParam(request, "questionID")
	questionID, err := strconv.ParseInt(questionIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid question ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	err = h.Store.DeleteQuestion(ctx, questionID)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "question deleted successfully",
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

// ==================== Question Options Handlers ====================

func (h *Handler) CreateQuestionOptionHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	claims := request.Context().Value("claims").(*tokens.Claims)
	userID := claims.UserID

	if userID == 0 {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	questionIDStr := chi.URLParam(request, "questionID")
	questionID, err := strconv.ParseInt(questionIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid question ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	data, err := jsonutil.UnmarshalJsonResponse[CreateQuestionOptionParams](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	params := CreateQuestionOptionParams{
		QuestionID: questionID,
		OptionText: data.OptionText,
		OrderIndex: data.OrderIndex,
	}

	option, err := h.Store.CreateQuestionOption(ctx, params)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "option created successfully",
		Data:    option,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusCreated)
}

func (h *Handler) GetOptionsByQuestionHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	questionIDStr := chi.URLParam(request, "questionID")
	questionID, err := strconv.ParseInt(questionIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid question ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	options, err := h.Store.GetOptionsByQuestionID(ctx, questionID)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "options retrieved successfully",
		Data:    options,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

func (h *Handler) UpdateQuestionOptionHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	claims := request.Context().Value("claims").(*tokens.Claims)
	userID := claims.UserID

	if userID == 0 {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	optionIDStr := chi.URLParam(request, "optionID")
	optionID, err := strconv.ParseInt(optionIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid option ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	data, err := jsonutil.UnmarshalJsonResponse[UpdateQuestionOptionParams](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	params := UpdateQuestionOptionParams{
		ID:         optionID,
		OptionText: data.OptionText,
		OrderIndex: data.OrderIndex,
	}

	option, err := h.Store.UpdateQuestionOption(ctx, params)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "option updated successfully",
		Data:    option,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

func (h *Handler) DeleteQuestionOptionHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	claims := request.Context().Value("claims").(*tokens.Claims)
	userID := claims.UserID

	if userID == 0 {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	optionIDStr := chi.URLParam(request, "optionID")
	optionID, err := strconv.ParseInt(optionIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid option ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	err = h.Store.DeleteQuestionOption(ctx, optionID)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "option deleted successfully",
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

// ==================== User Survey Response Handlers ====================

func (h *Handler) StartSurveyHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	claims := request.Context().Value("claims").(*tokens.Claims)
	userID := claims.UserID

	if userID == 0 {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	surveyIDStr := chi.URLParam(request, "surveyID")
	surveyID, err := strconv.ParseInt(surveyIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid survey ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	surveyResponse, err := h.Store.StartSurvey(ctx, int64(userID), surveyID)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "survey started successfully",
		Data:    surveyResponse,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusCreated)
}

func (h *Handler) GetUserSurveyProgressHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	claims := request.Context().Value("claims").(*tokens.Claims)
	userID := claims.UserID

	if userID == 0 {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	surveyIDStr := chi.URLParam(request, "surveyID")
	surveyID, err := strconv.ParseInt(surveyIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid survey ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	progress, err := h.Store.GetUserSurveyProgress(ctx, int64(userID), surveyID)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusNotFound)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "survey progress retrieved successfully",
		Data:    progress,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

func (h *Handler) ListUserSurveysHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	claims := request.Context().Value("claims").(*tokens.Claims)
	userID := claims.UserID

	if userID == 0 {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	status := request.URL.Query().Get("status")

	surveys, err := h.Store.ListUserSurveys(ctx, int64(userID), status)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "user surveys retrieved successfully",
		Data:    surveys,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

func (h *Handler) CompleteSurveyHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	claims := request.Context().Value("claims").(*tokens.Claims)
	userID := claims.UserID

	if userID == 0 {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	surveyIDStr := chi.URLParam(request, "surveyID")
	surveyID, err := strconv.ParseInt(surveyIDStr, 10, 64)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid survey ID",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	surveyResponse, err := h.Store.CompleteSurvey(ctx, int64(userID), surveyID)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "survey completed successfully",
		Data:    surveyResponse,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

// ==================== Answer Management Handlers ====================

func (h *Handler) SaveAnswerHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	claims := request.Context().Value("claims").(*tokens.Claims)
	userID := claims.UserID

	if userID == 0 {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	data, err := jsonutil.UnmarshalJsonResponse[SaveAnswerParams](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	params := SaveAnswerParams{
		UserSurveyResponseID: data.UserSurveyResponseID,
		QuestionID:           data.QuestionID,
		AnswerText:           data.AnswerText,
		SelectedOptionIDs:    data.SelectedOptionIDs,
	}

	answer, err := h.Store.SaveAnswer(ctx, params)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := jsonutil.Response{
		Status:  "success",
		Message: "answer saved successfully",
		Data:    answer,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
}

//
//func (h *Handler) SaveAnswersHandler(responseWriter http.ResponseWriter, request *http.Request) {
//	ctx := context.Background()
//
//	claims := request.Context().Value("claims").(*tokens.Claims)
//	userID := claims.UserID
//
//	if userID == 0 {
//		response := jsonutil.Response{
//			Status:  "error",
//			Message: "unauthorized",
//		}
//		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
//		return
//	}
//
//	data, err := jsonutil.UnmarshalJsonResponse[SaveAnswersBody](request)
//	if err != nil {
//		response := jsonutil.Response{
//			Status:  "error",
//			Message: err.Error(),
//		}
//		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
//		return
//	}
//
//	var
