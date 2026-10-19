package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/igetayor/recourse/internal/auth"
	"github.com/igetayor/recourse/internal/models"
)

// RoleHierarchy defines inheritance levels (higher roles possess lower role capabilities).
var RoleHierarchy = map[models.Role]int{
	models.RoleAdmin:   100,
	models.RoleAnalyst: 50,
	models.RoleAuditor: 30,
	models.RoleUser:    10,
}

// HasRole checks if the candidate role satisfies any of the target roles.
func HasRole(userRole models.Role, targetRoles ...models.Role) bool {
	userLevel, ok := RoleHierarchy[userRole]
	if !ok {
		return false
	}

	for _, target := range targetRoles {
		targetLevel := RoleHierarchy[target]
		// Direct match or Admin super-override
		if userRole == target || userRole == models.RoleAdmin || userLevel >= targetLevel {
			return true
		}
	}
	return false
}

// EnforceRole checks context claims against permitted roles, returning the claims or an authorization error.
func EnforceRole(ctx context.Context, permittedRoles ...models.Role) (*models.AuthClaims, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok || claims == nil {
		return nil, fmt.Errorf("unauthenticated: authentication token required")
	}

	for _, permitted := range permittedRoles {
		if claims.Role == permitted || claims.Role == models.RoleAdmin {
			return claims, nil
		}
	}

	return nil, fmt.Errorf("forbidden: role %s does not have permission (requires one of %v)", claims.Role, permittedRoles)
}

// RequireRoles HTTP middleware for REST endpoints enforcing RBAC authorization.
func RequireRoles(permittedRoles ...models.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok || claims == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error":  "Authentication required",
					"status": http.StatusUnauthorized,
				})
				return
			}

			if !HasRole(claims.Role, permittedRoles...) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error":           fmt.Sprintf("Insufficient privileges for role %s", claims.Role),
					"permitted_roles": permittedRoles,
					"status":          http.StatusForbidden,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
