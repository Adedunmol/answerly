package auth

import (
	"context"
	"database/sql"
	"errors"
	"github.com/Adedunmol/answerly/api/custom_errors"
	"github.com/Adedunmol/answerly/api/jsonutil"
	"github.com/Adedunmol/answerly/api/otp"
	"github.com/Adedunmol/answerly/api/profiles"
	"github.com/Adedunmol/answerly/api/tokens"
	"github.com/Adedunmol/answerly/api/wallets"
	"github.com/Adedunmol/answerly/database"
	"github.com/Adedunmol/answerly/queue"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"
	"log"
	"net/http"
	"time"
)

var allowedRoles = map[string]bool{
	"user":       true,
	"researcher": true,
}

const TokenExpiration = 30

type Response struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Handler struct {
	Store        Store
	Queue        queue.Queue
	OTPStore     otp.Store
	Token        tokens.TokenService
	WalletStore  wallets.Store
	ProfileStore profiles.Store
}

const OtpExpiration = 30

func (h *Handler) CreateUserHandler(responseWriter http.ResponseWriter, request *http.Request) {

	ctx := context.Background()

	data, err := jsonutil.UnmarshalJsonResponse[CreateUserBody](request)
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

	q := request.URL.Query()

	role := q.Get("role")

	if role == "" {
		role = "user"
	}

	if !allowedRoles[role] {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid role",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	data.Role = role

	user, err := h.Store.CreateUser(ctx, &data)

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

	_, err = h.WalletStore.CreateWallet(ctx, user.ID)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	err = h.ProfileStore.CreateProfile(ctx, user.ID)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	err = h.OTPStore.CreateOTP(ctx, user.ID, string(hashedCode), time.Now().Add(OtpExpiration*time.Minute), "verification")

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	err = h.Queue.Enqueue(&queue.EmailDeliveryPayload{
		Name:     "email",
		Template: "verification_mail",
		Subject:  "Verify your email",
		Email:    data.Email,
		Data: struct {
			Username   string
			Code       string
			Expiration int
		}{
			//Username:   data.Username,
			Code:       code,
			Expiration: OtpExpiration,
		},
	})

	if err != nil {
		log.Printf("error enqueuing email task: %s", err)
	}

	_, refreshToken := h.Token.GenerateToken(int(user.ID), data.Email, user.EmailVerified.Bool, user.Role) // user.Verified // role

	updateUser := UpdateUserBody{RefreshToken: refreshToken}

	if err = h.Store.UpdateUser(ctx, int(user.ID), updateUser); err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusCreated)
	return
}

func (h *Handler) LoginUserHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()
	data, err := jsonutil.UnmarshalJsonResponse[LoginUserBody](request)
	if err != nil {
		response := jsonutil.Response{Status: "error", Message: err.Error()}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	user, err := h.Store.FindUserByEmail(ctx, data.Email)

	if err != nil {
		response := jsonutil.Response{Status: "error", Message: err.Error()}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}
	match := h.Token.ComparePasswords(user.Password, data.Password)

	if !match {
		response := jsonutil.Response{Status: "error", Message: "invalid credentials"}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}
	token, refreshToken := h.Token.GenerateToken(int(user.ID), data.Email, user.EmailVerified.Bool, user.Role) //user.Verified // role
	updateUser := UpdateUserBody{RefreshToken: refreshToken}
	if err = h.Store.UpdateUser(ctx, int(user.ID), updateUser); err != nil {
		response := jsonutil.Response{Status: "error", Message: err.Error()}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}
	expires := time.Now().AddDate(0, 0, 7)

	cookie := &http.Cookie{Name: "refresh_token", Value: refreshToken, Path: "/", Expires: expires, Secure: true, HttpOnly: true, MaxAge: 86400}
	http.SetCookie(responseWriter, cookie)

	response := Response{Status: "Success", Message: "User logged in", Data: map[string]interface{}{"token": token, "expiration": TokenExpiration}}

	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
	return
}

