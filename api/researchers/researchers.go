package researchers

import (
	"errors"
	"github.com/Adedunmol/answerly/api/custom_errors"
	"github.com/Adedunmol/answerly/api/jsonutil"
	"github.com/Adedunmol/answerly/api/otp"
	"github.com/Adedunmol/answerly/queue"
	"golang.org/x/crypto/bcrypt"
	"log"
	"net/http"
	"time"
)

const TokenExpiration = 30 * time.Minute

type Response struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type TokenService interface {
	GenerateSecureOTP(length int) (string, error)
	ComparePasswords(storedPassword, candidatePassword string) bool
	GenerateToken(userID int, email string, verified bool) (string, error)
}

type Handler struct {
	Store    Store
	Queue    queue.Queue
	OTPStore otp.OTPStore
	Token    TokenService
}

const OtpExpiration = 30

func (h *Handler) CreateResearcherHandler(responseWriter http.ResponseWriter, request *http.Request) {

	//ctx := context.Background()

	data, err := jsonutil.UnmarshalJsonResponse[CreateAdminBody](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data.Password), 10)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	data.Password = string(hashedPassword)

	user, err := h.Store.CreateUser(&data)

	if err != nil {
		ok := errors.Is(err, custom_errors.ErrConflict)

		if ok {
			response := jsonutil.Response{
				Status:  "error",
				Message: err.Error(),
			}
			jsonutil.WriteJSONResponse(responseWriter, response, http.StatusConflict)
			return
		}

		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := Response{
		Status:  "Success",
		Message: "User created successfully",
		Data:    user,
	}

	code, err := h.Token.GenerateSecureOTP(6)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	hashedCode, err := bcrypt.GenerateFromPassword([]byte(code), 10)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	err = h.OTPStore.CreateOTP(data.Email, string(hashedCode), OtpExpiration)

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	err = h.Queue.Enqueue(&queue.TaskPayload{
		Type: queue.TypeEmailDelivery,
		Payload: map[string]interface{}{
			"email":    data.Email,
			"template": "verification_mail",
			"subject":  "Verify your email",
			"data": struct {
				//Username   string
				Code       string
				Expiration int
			}{
				//Username:   data.Username,
				Code:       code,
				Expiration: OtpExpiration,
			},
		},
	})

	if err != nil {
		log.Printf("error enqueuing email task: %s", err)
	}

	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusCreated)
	return
}
