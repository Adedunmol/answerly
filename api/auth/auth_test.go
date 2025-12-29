package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Adedunmol/answerly/api/tokens"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Adedunmol/answerly/api/auth"
	"github.com/Adedunmol/answerly/api/custom_errors"
	"github.com/Adedunmol/answerly/database"
	"github.com/Adedunmol/answerly/queue"
	"github.com/jackc/pgx/v5/pgtype"
)

// ============================================================================
// Test Helpers
// ============================================================================

func assertResponseCode(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("response code = %d, want %d", got, want)
	}
}

func assertResponseStatus(t *testing.T, got map[string]interface{}, wantStatus string) {
	t.Helper()
	if got["status"] != wantStatus {
		t.Errorf("status = %v, want %v", got["status"], wantStatus)
	}
}

func assertResponseMessage(t *testing.T, got map[string]interface{}, wantMessage string) {
	t.Helper()
	if got["message"] != wantMessage {
		t.Errorf("message = %v, want %v", got["message"], wantMessage)
	}
}

// ============================================================================
// Stubs
// ============================================================================
type StubTokenService struct {
	ShouldFailOTP   bool
	ShouldFailToken bool
}

func (s *StubTokenService) GenerateSecureOTP(length int) (string, error) {
	if s.ShouldFailOTP {
		return "", errors.New("failed to generate OTP")
	}
	return "123456", nil
}

func (s *StubTokenService) ComparePasswords(storedPassword, candidatePassword string) bool {
	return storedPassword == candidatePassword
}

func (s *StubTokenService) GenerateToken(userID int, email string, verified bool, role string) (string, string) {
	//if s.ShouldFailToken {
	//	return "", errors.New("failed to generate token")
	//}
	return "mock-jwt-token", "mock-refresh-token"
}

func (s *StubTokenService) DecodeToken(tokenString string) (tokens.Claims, error) {
	// You can add logic here to fail if tokenString is "invalid-token" if needed
	if tokenString == "invalid-token" {
		return tokens.Claims{}, errors.New("invalid token")
	}

	// Return a valid mock claim
	return tokens.Claims{
		// valid mock data
	}, nil
}

type StubQueue struct {
	Tasks      []queue.Processor
	ShouldFail bool
}

func (q *StubQueue) Enqueue(processor queue.Processor) error {
	if q.ShouldFail {
		return errors.New("queue error")
	}
	q.Tasks = append(q.Tasks, processor)
	return nil
}

type StubOTPStore struct {
	OTPs               map[int64]auth.OTP
	ShouldFailCreate   bool
	ShouldFailDelete   bool
	ShouldFailValidate bool
}

func NewStubOTPStore() *StubOTPStore {
	return &StubOTPStore{
		OTPs: make(map[int64]auth.OTP),
	}
}

// Fix: Signature changed to match Interface (expiration is int)
// Fix: Removed undefined `E: email` assignment
func (s *StubOTPStore) CreateOTP(userID int64, otp string, expiration time.Time) error {
	if s.ShouldFailCreate {
		return errors.New("failed to create OTP")
	}

	futureTime := expiration
	currentTime := time.Now()

	s.OTPs[userID] = auth.OTP{
		ID: 1,
		// E field removed as it wasn't passed and caused compilation error
		OTP:       otp,
		ExpiresAt: &futureTime,
		CreatedAt: &currentTime,
	}
	return nil
}

// Fix: Signature changed to match Interface (returns string, error)
// Fix: Logic updated to return the OTP string instead of boolean
func (s *StubOTPStore) GetOTP(userID int64) (string, error) {
	if s.ShouldFailValidate {
		return "", errors.New("validation error")
	}

	otpData, exists := s.OTPs[userID]
	if !exists {
		return "", custom_errors.ErrInvalidOtp
	}

	if otpData.ExpiresAt.Before(time.Now()) {
		return "", custom_errors.ErrInvalidOtp
	}

	return otpData.OTP, nil
}