func (h *Handler) VerifyOTPHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	data, err := jsonutil.UnmarshalJsonResponse[VerifyOTPBody](request)
	if err != nil {
		response := jsonutil.Response{Status: "error", Message: err.Error()}

		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return

	}

	user, err := h.Store.FindUserByEmail(ctx, data.Email)
	if err != nil {
		response := jsonutil.Response{Status: "error", Message: err.Error()}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	code, err := h.OTPStore.GetOTP(ctx, user.ID, "verification")

	if err != nil {
		response := jsonutil.Response{Status: "error", Message: err.Error()}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	isValid := h.Token.ComparePasswords(code, data.Code)
	if !isValid {
		response := jsonutil.Response{Status: "error", Message: "invalid credentials"}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}
	err = h.OTPStore.DeleteOTP(ctx, user.ID, "verification")

	if err != nil {
		response := jsonutil.Response{Status: "error", Message: err.Error()}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}
	updateBody := UpdateUserBody{Verified: true}

	err = h.Store.UpdateUser(ctx, int(user.ID), updateBody)
	if err != nil {
		response := jsonutil.Response{Status: "error", Message: err.Error()}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := Response{Status: "Success", Message: "User verified successfully"}
	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
	return
}

func (h *Handler) LogoutUserHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	refreshToken, err := request.Cookie("refresh_token")

	response := Response{
		Status:  "Success",
		Message: "User logged out successfully",
	}

	if err != nil {
		response := jsonutil.Response{
			Status:  "success",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
		return
	}

	cookie := http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		HttpOnly: true,
		MaxAge:   -1,
	}

	http.SetCookie(responseWriter, &cookie)

	err = h.Store.DeleteRefreshToken(ctx, refreshToken.Value)

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
	return
}

func (h *Handler) RequestCodeHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	data, err := jsonutil.UnmarshalJsonResponse[RequestOTPBody](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	user, err := h.Store.FindUserByEmail(ctx, data.Email)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	err = h.OTPStore.DeleteOTP(ctx, user.ID, "verification")

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
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
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	err = h.OTPStore.CreateOTP(ctx, user.ID, string(hashedCode), time.Now().Add(OtpExpiration*time.Minute), "verification")

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	err = h.Queue.Enqueue(&queue.EmailDeliveryPayload{
		Name:     "email",
		Template: "verification_mail",
		Subject:  "Verify your email",
		Email:    data.Email,
		Data: struct {
			Username   string
			Code       string
			Expiration int
		}{
			Code:       code,
			Expiration: OtpExpiration,
		},
	})

	if err != nil {
		log.Printf("error enqueuing email task: %s", err)
	}

	response := Response{
		Status:  "Success",
		Message: "Code has been sent successfully",
	}

	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
	return
}

func (h *Handler) ForgotPasswordRequestHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	data, err := jsonutil.UnmarshalJsonResponse[RequestOTPBody](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	user, err := h.Store.FindUserByEmail(ctx, data.Email)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	err = h.OTPStore.DeleteOTP(ctx, user.ID, "forgot-password")

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
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

	err = h.OTPStore.CreateOTP(ctx, user.ID, string(hashedCode), time.Now().Add(OtpExpiration*time.Minute), "forgot-password")

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	err = h.Queue.Enqueue(&queue.EmailDeliveryPayload{
		Name:     "email",
		Template: "forgot_password_mail",
		Subject:  "Forgot Password",
		Email:    data.Email,
		Data: struct {
			Username   string
			Code       string
			Expiration int
		}{
			//Username:   user.Username.String,
			Code:       code,
			Expiration: OtpExpiration,
		},
	})

	if err != nil {
		log.Printf("error enqueuing email task: %s", err)
	}

	response := Response{
		Status:  "Success",
		Message: "Code has been sent successfully",
	}

	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
	return
}

func (h *Handler) ForgotPasswordHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	data, err := jsonutil.UnmarshalJsonResponse[ForgotPasswordBody](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	user, err := h.Store.FindUserByEmail(ctx, data.Email)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	code, err := h.OTPStore.GetOTP(ctx, user.ID, "forgot-password")

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	log.Println("code: ", code)
	log.Println("candidate: ", data.Code)

	isValid := h.Token.ComparePasswords(code, data.Code)

	if !isValid {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid code",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	err = h.OTPStore.DeleteOTP(ctx, user.ID, "forgot-password")

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data.NewPassword), 10)

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	updateBody := UpdateUserBody{Password: string(hashedPassword)}

	err = h.Store.UpdateUser(ctx, int(user.ID), updateBody)

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := Response{
		Status:  "Success",
		Message: "Password has been reset successfully",
	}

	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
	return
}

