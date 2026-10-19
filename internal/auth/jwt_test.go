package auth_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/igetayor/recourse/internal/auth"
	"github.com/igetayor/recourse/internal/config"
	"github.com/igetayor/recourse/internal/models"
)

func testConfig() *config.Config {
	return &config.Config{
		JWTSecret:     "test-super-secret-key-for-unit-tests-2026",
		JWTExpiration: 1 * time.Hour,
		ServiceName:   "recourse-test",
	}
}

func testUser() *models.User {
	return &models.User{
		ID:    "user-123",
		Email: "alice@recourse.io",
		Role:  models.RoleUser,
	}
}

func TestJWTService_GenerateAndValidate(t *testing.T) {
	svc := auth.NewJWTService(testConfig())
	user := testUser()

	token, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken: unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("GenerateToken: returned empty token")
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: unexpected error: %v", err)
	}
	if claims.UserID != user.ID {
		t.Errorf("claims.UserID: got %q, want %q", claims.UserID, user.ID)
	}
	if claims.Email != user.Email {
		t.Errorf("claims.Email: got %q, want %q", claims.Email, user.Email)
	}
	if claims.Role != user.Role {
		t.Errorf("claims.Role: got %q, want %q", claims.Role, user.Role)
	}
}

func TestJWTService_AdminPermissions(t *testing.T) {
	svc := auth.NewJWTService(testConfig())
	admin := &models.User{ID: "admin-1", Email: "admin@recourse.io", Role: models.RoleAdmin}

	token, _ := svc.GenerateToken(admin)
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	requiredPerms := []string{"disputes:read", "disputes:write", "disputes:resolve", "disputes:delete", "users:manage"}
	permSet := make(map[string]bool, len(claims.Permissions))
	for _, p := range claims.Permissions {
		permSet[p] = true
	}
	for _, req := range requiredPerms {
		if !permSet[req] {
			t.Errorf("admin missing permission: %q", req)
		}
	}
}

func TestJWTService_InvalidToken(t *testing.T) {
	svc := auth.NewJWTService(testConfig())

	_, err := svc.ValidateToken("this.is.not.valid")
	if err == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
}

func TestJWTService_WrongSecret(t *testing.T) {
	svc1 := auth.NewJWTService(testConfig())
	cfg2 := testConfig()
	cfg2.JWTSecret = "a-completely-different-secret"
	svc2 := auth.NewJWTService(cfg2)

	token, _ := svc1.GenerateToken(testUser())
	_, err := svc2.ValidateToken(token)
	if err == nil {
		t.Fatal("expected error validating token with wrong secret")
	}
}

func TestAuthMiddleware_NoHeader(t *testing.T) {
	svc := auth.NewJWTService(testConfig())
	handler := auth.Middleware(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should pass through without claims
		_, ok := auth.ClaimsFromContext(r.Context())
		if ok {
			w.WriteHeader(http.StatusBadRequest) // Should not have claims
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	svc := auth.NewJWTService(testConfig())
	user := testUser()
	token, _ := svc.GenerateToken(user)

	handler := auth.Middleware(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if claims.UserID != user.ID {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_MalformedHeader(t *testing.T) {
	svc := auth.NewJWTService(testConfig())

	handler := auth.Middleware(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, badHeader := range []string{"Bearer", "Token abc123", "justaplainstring"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", badHeader)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusOK && strings.Contains(badHeader, " ") {
			// "Token abc123" should be rejected (not Bearer)
			if strings.HasPrefix(badHeader, "Token") {
				if rec.Code != http.StatusUnauthorized {
					t.Errorf("header %q: expected 401, got %d", badHeader, rec.Code)
				}
			}
		}
	}
}
