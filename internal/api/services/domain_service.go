// Package services contains domain orchestration for auth, disputes, and GraphQL resolution.
package services

import (
	"context"

	"github.com/igetayor/recourse/internal/graph"
)

// DisputeService orchestrates dispute-related business logic through the GraphQL schema.
type DisputeService struct {
	schema *graph.Schema
}

// NewDisputeService creates a new DisputeService instance.
func NewDisputeService(schema *graph.Schema) *DisputeService {
	return &DisputeService{schema: schema}
}

// ListDisputes executes a filtered disputes query.
func (s *DisputeService) ListDisputes(ctx context.Context, statusFilter string) *graph.Response {
	vars := map[string]any{}
	if statusFilter != "" {
		vars["status"] = statusFilter
	}
	req := &graph.Request{
		OperationName: "Disputes",
		Query:         "query Disputes($status: DisputeStatus) { disputes(status: $status) { id title amount currency status createdAt } }",
		Variables:     vars,
	}
	return s.schema.Execute(ctx, req)
}

// GetDispute fetches a single dispute by ID.
func (s *DisputeService) GetDispute(ctx context.Context, id string) *graph.Response {
	req := &graph.Request{
		OperationName: "Dispute",
		Query:         "query Dispute($id: ID!) { dispute(id: $id) { id title description amount currency status resolutionNotes createdAt updatedAt createdBy { id email fullName } assignee { id email fullName } } }",
		Variables:     map[string]any{"id": id},
	}
	return s.schema.Execute(ctx, req)
}

// AuthService handles authentication and user registration operations.
type AuthService struct {
	schema *graph.Schema
}

// NewAuthService creates a new AuthService instance.
func NewAuthService(schema *graph.Schema) *AuthService {
	return &AuthService{schema: schema}
}

// Register creates a new user account.
func (s *AuthService) Register(ctx context.Context, email, password, fullName, role string) *graph.Response {
	req := &graph.Request{
		OperationName: "Register",
		Query:         "mutation Register($input: RegisterInput!) { register(input: $input) { token expiresIn user { id email fullName role } } }",
		Variables: map[string]any{"input": map[string]any{
			"email":    email,
			"password": password,
			"fullName": fullName,
			"role":     role,
		}},
	}
	return s.schema.Execute(ctx, req)
}

// Login authenticates a user and returns a JWT token.
func (s *AuthService) Login(ctx context.Context, email, password string) *graph.Response {
	req := &graph.Request{
		OperationName: "Login",
		Query:         "mutation Login($input: LoginInput!) { login(input: $input) { token expiresIn user { id email fullName role } } }",
		Variables:     map[string]any{"input": map[string]any{"email": email, "password": password}},
	}
	return s.schema.Execute(ctx, req)
}