func (h *Handler) ResetPasswordHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	data, err := jsonutil.UnmarshalJsonResponse[ResetPasswordBody](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	claims := request.Context().Value("claims").(*tokens.Claims)
	email := claims.Email

	if email == "" {
		response := jsonutil.Response{
			Status:  "error",
			Message: "unauthorized",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	userData, err := h.Store.FindUserByEmail(ctx, email)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	match := h.Token.ComparePasswords(userData.Password, data.OldPassword)

	if !match {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid credentials",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data.NewPassword), 10)

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	updateBody := UpdateUserBody{Password: string(hashedPassword)}

	err = h.Store.UpdateUser(ctx, int(userData.ID), updateBody)

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	response := Response{
		Status:  "Success",
		Message: "Password has been reset successfully",
	}

	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
	return
}

func (h *Handler) RefreshTokenHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	oldRefreshToken, err := request.Cookie("refresh_token")

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	_, err = h.Token.DecodeToken(oldRefreshToken.Value)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	user, err := h.Store.FindUserWithRefreshToken(ctx, oldRefreshToken.Value)

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: "invalid token",
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	accessToken, newRefreshToken := h.Token.GenerateToken(int(user.ID), user.Email, user.EmailVerified.Bool, user.Role)

	err = h.Store.UpdateRefreshToken(ctx, oldRefreshToken.Value, newRefreshToken)

	if err != nil && errors.Is(err, custom_errors.ErrNotFound) {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	expires := time.Now().AddDate(0, 0, 7)

	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    newRefreshToken,
		Path:     "/",
		Expires:  expires,
		Secure:   true,
		HttpOnly: true,
		MaxAge:   86400,
	}

	http.SetCookie(responseWriter, cookie)

	response := Response{
		Status:  "Success",
		Message: "Access token refreshed successfully",
		Data:    map[string]interface{}{"token": accessToken, "expiration": time.Now().Add(24 * time.Hour)},
	}

	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
	return
}

func (h *Handler) GoogleSignUpHandler(responseWriter http.ResponseWriter, request *http.Request) {
	ctx := context.Background()

	data, err := jsonutil.UnmarshalJsonResponse[GoogleAuthRequestBody](request)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
		return
	}

	payload, err := verifyGoogleIDToken(data.IDToken)
	if err != nil {
		response := jsonutil.Response{
			Status:  "error",
			Message: err.Error(),
		}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusUnauthorized)
		return
	}

	email := payload.Claims["email"].(string)
	googleID := payload.Subject

	user, err := h.Store.FindUserByEmail(context.Background(), email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			q := request.URL.Query()

			role := q.Get("role")

			if role == "" {
				role = "user"
			}

			if !allowedRoles[role] {
				response := jsonutil.Response{
					Status:  "error",
					Message: "invalid role",
				}
				jsonutil.WriteJSONResponse(responseWriter, response, http.StatusBadRequest)
				return
			}
			body := CreateUserBody{
				Email:    email,
				Role:     role,
				GoogleID: googleID,
			}

			newUser, err := h.Store.CreateUser(ctx, &body)

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

			_, err = h.WalletStore.CreateWallet(ctx, user.ID)
			if err != nil {
				response := jsonutil.Response{
					Status:  "error",
					Message: err.Error(),
				}
				jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
				return
			}

			err = h.ProfileStore.CreateProfile(ctx, user.ID)
			if err != nil {
				response := jsonutil.Response{
					Status:  "error",
					Message: err.Error(),
				}
				jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
				return
			}

			accessToken, refreshToken := h.Token.GenerateToken(int(user.ID), newUser.Email, newUser.EmailVerified.Bool, newUser.Role)

			updateUser := UpdateUserBody{RefreshToken: refreshToken, Verified: true}

			if err = h.Store.UpdateUser(ctx, int(newUser.ID), updateUser); err != nil {
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
				Data: struct {
					User        database.User `json:"user"`
					AccessToken string        `json:"access_token"`
				}{
					User:        newUser,
					AccessToken: accessToken,
				},
			}

			jsonutil.WriteJSONResponse(responseWriter, response, http.StatusCreated)
			return
		}

		response := Response{
			Status:  "error",
			Message: err.Error(),
		}

		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	token, refreshToken := h.Token.GenerateToken(int(user.ID), user.Email, user.EmailVerified.Bool, user.Role)

	updateUser := UpdateUserBody{RefreshToken: refreshToken}

	if err = h.Store.UpdateUser(ctx, int(user.ID), updateUser); err != nil {
		response := jsonutil.Response{Status: "error", Message: err.Error()}
		jsonutil.WriteJSONResponse(responseWriter, response, http.StatusInternalServerError)
		return
	}

	expires := time.Now().AddDate(0, 0, 7)

	cookie := &http.Cookie{Name: "refresh_token", Value: refreshToken, Path: "/", Expires: expires, Secure: true, HttpOnly: true, MaxAge: 86400}

	http.SetCookie(responseWriter, cookie)

	response := Response{Status: "Success", Message: "User logged in", Data: map[string]interface{}{"token": token, "expiration": TokenExpiration}}

	jsonutil.WriteJSONResponse(responseWriter, response, http.StatusOK)
	return
}

