package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Adedunmol/answerly/api/auth"
	"github.com/Adedunmol/answerly/api/custom_errors"
	"github.com/Adedunmol/answerly/api/tokens"
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
	return "mock-jwt-token", "mock-refresh-token"
}

func (s *StubTokenService) DecodeToken(tokenString string) (*tokens.Claims, error) {
	if tokenString == "invalid-token" {
		return nil, errors.New("invalid token")
	}

	return &tokens.Claims{
		UserID: 1,
		Email:  "test@example.com",
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
	OTPs               map[string]string // key: userID-domain, value: otp
	Expirations        map[string]time.Time
	ShouldFailCreate   bool
	ShouldFailDelete   bool
	ShouldFailValidate bool
}

func NewStubOTPStore() *StubOTPStore {
	return &StubOTPStore{
		OTPs:        make(map[string]string),
		Expirations: make(map[string]time.Time),
	}
}

func (s *StubOTPStore) CreateOTP(ctx context.Context, userID int64, code string, expiration time.Time, domain string) error {
	if s.ShouldFailCreate {
		return errors.New("failed to create OTP")
	}

	key := s.makeKey(userID, domain)
	s.OTPs[key] = code
	s.Expirations[key] = expiration
	return nil
}

func (s *StubOTPStore) GetOTP(ctx context.Context, userID int64, domain string) (string, error) {
	if s.ShouldFailValidate {
		return "", errors.New("validation error")
	}

	key := s.makeKey(userID, domain)
	otp, exists := s.OTPs[key]
	if !exists {
		return "", custom_errors.ErrInvalidOTP
	}

	expiration, hasExpiration := s.Expirations[key]
	if hasExpiration && expiration.Before(time.Now()) {
		return "", custom_errors.ErrInvalidOTP
	}

	return otp, nil
}

func (s *StubOTPStore) DeleteOTP(ctx context.Context, userID int64, domain string) error {
	if s.ShouldFailDelete {
		return errors.New("failed to delete OTP")
	}
	key := s.makeKey(userID, domain)
	delete(s.OTPs, key)
	delete(s.Expirations, key)
	return nil
}

func (s *StubOTPStore) makeKey(userID int64, domain string) string {
	return string(rune(userID)) + "-" + domain
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

func (s *StubUserStore) CreateUser(ctx context.Context, body *auth.CreateUserBody) (database.User, error) {
	if s.ShouldFailCreate {
		return database.User{}, errors.New("database error")
	}

	for _, u := range s.Users {
		if u.Email == body.Email {
			return database.User{}, custom_errors.ErrConflict
		}
	}

	user := database.User{
		ID:            int64(len(s.Users) + 1),
		Email:         body.Email,
		Password:      body.Password,
		EmailVerified: pgtype.Bool{Bool: false, Valid: true},
		Role:          body.Role,
	}

	s.Users = append(s.Users, user)

	return user, nil
}

func (s *StubUserStore) FindUserByEmail(ctx context.Context, email string) (database.User, error) {
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

func (s *StubUserStore) FindUserByID(ctx context.Context, id int) (database.User, error) {
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

func (s *StubUserStore) UpdateUser(ctx context.Context, id int, data auth.UpdateUserBody) error {
	if s.ShouldFailUpdate {
		return errors.New("database error")
	}

	for i, u := range s.Users {
		if u.ID == int64(id) {
			if data.Verified {
				s.Users[i].EmailVerified = pgtype.Bool{Bool: data.Verified, Valid: true}
			}
			if data.Password != "" {
				s.Users[i].Password = data.Password
			}
			if data.RefreshToken != "" {
				s.Users[i].RefreshToken = pgtype.Text{String: data.RefreshToken, Valid: true}
			}
			return nil
		}
	}

	return errors.New("user not found")
}

func (s *StubUserStore) FindUserWithRefreshToken(ctx context.Context, refreshToken string) (database.User, error) {
	if s.ShouldFailFind {
		return database.User{}, errors.New("database error")
	}

	for _, u := range s.Users {
		if u.RefreshToken.Valid && u.RefreshToken.String == refreshToken {
			return u, nil
		}
	}
	return database.User{}, errors.New("user not found")
}

func (s *StubUserStore) DeleteRefreshToken(ctx context.Context, refreshToken string) error {
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

func (s *StubUserStore) UpdateRefreshToken(ctx context.Context, oldRefreshToken, refreshToken string) error {
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
			"email": "john@example.com",
			"password": "password123",
			"password_confirmation": "password123"
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
			"email": "john@example.com",
			"password": "password123",
			"password_confirmation": "password123"
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
			"email": "john@example.com",
			"password": "password123",
			"password_confirmation": "password123"
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
			"email": "john@example.com",
			"password": "password123",
			"password_confirmation": "password123"
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
		store.Users = []database.User{
			{
				ID:            1,
				Email:         "john@example.com",
				Password:      "hashedpassword",
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
				ID:       1,
				Email:    "john@example.com",
				Password: "correctpassword",
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

	t.Run("returns 500 when update user fails", func(t *testing.T) {
		store := NewStubUserStore()
		store.Users = []database.User{
			{
				ID:            1,
				Email:         "john@example.com",
				Password:      "password123",
				EmailVerified: pgtype.Bool{Bool: true, Valid: true},
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
		ctx := context.Background()
		store := NewStubUserStore()
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com", EmailVerified: pgtype.Bool{Bool: false, Valid: true}},
		}

		otpStore := NewStubOTPStore()
		_ = otpStore.CreateOTP(ctx, 1, "123456", time.Now().Add(30*time.Minute), "verification")

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
		user, _ := store.FindUserByID(ctx, 1)
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
		ctx := context.Background()
		store := NewStubUserStore()
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com"},
		}

		otpStore := NewStubOTPStore()
		_ = otpStore.CreateOTP(ctx, 1, "123456", time.Now().Add(30*time.Minute), "verification")

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
		ctx := context.Background()
		store := NewStubUserStore()
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com"},
		}

		otpStore := NewStubOTPStore()
		_ = otpStore.CreateOTP(ctx, 1, "123456", time.Now().Add(30*time.Minute), "verification")
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
		ctx := context.Background()
		store := NewStubUserStore()
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com"},
		}
		store.ShouldFailUpdate = true

		otpStore := NewStubOTPStore()
		_ = otpStore.CreateOTP(ctx, 1, "123456", time.Now().Add(30*time.Minute), "verification")

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
				ID:           1,
				Email:        "john@example.com",
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
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com"},
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
			{ID: 1, Email: "john@example.com"},
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
		ctx := context.Background()
		store := NewStubUserStore()
		store.Users = []database.User{
			{
				ID:       1,
				Email:    "john@example.com",
				Password: "oldpassword",
			},
		}

		otpStore := NewStubOTPStore()
		_ = otpStore.CreateOTP(ctx, 1, "123456", time.Now().Add(30*time.Minute), "forgot_password")

		handler := &auth.Handler{
			Store:    store,
			OTPStore: otpStore,
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "john@example.com",
			"code": "123456",
			"new_password": "newpassword123",
			"new_password_confirm": "newpassword123"
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
		ctx := context.Background()
		store := NewStubUserStore()
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com"},
		}

		otpStore := NewStubOTPStore()
		_ = otpStore.CreateOTP(ctx, 1, "123456", time.Now().Add(30*time.Minute), "forgot_password")

		handler := &auth.Handler{
			Store:    store,
			OTPStore: otpStore,
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "john@example.com",
			"code": "wrong-code",
			"new_password": "newpassword123",
			"new_password_confirm": "newpassword123"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.ForgotPasswordHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusBadRequest)
	})

	t.Run("returns 500 when update fails", func(t *testing.T) {
		ctx := context.Background()
		store := NewStubUserStore()
		store.Users = []database.User{
			{ID: 1, Email: "john@example.com"},
		}
		store.ShouldFailUpdate = true

		otpStore := NewStubOTPStore()
		_ = otpStore.CreateOTP(ctx, 1, "123456", time.Now().Add(30*time.Minute), "forgot_password")

		handler := &auth.Handler{
			Store:    store,
			OTPStore: otpStore,
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"email": "john@example.com",
			"code": "123456",
			"new_password": "newpassword123",
			"new_password_confirm": "newpassword123"
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
		ctx := context.Background()
		store := NewStubUserStore()
		store.Users = []database.User{
			{
				ID:       1,
				Email:    "john@example.com",
				Password: "oldpassword",
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
			"new_password": "newpassword123",
			"new_password_confirm": "newpassword123"
		}`)

		reqCtx := context.WithValue(ctx, "claims", &tokens.Claims{
			UserID: 1,
			Email:  "john@example.com",
		})
		req := httptest.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewBuffer(data))
		req = req.WithContext(reqCtx)
		rec := httptest.NewRecorder()

		handler.ResetPasswordHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		assertResponseCode(t, rec.Code, http.StatusOK)
		assertResponseStatus(t, got, "Success")
		assertResponseMessage(t, got, "Password has been reset successfully")
	})

	t.Run("returns 401 when no claims in context", func(t *testing.T) {
		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"old_password": "oldpassword",
			"new_password": "newpassword123",
			"new_password_confirm": "newpassword123"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		handler.ResetPasswordHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusUnauthorized)
	})

	t.Run("returns 401 when old password doesn't match", func(t *testing.T) {
		ctx := context.Background()
		store := NewStubUserStore()
		store.Users = []database.User{
			{
				ID:       1,
				Email:    "john@example.com",
				Password: "correctpassword",
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
			"new_password": "newpassword123",
			"new_password_confirm": "newpassword123"
		}`)

		reqCtx := context.WithValue(ctx, "claims", &tokens.Claims{
			UserID: 1,
			Email:  "john@example.com",
		})
		req := httptest.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewBuffer(data))
		req = req.WithContext(reqCtx)
		rec := httptest.NewRecorder()

		handler.ResetPasswordHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusUnauthorized)
	})

	t.Run("returns 401 when user not found", func(t *testing.T) {
		ctx := context.Background()
		handler := &auth.Handler{
			Store:    NewStubUserStore(),
			OTPStore: NewStubOTPStore(),
			Queue:    &StubQueue{},
			Token:    &StubTokenService{},
		}

		data := []byte(`{
			"old_password": "oldpassword",
			"new_password": "newpassword123",
			"new_password_confirm": "newpassword123"
		}`)

		reqCtx := context.WithValue(ctx, "claims", &tokens.Claims{
			UserID: 999,
			Email:  "nonexistent@example.com",
		})
		req := httptest.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewBuffer(data))
		req = req.WithContext(reqCtx)
		rec := httptest.NewRecorder()

		handler.ResetPasswordHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusUnauthorized)
	})

	t.Run("returns 500 when update fails", func(t *testing.T) {
		ctx := context.Background()
		store := NewStubUserStore()
		store.Users = []database.User{
			{
				ID:       1,
				Email:    "john@example.com",
				Password: "oldpassword",
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
			"new_password": "newpassword123",
			"new_password_confirm": "newpassword123"
		}`)

		reqCtx := context.WithValue(ctx, "claims", &tokens.Claims{
			UserID: 1,
			Email:  "john@example.com",
		})
		req := httptest.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewBuffer(data))
		req = req.WithContext(reqCtx)
		rec := httptest.NewRecorder()

		handler.ResetPasswordHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusInternalServerError)
	})
}
