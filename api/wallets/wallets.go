package wallets

import (
	"context"
	"errors"
	"github.com/Adedunmol/answerly/api/custom_errors"
	"github.com/Adedunmol/answerly/api/jsonutil"
	"github.com/Adedunmol/answerly/api/tokens"
	"github.com/shopspring/decimal"
	"net/http"
)

type Handler struct {
	Store Store
}

func (h *Handler) GetWalletHandler(responseWriter http.ResponseWriter, request *http.Request) {
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

	wallet, err := h.Store.GetWallet(ctx, int64(userID))
	if err != nil {
		if errors.Is(err, custom_errors.ErrNotFound) {
			response := jsonutil.Response{
				Status:  "error",
				Message: err.Error(),
			}
			jsonutil.WriteJSONResponse(responseWriter, response, http.StatusNotFound)
			return
		}
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}
	response := jsonutil.Response{
		Status:  "success",
		Message: "retrieved company's wallet successfully",
		Data:    wallet,
	}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
	return
}

func (h *Handler) TopUpWalletHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	body, err := jsonutil.UnmarshalJsonResponse[TopUpWalletBody](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	if body.Amount.LessThanOrEqual(decimal.Zero) {
		response := jsonutil.Response{
			Status:  "error",
			Message: "amount must be greater than zero",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

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

	wallet, err := h.Store.TopUpWallet(ctx, int64(userID), body.Amount)

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
		Message: "updated user's wallet successfully",
		Data:    wallet,
	}

	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
	return
}

func (h *Handler) WithdrawFromWalletHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	body, err := jsonutil.UnmarshalJsonResponse[WithdrawBody](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	if body.Amount.LessThanOrEqual(decimal.Zero) {
		response := jsonutil.Response{
			Status:  "error",
			Message: "amount must be greater than zero",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

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

	wallet, err := h.Store.GetWallet(ctx, int64(userID))
	if err != nil {
		if errors.Is(err, custom_errors.ErrNotFound) {
			response := jsonutil.Response{
				Status:  "error",
				Message: err.Error(),
			}
			jsonutil.WriteJSONResponse(responseWriter, response, http.StatusNotFound)
			return
		}
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	var dec decimal.Decimal
	err = dec.Scan(wallet.Wallet.Balance)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "internal server error",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	if dec.LessThan(body.Amount) {
		response := jsonutil.Response{
			Status:  "error",
			Message: "insufficient balance",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	if body.Save {
		// create an entry for the details in the db (unique account number, phone number)
		err = h.Store.CreatePaymentMethod(ctx, PaymentMethodBody{
			UserID:        int64(userID),
			Type:          body.Method,
			Provider:      "",
			AccountName:   "",
			AccountNumber: body.AccountNumber,
			PhoneNumber:   body.PhoneNumber,
		})
		if err != nil && !errors.Is(err, custom_errors.ErrConflict) {
			response := jsonutil.Response{
				Status:  "error",
				Message: err.Error(),
			}
			jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
			return
		}
	}

	switch body.Method {
	case "airtime":
		// create transaction entry
		err = h.Store.CreateTransaction(ctx, wallet.Wallet.ID, body.Amount, "airtime")
		if err != nil {
			response := jsonutil.Response{
				Status:  "error",
				Message: err.Error(),
			}
			jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
			return
		}

		// perform necessary operations

		jsonutil.WriteJSONResponse(responseWriter, struct{}{}, http.StatusOK)
		return
	case "bank_transfer":
		// create transaction entry
		err = h.Store.CreateTransaction(ctx, wallet.Wallet.ID, body.Amount, "bank_transfer")
		if err != nil {
			response := jsonutil.Response{
				Status:  "error",
				Message: err.Error(),
			}
			jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
			return
		}

		// perform necessary operations

		jsonutil.WriteJSONResponse(responseWriter, struct{}{}, http.StatusOK)
		return
	default:
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid method",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}
}