func (s *StubOTPStore) DeleteOTP(userID int64) error {
	if s.ShouldFailDelete {
		return errors.New("failed to delete OTP")
	}
	delete(s.OTPs, userID)
	return nil
}

type StubUserStore struct {
	Users                 []database.User
	ShouldFailCreate      bool
	ShouldFailFind        bool
	ShouldFailUpdate      bool
	ShouldFailDeleteToken bool
	ShouldFailUpdateToken bool
}

func NewStubUserStore() *StubUserStore {
	return &StubUserStore{
		Users: make([]database.User, 0),
	}
}

func (s *StubUserStore) CreateUser(body *auth.CreateUserBody) (database.CreateUserRow, error) {
	if s.ShouldFailCreate {
		return database.CreateUserRow{}, errors.New("database error")
	}

	for _, u := range s.Users {
		if u.Email == body.Email {
			return database.CreateUserRow{}, custom_errors.ErrConflict
		}
	}

	user := database.User{
		ID:            int64(len(s.Users) + 1),
		Username:      pgtype.Text{String: body.Username, Valid: true},
		Email:         body.Email,
		PasswordHash:  pgtype.Text{String: body.Password, Valid: true},
		EmailVerified: pgtype.Bool{Bool: false, Valid: true},
	}

	s.Users = append(s.Users, user)

	return database.CreateUserRow{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	}, nil
}

func (s *StubUserStore) FindUserByEmail(email string) (database.User, error) {
	if s.ShouldFailFind {
		return database.User{}, errors.New("database error")
	}

	for _, u := range s.Users {
		if u.Email == email {
			return u, nil
		}
	}
	return database.User{}, errors.New("user not found")
}

func (s *StubUserStore) FindUserByID(id int) (database.User, error) {
	if s.ShouldFailFind {
		return database.User{}, errors.New("database error")
	}

	for _, u := range s.Users {
		if u.ID == int64(id) {
			return u, nil
		}
	}
	return database.User{}, errors.New("user not found")
}

func (s *StubUserStore) UpdateUser(id int, data auth.UpdateUserBody) error {
	if s.ShouldFailUpdate {
		return errors.New("database error")
	}

	for i, u := range s.Users {
		if u.ID == int64(id) {
			if data.Verified {
				s.Users[i].EmailVerified = pgtype.Bool{Bool: data.Verified, Valid: true}
			}
			if data.Password != "" {
				s.Users[i].PasswordHash = pgtype.Text{String: data.Password, Valid: true}
			}
			if data.RefreshToken != "" {
				s.Users[i].RefreshToken = pgtype.Text{String: data.RefreshToken, Valid: true}
			}
			return nil
		}
	}

	return errors.New("user not found")
}

func (s *StubUserStore) DeleteRefreshToken(refreshToken string) error {
	if s.ShouldFailDeleteToken {
		return errors.New("failed to delete token")
	}

	for i, u := range s.Users {
		if u.RefreshToken.Valid && u.RefreshToken.String == refreshToken {
			s.Users[i].RefreshToken = pgtype.Text{Valid: false}
			return nil
		}
	}
	return nil
}

func (s *StubUserStore) UpdateRefreshToken(oldRefreshToken, refreshToken string) error {
	if s.ShouldFailUpdateToken {
		return errors.New("failed to update token")
	}

	for i, u := range s.Users {
		if u.RefreshToken.Valid && u.RefreshToken.String == oldRefreshToken {
			s.Users[i].RefreshToken = pgtype.Text{String: refreshToken, Valid: true}
			return nil
		}
	}
	return errors.New("refresh token not found")
}

// ============================================================================
// CreateUserHandler Tests
// ============================================================================

