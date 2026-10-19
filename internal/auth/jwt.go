package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/igetayor/recourse/internal/config"
	"github.com/igetayor/recourse/internal/models"
)

type contextKey string

const (
	ClaimsContextKey contextKey = "auth_claims"
)

var (
	ErrInvalidToken = errors.New("invalid or expired authentication token")
	ErrMissingAuth  = errors.New("authorization header is required")
)

// JWTService handles token generation, claims verification, and parsing.
type JWTService struct {
	secret     []byte
	expiration time.Duration
	issuer     string
}

// NewJWTService builds a new JWT token service.
func NewJWTService(cfg *config.Config) *JWTService {
	return &JWTService{
		secret:     []byte(cfg.JWTSecret),
		expiration: cfg.JWTExpiration,
		issuer:     cfg.ServiceName,
	}
}

// GenerateToken generates a signed JWT token for a user with role and permissions.
func (s *JWTService) GenerateToken(user *models.User) (string, error) {
	now := time.Now()
	claims := models.AuthClaims{
		UserID:      user.ID,
		Email:       user.Email,
		Role:        user.Role,
		Permissions: permissionsForRole(user.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   user.ID,
			Audience:  jwt.ClaimStrings{"recourse-api"},
			ExpiresAt: jwt.NewNumericDate(now.Add(s.expiration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ValidateToken parses, validates signature, and verifies claims expiration.
func (s *JWTService) ValidateToken(tokenStr string) (*models.AuthClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &models.AuthClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	}, jwt.WithIssuer(s.issuer), jwt.WithAudience("recourse-api"))

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*models.AuthClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func permissionsForRole(role models.Role) []string {
	switch role {
	case models.RoleAdmin:
		return []string{"disputes:read", "disputes:write", "disputes:resolve", "disputes:delete", "users:read", "users:manage"}
	case models.RoleAnalyst:
		return []string{"disputes:read", "disputes:write", "disputes:resolve", "users:read"}
	case models.RoleAuditor:
		return []string{"disputes:read", "users:read"}
	default:
		return []string{"disputes:read", "disputes:write"}
	}
}

// Middleware extracts and validates JWT from Authorization header and injects claims into Context.
func Middleware(jwtService *JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// Allow request to proceed unauthenticated; resolvers/handlers enforce requirements
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				renderJSONError(w, http.StatusUnauthorized, "Malformed authorization header. Expected 'Bearer <token>'")
				return
			}

			claims, err := jwtService.ValidateToken(parts[1])
			if err != nil {
				renderJSONError(w, http.StatusUnauthorized, "Invalid, expired, or untrusted authentication token")
				return
			}

			// Inject claims into request context
			ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext extracts AuthClaims from context.
func ClaimsFromContext(ctx context.Context) (*models.AuthClaims, bool) {
	if ctx == nil {
		return nil, false
	}
	claims, ok := ctx.Value(ClaimsContextKey).(*models.AuthClaims)
	return claims, ok && claims != nil
}

func renderJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":  msg,
		"status": status,
	})
}
