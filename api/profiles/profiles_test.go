package profiles_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Adedunmol/answerly/api/profiles"
	"github.com/Adedunmol/answerly/api/tokens"
	"github.com/Adedunmol/answerly/database"
	"github.com/jackc/pgx/v5/pgtype"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ============================================================================
// Stub Profile Store
// ============================================================================

type StubProfileStore struct {
	Profiles         map[int64]database.Profile
	ShouldFailGet    bool
	ShouldFailUpdate bool
	ShouldFailCreate bool
}

func NewStubProfileStore() *StubProfileStore {
	return &StubProfileStore{
		Profiles: make(map[int64]database.Profile),
	}
}

func (s *StubProfileStore) CreateProfile(ctx context.Context, userID int64) error {
	if s.ShouldFailCreate {
		return errors.New("database error")
	}

	s.Profiles[userID] = database.Profile{
		ID:     int64(len(s.Profiles) + 1),
		UserID: userID,
	}
	return nil
}

func (s *StubProfileStore) GetProfile(ctx context.Context, userID int64) (database.Profile, error) {
	if s.ShouldFailGet {
		return database.Profile{}, errors.New("database error")
	}

	profile, exists := s.Profiles[userID]
	if !exists {
		return database.Profile{}, errors.New("profile not found")
	}

	return profile, nil
}

func (s *StubProfileStore) UpdateProfile(ctx context.Context, userID int64, data profiles.UpdateProfileBody) (database.Profile, error) {
	if s.ShouldFailUpdate {
		return database.Profile{}, errors.New("database error")
	}

	profile, exists := s.Profiles[userID]
	if !exists {
		return database.Profile{}, errors.New("profile not found")
	}

	// Update fields if provided
	if data.FirstName != "" {
		profile.FirstName = pgtype.Text{String: data.FirstName, Valid: true}
	}
	if data.LastName != "" {
		profile.LastName = pgtype.Text{String: data.LastName, Valid: true}
	}
	if !data.DateOfBirth.IsZero() {
		profile.DateOfBirth = pgtype.Date{Time: data.DateOfBirth, Valid: true}
	}

	//var gender database.NullGender
	//
	//gender = database.NullGender{
	//	Gender: database.Gender(profile.Gender),
	//	Valid:  true,
	//}
	//
	//if data.Gender != "" {
	//	profile.Gender = pgtype.Text{String: data.Gender, Valid: true}
	//}
	if data.University != "" {
		profile.University = pgtype.Text{String: data.University, Valid: true}
	}
	if data.Faculty != "" {
		profile.Faculty = pgtype.Text{String: data.Faculty, Valid: true}
	}
	if data.Location != "" {
		profile.Location = pgtype.Text{String: data.Location, Valid: true}
	}

	s.Profiles[userID] = profile
	return profile, nil
}

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
// GetProfileHandler Tests
// ============================================================================