func TestCreateUserHandler(t *testing.T) {
	t.Run("successfully creates a user and sends OTP", func(t *testing.T) {
		store := NewStubUserStore()
		otpStore := NewStubOTPStore()
		queue := &StubQueue{}
		tokenService := &StubTokenService{}

		handler := &auth.Handler{
			Store:    store,
			OTPStore: otpStore,
			Queue:    queue,
			Token:    tokenService,
		}

		data := []byte(`{
			"username": "johndoe",
			"email": "john@example.com",
			"password": "password123",
			"date_of_birth": "1990-01-01"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.CreateUserHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusCreated)
		assertResponseStatus(t, got, "Success")
		assertResponseMessage(t, got, "User created successfully")

		if len(store.Users) != 1 {
			t.Errorf("expected 1 user in store, got %d", len(store.Users))
		}

		if len(otpStore.OTPs) != 1 {
			t.Errorf("expected 1 OTP in store, got %d", len(otpStore.OTPs))
		}

		if len(queue.Tasks) != 1 {
			t.Errorf("expected 1 email task in queue, got %d", len(queue.Tasks))
		}
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{"email": "test@example.com"`) // Invalid JSON

		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.CreateUserHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusBadRequest)
		assertResponseStatus(t, got, "error")
	})

	t.Run("returns 409 when user already exists", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com"},
		}

		handler := &auth.Handler{
			Store:    store,
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"username": "johndoe",
			"email": "john@example.com",
			"password": "password123",
			"date_of_birth": "1990-01-01"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.CreateUserHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusConflict)
		assertResponseStatus(t, got, "error")
	})

	t.Run("returns 500 when OTP generation fails", func(t *testing.T) {
		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{ShouldFailOTP: true},
		}

		data := []byte(`{
			"username": "johndoe",
			"email": "john@example.com",
			"password": "password123",
			"date_of_birth": "1990-01-01"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.CreateUserHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusInternalServerError)
	})

	t.Run("returns 500 when OTP store fails", func(t *testing.T) {
		otpStore := NewStubOTPStore()
		otpStore.ShouldFailCreate = true

		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: otpStore,
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"username": "johndoe",
			"email": "john@example.com",
			"password": "password123",
			"date_of_birth": "1990-01-01"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.CreateUserHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusInternalServerError)
	})
}

// ============================================================================
// LoginUserHandler Tests
// ============================================================================

func TestLoginUserHandler(t *testing.T) {
	t.Run("successfully logs in a user", func(t *testing.T) {
		store := NewStubUserStore()
		// Fix: Use pgtype.Text and correct field names (PasswordHash, EmailVerified) to match pgx/database.User struct
		store.Users = []database.User{
			{
				ID:            1,
				Email:         "john@example.com",
				PasswordHash:  pgtype.Text{String: "hashedpassword", Valid: true},
				EmailVerified: pgtype.Bool{Bool: true, Valid: true},
			},
		}

		handler := &auth.Handler{
			Store:    store,
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "john@example.com",
			"password": "hashedpassword"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.LoginUserHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusOK)
		assertResponseStatus(t, got, "Success")
		assertResponseMessage(t, got, "User logged in")

		// Check cookie was set
		cookies := rec.Result().Cookies()
		found := false
		for _, cookie := range cookies {
			if cookie.Name == "refresh_token" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected refresh_token cookie to be set")
		}
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{"email": "test@example.com"`) // Invalid JSON

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.LoginUserHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusBadRequest)
	})

	t.Run("returns 401 when user not found", func(t *testing.T) {
		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "nonexistent@example.com",
			"password": "password123"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.LoginUserHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusUnauthorized)
	})

	t.Run("returns 401 when password doesn't match", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{
				ID:           1,
				Email:        "john@example.com",
				PasswordHash: pgtype.Text{String: "correctpassword", Valid: true},
			},
		}

		handler := &auth.Handler{
			Store:    store,
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "john@example.com",
			"password": "wrongpassword"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.LoginUserHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusUnauthorized)
	})

	t.Run("returns 500 when token generation fails", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{
				ID:           1,
				Email:        "john@example.com",
				PasswordHash: pgtype.Text{String: "password123", Valid: true},
			},
		}

		handler := &auth.Handler{
			Store:    store,
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{ShouldFailToken: true},
		}

		data := []byte(`{
			"email": "john@example.com",
			"password": "password123"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.LoginUserHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusInternalServerError)
	})

	t.Run("returns 500 when update user fails", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{
				ID:           1,
				Email:        "john@example.com",
				PasswordHash: pgtype.Text{String: "password123", Valid: true},
			},
		}
		store.ShouldFailUpdate = true

		handler := &auth.Handler{
			Store:    store,
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "john@example.com",
			"password": "password123"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.LoginUserHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusInternalServerError)
	})
}

// ============================================================================
// VerifyOTPHandler Tests
// ============================================================================

func TestVerifyOTPHandler(t *testing.T) {
	t.Run("successfully verifies OTP", func(t *testing.T) {
		store := NewStubUserStore()
		// Fix: Use EmailVerified with pgtype
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com", EmailVerified: pgtype.Bool{Bool: false, Valid: true}},
		}

		otpStore := NewStubOTPStore()
		// Fix: Pass int expiration (e.g., 30 minutes)
		_ = otpStore.CreateOTP(1, "123456", time.Now().Add(30*time.Minute))

		handler := &auth.Handler{
			Store:    store,
			OTPStore: otpStore,
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "john@example.com",
			"code": "123456"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/verify", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.VerifyOTPHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusOK)
		assertResponseStatus(t, got, "Success")
		assertResponseMessage(t, got, "User verified successfully")

		// Check user is now verified
		user, _ := store.FindUserByID(1)
		// Fix: Check pgtype.Bool value
		if !user.EmailVerified.Bool {
			t.Error("expected user to be verified")
		}
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{"email": "test@example.com"`) // Invalid JSON

		req := httptest.NewRequest(http.MethodPost, "/auth/verify", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.VerifyOTPHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusBadRequest)
	})

	t.Run("returns 400 when user not found", func(t *testing.T) {
		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "nonexistent@example.com",
			"code": "123456"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/verify", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.VerifyOTPHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusBadRequest)
	})

	t.Run("returns 400 when OTP is invalid", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com"},
		}

		otpStore := NewStubOTPStore()
		_ = otpStore.CreateOTP(1, "123456", time.Now().Add(30*time.Minute))

		handler := &auth.Handler{
			Store:    store,
			OTPStore: otpStore,
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "john@example.com",
			"code": "wrong-code"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/verify", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.VerifyOTPHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusBadRequest)
	})

	t.Run("returns 500 when delete OTP fails", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com"},
		}

		otpStore := NewStubOTPStore()
		_ = otpStore.CreateOTP(1, "123456", time.Now().Add(30*time.Minute))
		otpStore.ShouldFailDelete = true

		handler := &auth.Handler{
			Store:    store,
			OTPStore: otpStore,
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "john@example.com",
			"code": "123456"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/verify", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.VerifyOTPHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusInternalServerError)
	})

	t.Run("returns 500 when update user fails", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com"},
		}
		store.ShouldFailUpdate = true

		otpStore := NewStubOTPStore()
		_ = otpStore.CreateOTP(1, "123456", time.Now().Add(30*time.Minute))

		handler := &auth.Handler{
			Store:    store,
			OTPStore: otpStore,
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "john@example.com",
			"code": "123456"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/verify", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.VerifyOTPHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusInternalServerError)
	})
}

