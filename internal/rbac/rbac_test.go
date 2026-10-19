package rbac_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/igetayor/recourse/internal/auth"
	"github.com/igetayor/recourse/internal/config"
	"github.com/igetayor/recourse/internal/models"
	"github.com/igetayor/recourse/internal/rbac"
	"time"
)

func contextWithRole(role models.Role) context.Context {
	cfg := &config.Config{
		JWTSecret:     "rbac-test-secret-key-2026",
		JWTExpiration: 1 * time.Hour,
		ServiceName:   "rbac-test",
	}
	svc := auth.NewJWTService(cfg)
	user := &models.User{ID: "test-user", Email: "test@example.com", Role: role}
	token, _ := svc.GenerateToken(user)
	claims, _ := svc.ValidateToken(token)
	return context.WithValue(context.Background(), auth.ClaimsContextKey, claims)
}

func TestHasRole_DirectMatch(t *testing.T) {
	if !rbac.HasRole(models.RoleAdmin, models.RoleAdmin) {
		t.Error("Admin should match Admin")
	}
	if !rbac.HasRole(models.RoleAnalyst, models.RoleAnalyst) {
		t.Error("Analyst should match Analyst")
	}
}

func TestHasRole_AdminSuperpower(t *testing.T) {
	// Admin should satisfy all roles
	for _, role := range []models.Role{models.RoleUser, models.RoleAuditor, models.RoleAnalyst, models.RoleAdmin} {
		if !rbac.HasRole(models.RoleAdmin, role) {
			t.Errorf("Admin should supersede role %s", role)
		}
	}
}

func TestHasRole_InsufficientPrivilege(t *testing.T) {
	if rbac.HasRole(models.RoleUser, models.RoleAdmin) {
		t.Error("USER should not satisfy ADMIN role")
	}
	if rbac.HasRole(models.RoleAuditor, models.RoleAnalyst) {
		t.Error("AUDITOR should not satisfy ANALYST role")
	}
}

func TestEnforceRole_Authenticated(t *testing.T) {
	ctx := contextWithRole(models.RoleAnalyst)
	_, err := rbac.EnforceRole(ctx, models.RoleAnalyst)
	if err != nil {
		t.Errorf("EnforceRole: unexpected error for analyst: %v", err)
	}
}

func TestEnforceRole_Unauthenticated(t *testing.T) {
	_, err := rbac.EnforceRole(context.Background(), models.RoleUser)
	if err == nil {
		t.Error("expected unauthenticated error for empty context")
	}
}

func TestEnforceRole_Forbidden(t *testing.T) {
	ctx := contextWithRole(models.RoleUser)
	_, err := rbac.EnforceRole(ctx, models.RoleAdmin)
	if err == nil {
		t.Error("expected forbidden error for USER attempting ADMIN operation")
	}
}

func TestRequireRoles_Middleware(t *testing.T) {
	tests := []struct {
		name           string
		role           models.Role
		requiredRoles  []models.Role
		expectedStatus int
	}{
		{"admin-passes", models.RoleAdmin, []models.Role{models.RoleAdmin}, http.StatusOK},
		{"analyst-passes-analyst", models.RoleAnalyst, []models.Role{models.RoleAnalyst}, http.StatusOK},
		{"user-fails-admin", models.RoleUser, []models.Role{models.RoleAdmin}, http.StatusForbidden},
		{"auditor-fails-analyst", models.RoleAuditor, []models.Role{models.RoleAnalyst}, http.StatusForbidden},
		{"admin-passes-any", models.RoleAdmin, []models.Role{models.RoleAuditor}, http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := contextWithRole(tc.role)
			handler := rbac.RequireRoles(tc.requiredRoles...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("status: got %d, want %d", rec.Code, tc.expectedStatus)
			}
		})
	}
}