func TestGetProfileHandler(t *testing.T) {
	t.Run("successfully retrieves user profile", func(t *testing.T) {
		store := NewStubProfileStore()
		store.Profiles[1] = database.Profile{
			ID:        1,
			UserID:    1,
			FirstName: pgtype.Text{String: "John", Valid: true},
			LastName:  pgtype.Text{String: "Doe", Valid: true},
		}

		handler := &profiles.Handler{
			Store: store,
		}

		req := httptest.NewRequest(http.MethodGet, "/profile", nil)
		rec := httptest.NewRecorder()

		// Add claims to context
		claims := &tokens.Claims{
			UserID: 1,
			Email:  "john@example.com",
		}
		ctx := context.WithValue(req.Context(), "claims", claims)
		req = req.WithContext(ctx)

		handler.GetProfileHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusOK)
		assertResponseStatus(t, got, "success")
		assertResponseMessage(t, got, "retrieved user's profile successfully")

		if got["data"] == nil {
			t.Error("expected profile data in response")
		}
	})

	t.Run("returns 401 when userID is 0", func(t *testing.T) {
		handler := &profiles.Handler{
			Store: NewStubProfileStore(),
		}

		req := httptest.NewRequest(http.MethodGet, "/profile", nil)
		rec := httptest.NewRecorder()

		// Add claims with userID = 0
		claims := &tokens.Claims{
			UserID: 0,
		}
		ctx := context.WithValue(req.Context(), "claims", claims)
		req = req.WithContext(ctx)

		handler.GetProfileHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusUnauthorized)
		assertResponseStatus(t, got, "error")
		assertResponseMessage(t, got, "unauthorized")
	})

	t.Run("returns 404 when profile not found", func(t *testing.T) {
		store := NewStubProfileStore()

		handler := &profiles.Handler{
			Store: store,
		}

		req := httptest.NewRequest(http.MethodGet, "/profile", nil)
		rec := httptest.NewRecorder()

		claims := &tokens.Claims{
			UserID: 999,
			Email:  "notfound@example.com",
		}
		ctx := context.WithValue(req.Context(), "claims", claims)
		req = req.WithContext(ctx)

		handler.GetProfileHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusNotFound)
		assertResponseStatus(t, got, "error")
	})

	t.Run("returns 404 when database error occurs", func(t *testing.T) {
		store := NewStubProfileStore()
		store.ShouldFailGet = true

		handler := &profiles.Handler{
			Store: store,
		}

		req := httptest.NewRequest(http.MethodGet, "/profile", nil)
		rec := httptest.NewRecorder()

		claims := &tokens.Claims{
			UserID: 1,
			Email:  "john@example.com",
		}
		ctx := context.WithValue(req.Context(), "claims", claims)
		req = req.WithContext(ctx)

		handler.GetProfileHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusNotFound)
	})
}

// ============================================================================
// UpdateProfileHandler Tests
// ============================================================================