// ============================================================================
// LogoutUserHandler Tests
// ============================================================================

func TestLogoutUserHandler(t *testing.T) {
	t.Run("successfully logs out user with refresh token", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{
				ID:    1,
				Email: "john@example.com",
				// Fix: Use pgtype.Text instead of sql.NullString
				RefreshToken: pgtype.Text{String: "valid-token", Valid: true},
			},
		}

		handler := &auth.Handler{
			Store:    store,
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "valid-token"})
		rec := httptest.NewRecorder()

		handler.LogoutUserHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusOK)
		assertResponseStatus(t, got, "Success")
		assertResponseMessage(t, got, "User logged out successfully")
	})

	t.Run("returns 200 when no refresh token cookie exists", func(t *testing.T) {
		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		rec := httptest.NewRecorder()

		handler.LogoutUserHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusOK)
	})

	t.Run("returns 200 even when delete fails", func(t *testing.T) {
		store := NewStubUserStore()
		store.ShouldFailDeleteToken = true

		handler := &auth.Handler{
			Store:    store,
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "token"})
		rec := httptest.NewRecorder()

		handler.LogoutUserHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusOK)
	})
}

// ============================================================================
// RequestCodeHandler Tests
// ============================================================================

func TestRequestCodeHandler(t *testing.T) {
	t.Run("successfully sends verification code", func(t *testing.T) {
		store := NewStubUserStore()
		// Fix: Use pgtype.Text
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com", Username: pgtype.Text{String: "johndoe", Valid: true}},
		}

		otpStore := NewStubOTPStore()
		queue := &StubQueue{}

		handler := &auth.Handler{
			Store:    store,
			OTPStore: otpStore,
			Queue:    queue,
			Token:    &StubTokenService{},
		}

		data := []byte(`{"email": "john@example.com"}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/request-code", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.RequestCodeHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusOK)
		assertResponseStatus(t, got, "Success")
		assertResponseMessage(t, got, "Code has been sent successfully")

		if len(queue.Tasks) != 1 {
			t.Errorf("expected 1 email task, got %d", len(queue.Tasks))
		}
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{"email": "test"`) // Invalid JSON

		req := httptest.NewRequest(http.MethodPost, "/auth/request-code", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.RequestCodeHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusBadRequest)
	})

	t.Run("returns 400 when user not found", func(t *testing.T) {
		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{"email": "nonexistent@example.com"}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/request-code", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.RequestCodeHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusBadRequest)
	})

	t.Run("returns 500 when OTP generation fails", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com"},
		}

		handler := &auth.Handler{
			Store:    store,
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{ShouldFailOTP: true},
		}

		data := []byte(`{"email": "john@example.com"}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/request-code", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.RequestCodeHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusInternalServerError)
	})
}

// ============================================================================
// ForgotPasswordRequestHandler Tests
// ============================================================================

func TestForgotPasswordRequestHandler(t *testing.T) {
	t.Run("successfully sends forgot password code", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com", Username: pgtype.Text{String: "johndoe", Valid: true}},
		}

		otpStore := NewStubOTPStore()
		queue := &StubQueue{}

		handler := &auth.Handler{
			Store:    store,
			OTPStore: otpStore,
			Queue:    queue,
			Token:    &StubTokenService{},
		}

		data := []byte(`{"email": "john@example.com"}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password/request", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.ForgotPasswordRequestHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusOK)
		assertResponseStatus(t, got, "Success")

		if len(queue.Tasks) != 1 {
			t.Errorf("expected 1 email task, got %d", len(queue.Tasks))
		}

		// Verify correct template was used
		//if queue.Tasks[0].Payload["template"] != "forgot_password_mail" {
		//	t.Error("expected forgot_password_mail template")
		//}
	})

	t.Run("returns 400 when user not found", func(t *testing.T) {
		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{"email": "nonexistent@example.com"}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password/request", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.ForgotPasswordRequestHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusBadRequest)
	})
}

