// Package controllers contains HTTP adapter layer for Recourse REST API endpoints.
package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/igetayor/recourse/internal/api/services"
	"github.com/igetayor/recourse/internal/graph"
)

// DisputeController handles REST endpoints for dispute inspection and management.
type DisputeController struct {
	service *services.DisputeService
}

// NewDisputeController creates a new DisputeController.
func NewDisputeController(s *services.DisputeService) *DisputeController {
	return &DisputeController{service: s}
}

// HandleListDisputes returns disputes filtered by optional status query param.
func (c *DisputeController) HandleListDisputes(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	resp := c.service.ListDisputes(r.Context(), statusFilter)
	writeGQLResponse(w, resp)
}

// HandleGetDispute returns a single dispute by chi URL param :id.
func (c *DisputeController) HandleGetDispute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	resp := c.service.GetDispute(r.Context(), id)
	writeGQLResponse(w, resp)
}

// AuthController handles authentication REST endpoints (register, login).
type AuthController struct {
	service *services.AuthService
}

// NewAuthController creates a new AuthController.
func NewAuthController(s *services.AuthService) *AuthController {
	return &AuthController{service: s}
}

// HandleRegister creates a new user account via REST → GraphQL mutation.
func (c *AuthController) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		FullName string `json:"fullName"`
		Role     string `json:"role,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp := c.service.Register(r.Context(), body.Email, body.Password, body.FullName, body.Role)
	writeGQLResponse(w, resp)
}

// HandleLogin authenticates a user and returns a JWT token.
func (c *AuthController) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp := c.service.Login(r.Context(), body.Email, body.Password)
	writeGQLResponse(w, resp)
}

// ─── Shared HTTP response utilities ───────────────────────────────────────────

func writeGQLResponse(w http.ResponseWriter, resp *graph.Response) {
	w.Header().Set("Content-Type", "application/json")
	if len(resp.Errors) > 0 {
		msg := resp.Errors[0].Message
		status := http.StatusBadRequest
		sl := strings.ToLower(msg)
		if strings.Contains(sl, "unauthenticated") || strings.Contains(sl, "authentication") {
			status = http.StatusUnauthorized
		} else if strings.Contains(sl, "forbidden") || strings.Contains(sl, "permission") || strings.Contains(sl, "role") {
			status = http.StatusForbidden
		} else if strings.Contains(sl, "not found") {
			status = http.StatusNotFound
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
		return
	}
	_ = json.NewEncoder(w).Encode(resp.Data)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}
