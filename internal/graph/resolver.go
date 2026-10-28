package graph

import (
	"context"

	"github.com/igetayor/recourse/internal/auth"
	"github.com/igetayor/recourse/internal/config"
	"github.com/igetayor/recourse/internal/db"
)

// ─────────────────────────── GQL Output Types ───────────────────────────

// Role is the GraphQL Role enum.
type Role string

const (
	RoleAdmin   Role = "ADMIN"
	RoleAnalyst Role = "ANALYST"
	RoleAuditor Role = "AUDITOR"
	RoleUser    Role = "USER"
)

// DisputeStatus is the GraphQL DisputeStatus enum.
type DisputeStatus string

const (
	DisputeStatusOpen        DisputeStatus = "OPEN"
	DisputeStatusUnderReview DisputeStatus = "UNDER_REVIEW"
	DisputeStatusResolved    DisputeStatus = "RESOLVED"
	DisputeStatusRejected    DisputeStatus = "REJECTED"
)

// User is the GraphQL User type.
type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FullName  string `json:"fullName"`
	Role      Role   `json:"role"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// Dispute is the GraphQL Dispute type.
type Dispute struct {
	ID              string        `json:"id"`
	Title           string        `json:"title"`
	Description     string        `json:"description"`
	Amount          float64       `json:"amount"`
	Currency        string        `json:"currency"`
	Status          DisputeStatus `json:"status"`
	CreatedBy       *User         `json:"createdBy"`
	Assignee        *User         `json:"assignee,omitempty"`
	ResolutionNotes *string       `json:"resolutionNotes,omitempty"`
	CreatedAt       string        `json:"createdAt"`
	UpdatedAt       string        `json:"updatedAt"`
}

// AuthPayload is returned on successful authentication.
type AuthPayload struct {
	Token     string `json:"token"`
	User      *User  `json:"user"`
	ExpiresIn int    `json:"expiresIn"`
}

// ─────────────────────────── GQL Input Types ───────────────────────────

// RegisterInput is the input for the register mutation.
type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"fullName"`
	Role     *Role  `json:"role,omitempty"`
}

// LoginInput is the input for the login mutation.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// CreateDisputeInput is the input for creating a dispute.
type CreateDisputeInput struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
}

// UpdateDisputeInput is the input for updating a dispute.
type UpdateDisputeInput struct {
	Status          DisputeStatus `json:"status"`
	ResolutionNotes *string       `json:"resolutionNotes,omitempty"`
	AssigneeID      *string       `json:"assigneeId,omitempty"`
}

// ─────────────────────────── Resolver Infrastructure ───────────────────────────

// Resolver is the root resolver holding shared service dependencies.
type Resolver struct {
	UserRepo    db.UserRepo
	DisputeRepo db.DisputeRepo
	JWTService  *auth.JWTService
	Cfg         *config.Config
}

// Query returns the QueryResolver implementation.
func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

// Mutation returns the MutationResolver implementation.
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }

// queryResolver implements QueryResolver.
type queryResolver struct{ *Resolver }

// mutationResolver implements MutationResolver.
type mutationResolver struct{ *Resolver }

// ─────────────────────────── Resolver Interfaces ───────────────────────────

// QueryResolver defines all read operations.
type QueryResolver interface {
	Me(ctx context.Context) (*User, error)
	Users(ctx context.Context) ([]*User, error)
	Dispute(ctx context.Context, id string) (*Dispute, error)
	Disputes(ctx context.Context, status *DisputeStatus) ([]*Dispute, error)
}

// MutationResolver defines all write operations.
type MutationResolver interface {
	Register(ctx context.Context, input RegisterInput) (*AuthPayload, error)
	Login(ctx context.Context, input LoginInput) (*AuthPayload, error)
	CreateDispute(ctx context.Context, input CreateDisputeInput) (*Dispute, error)
	UpdateDispute(ctx context.Context, id string, input UpdateDisputeInput) (*Dispute, error)
	DeleteDispute(ctx context.Context, id string) (bool, error)
	UpdateUserRole(ctx context.Context, userID string, role Role) (*User, error)
}