// ============================================================================
// ForgotPasswordHandler Tests
// ============================================================================

func TestForgotPasswordHandler(t *testing.T) {
	t.Run("successfully resets password with valid OTP", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{
				ID:           1,
				Email:        "john@example.com",
				PasswordHash: pgtype.Text{String: "oldpassword", Valid: true},
			},
		}

		otpStore := NewStubOTPStore()
		_ = otpStore.CreateOTP(1, "123456", time.Now().Add(30*time.Minute))

		handler := &auth.Handler{
			Store:    store,
			OTPStore: otpStore,
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "john@example.com",
			"code": "123456",
			"new_password": "newpassword123"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.ForgotPasswordHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusOK)
		assertResponseStatus(t, got, "Success")
		assertResponseMessage(t, got, "Password has been reset successfully")
	})

	t.Run("returns 400 with invalid OTP", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com"},
		}

		otpStore := NewStubOTPStore()
		_ = otpStore.CreateOTP(1, "123456", time.Now().Add(30*time.Minute))

		handler := &auth.Handler{
			Store:    store,
			OTPStore: otpStore,
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "john@example.com",
			"code": "wrong-code",
			"new_password": "newpassword123"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.ForgotPasswordHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusBadRequest)
	})

	t.Run("returns 500 when update fails", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com"},
		}
		store.ShouldFailUpdate = true

		otpStore := NewStubOTPStore()
		_ = otpStore.CreateOTP(1, "123456", time.Now().Add(30*time.Minute))

		handler := &auth.Handler{
			Store:    store,
			OTPStore: otpStore,
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "john@example.com",
			"code": "123456",
			"new_password": "newpassword123"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.ForgotPasswordHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusInternalServerError)
	})
}