func verifyGoogleIDToken(token string) (*idtoken.Payload, error) {
	payload, err := idtoken.Validate(
		context.Background(),
		token,
		"")

	if err != nil {
		return nil, err
	}

	return payload, nil
}

//
//// implement oauth
////
////const (
////	key    = "somerandomkey"
////	maxAge = 86400 * 30
////	isProd = false
////)
////
////func (h *Handler) NewAuthHandler() {
////
////	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
////	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
////
////	// gorilla sessions
////	store := sessions.NewCookieStore([]byte(key))
////	store.MaxAge(maxAge)
////
////	store.Options.Path = "/"
////	store.Options.HttpOnly = true
////	store.Options.Secure = isProd
////
////	gothic.Store = store
////
////	goth.UseProviders(
////		google.New(googleClientID, googleClientSecret, "http://localhost:3000/auth/google/callback"),
////	)
////}
////
////func (h *Handler) GetAuthCallback(responseWriter http.ResponseWriter, request *http.Request) {
////	provider := chi.URLParam(request, "provider")
////
////	request = request.WithContext(context.WithValue(request.Context(), "provider", provider))
////
////	user, err := gothic.CompleteUserAuth(responseWriter, request)
////	if err != nil {
////		fmt.Fprintln(responseWriter, err)
////		return
////	}
////	fmt.Println(user)
////
////	http.Redirect(responseWriter, request, "/", http.StatusFound)
////}
////
////func (h *Handler) LogoutOAuth(responseWriter http.ResponseWriter, request *http.Request) {
////	gothic.Logout(responseWriter, request)
////	responseWriter.Header().Set("Location", "/")
////	responseWriter.WriteHeader(http.StatusTemporaryRedirect)
////}
////
////func (h *Handler) GetUserOAuth(responseWriter http.ResponseWriter, request *http.Request) {
////	if gothUser, err := gothic.CompleteUserAuth(responseWriter, request); err == nil {
////		//t, _ := template.New("foo").Parse(userTemplate)
////		//t.Execute(responseWriter, gothUser)
////		http.Redirect(responseWriter, request, "/", http.StatusFound)
////	} else {
////		gothic.BeginAuthHandler(responseWriter, request)
////	}
////}