func TestUpdateProfileHandler(t *testing.T) {
	t.Run("successfully updates user profile", func(t *testing.T) {
		store := NewStubProfileStore()
		store.Profiles[1] = database.Profile{
			ID:        1,
			UserID:    1,
			FirstName: pgtype.Text{String: "John", Valid: true},
			LastName:  pgtype.Text{String: "Doe", Valid: true},
		}

		handler := &profiles.Handler{
			Store: store,
		}

		data := []byte(`{
			"first_name": "Jane",
			"last_name": "Smith",
			"gender": "Female",
			"university": "MIT",
			"faculty": "Computer Science",
			"location": "Boston, MA"
		}`)

		req := httptest.NewRequest(http.MethodPatch, "/profile", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		claims := &tokens.Claims{
			UserID: 1,
			Email:  "john@example.com",
		}
		ctx := context.WithValue(req.Context(), "claims", claims)
		req = req.WithContext(ctx)

		handler.UpdateProfileHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusOK)
		assertResponseStatus(t, got, "success")
		assertResponseMessage(t, got, "updated user's profile successfully")

		if got["data"] == nil {
			t.Error("expected profile data in response")
		}

		// Verify profile was actually updated in store
		updatedProfile := store.Profiles[1]
		if updatedProfile.FirstName.String != "Jane" {
			t.Errorf("expected FirstName to be 'Jane', got '%s'", updatedProfile.FirstName.String)
		}
		if updatedProfile.University.String != "MIT" {
			t.Errorf("expected University to be 'MIT', got '%s'", updatedProfile.University.String)
		}
		if updatedProfile.Location.String != "Boston, MA" {
			t.Errorf("expected Location to be 'Boston, MA', got '%s'", updatedProfile.Location.String)
		}
	})

	t.Run("returns 401 when userID is 0", func(t *testing.T) {
		handler := &profiles.Handler{
			Store: NewStubProfileStore(),
		}

		data := []byte(`{"first_name": "John"}`)

		req := httptest.NewRequest(http.MethodPatch, "/profile", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		claims := &tokens.Claims{
			UserID: 0,
		}
		ctx := context.WithValue(req.Context(), "claims", claims)
		req = req.WithContext(ctx)

		handler.UpdateProfileHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusUnauthorized)
		assertResponseStatus(t, got, "error")
		assertResponseMessage(t, got, "unauthorized")
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		handler := &profiles.Handler{
			Store: NewStubProfileStore(),
		}

		data := []byte(`{"first_name": "John"`) // Invalid JSON

		req := httptest.NewRequest(http.MethodPatch, "/profile", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		claims := &tokens.Claims{
			UserID: 1,
			Email:  "john@example.com",
		}
		ctx := context.WithValue(req.Context(), "claims", claims)
		req = req.WithContext(ctx)

		handler.UpdateProfileHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusBadRequest)
		assertResponseStatus(t, got, "error")
	})

	t.Run("returns 404 when profile not found", func(t *testing.T) {
		store := NewStubProfileStore()

		handler := &profiles.Handler{
			Store: store,
		}

		data := []byte(`{"first_name": "John"}`)

		req := httptest.NewRequest(http.MethodPatch, "/profile", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		claims := &tokens.Claims{
			UserID: 999,
			Email:  "notfound@example.com",
		}
		ctx := context.WithValue(req.Context(), "claims", claims)
		req = req.WithContext(ctx)

		handler.UpdateProfileHandler(rec, req)

		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)

		assertResponseCode(t, rec.Code, http.StatusNotFound)
		assertResponseStatus(t, got, "error")
	})

	t.Run("returns 404 when database error occurs", func(t *testing.T) {
		store := NewStubProfileStore()
		store.ShouldFailUpdate = true
		store.Profiles[1] = database.Profile{
			ID:     1,
			UserID: 1,
		}

		handler := &profiles.Handler{
			Store: store,
		}

		data := []byte(`{"first_name": "John"}`)

		req := httptest.NewRequest(http.MethodPatch, "/profile", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		claims := &tokens.Claims{
			UserID: 1,
			Email:  "john@example.com",
		}
		ctx := context.WithValue(req.Context(), "claims", claims)
		req = req.WithContext(ctx)

		handler.UpdateProfileHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusNotFound)
	})

	t.Run("updates only provided fields", func(t *testing.T) {
		store := NewStubProfileStore()
		store.Profiles[1] = database.Profile{
			ID:         1,
			UserID:     1,
			FirstName:  pgtype.Text{String: "John", Valid: true},
			LastName:   pgtype.Text{String: "Doe", Valid: true},
			University: pgtype.Text{String: "Harvard", Valid: true},
		}

		handler := &profiles.Handler{
			Store: store,
		}

		// Only update location
		data := []byte(`{"location": "New York, NY"}`)

		req := httptest.NewRequest(http.MethodPatch, "/profile", bytes.NewBuffer(data))
		rec := httptest.NewRecorder()

		claims := &tokens.Claims{
			UserID: 1,
			Email:  "john@example.com",
		}
		ctx := context.WithValue(req.Context(), "claims", claims)
		req = req.WithContext(ctx)

		handler.UpdateProfileHandler(rec, req)

		assertResponseCode(t, rec.Code, http.StatusOK)

		// Verify only location was updated
		updatedProfile := store.Profiles[1]
		if updatedProfile.FirstName.String != "John" {
			t.Errorf("expected FirstName to remain 'John', got '%s'", updatedProfile.FirstName.String)
		}
		if updatedProfile.LastName.String != "Doe" {
			t.Errorf("expected LastName to remain 'Doe', got '%s'", updatedProfile.LastName.String)
		}
		if updatedProfile.University.String != "Harvard" {
			t.Errorf("expected University to remain 'Harvard', got '%s'", updatedProfile.University.String)
		}
		if updatedProfile.Location.String != "New York, NY" {
			t.Errorf("expected Location to be 'New York, NY', got '%s'", updatedProfile.Location.String)
		}
	})
}