// ============================================================================
// ResetPasswordHandler Tests
// ============================================================================

func TestResetPasswordHandler(t *testing.T) {
	t.Run("successfully resets password for authenticated user", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{
				ID:           1,
				Email:        "john@example.com",
				PasswordHash: pgtype.Text{String: "oldpassword", Valid: true},
			},
		}

		handler := &auth.Handler{
			Store:    store,
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"old_password": "oldpassword",
			"new_password": "newpassword123"
		}`)

		ctx := context.WithValue(context.Background(), "email", "john@example.com")
		req := httptest.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewBuffer(data))
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ResetPasswordHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		assertResponseCode(t, rec.Code, http.StatusOK)
		assertResponseStatus(t, got, "Success")
		assertResponseMessage(t, got, "Password has been reset successfully")
	})
	t.Run("returns 401 when no email in context", func(t *testing.T) {
		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
		"old_password": "oldpassword",
		"new_password": "newpassword123"
	}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.ResetPasswordHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusUnauthorized)
	})

	t.Run("returns 401 when old password doesn't match", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{
				ID:           1,
				Email:        "john@example.com",
				PasswordHash: pgtype.Text{String: "correctpassword", Valid: true},
			},
		}

		handler := &auth.Handler{
			Store:    store,
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
		"old_password": "wrongpassword",
		"new_password": "newpassword123"
	}`)

		ctx := context.WithValue(context.Background(), "email", "john@example.com")
		req := httptest.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewBuffer(data))
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ResetPasswordHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusUnauthorized)
	})

	t.Run("returns 401 when user not found", func(t *testing.T) {
		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
		"old_password": "oldpassword",
		"new_password": "newpassword123"
	}`)

		ctx := context.WithValue(context.Background(), "email", "nonexistent@example.com")
		req := httptest.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewBuffer(data))
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ResetPasswordHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusUnauthorized)
	})

	t.Run("returns 500 when update fails", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{
				ID:           1,
				Email:        "john@example.com",
				PasswordHash: pgtype.Text{String: "oldpassword", Valid: true},
			},
		}
		store.ShouldFailUpdate = true

		handler := &auth.Handler{
			Store:    store,
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
		"old_password": "oldpassword",
		"new_password": "newpassword123"
	}`)

		ctx := context.WithValue(context.Background(), "email", "john@example.com")
		req := httptest.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewBuffer(data))
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ResetPasswordHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusInternalServerError)
	})
}
