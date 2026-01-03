package profiles

import (
	"context"
	"github.com/Adedunmol/answerly/api/jsonutil"
	"github.com/Adedunmol/answerly/api/tokens"
	"net/http"
)

type Handler struct {
	Store Store
}

func (h *Handler) GetProfileHandler(responseWriter http.ResponseWriter, request *http.Request) {
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

	data, err := h.Store.GetProfile(ctx, int64(userID))
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
		Message: "retrieved user's profile successfully",
		Data:    data,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
	return
}

func (h *Handler) UpdateProfileHandler(responseWriter http.ResponseWriter, request *http.Request) {
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

	data, err := jsonutil.UnmarshalJsonResponse[UpdateProfileBody](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	profile, err := h.Store.UpdateProfile(ctx, int64(userID), data)
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
		Message: "updated user's profile successfully",
		Data:    profile,
	}

	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
	return
}
