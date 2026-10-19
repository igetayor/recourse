package models

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Role represents user authorization level.
type Role string

const (
	RoleAdmin   Role = "ADMIN"
	RoleAnalyst Role = "ANALYST"
	RoleUser    Role = "USER"
	RoleAuditor Role = "AUDITOR"
)

// DisputeStatus represents lifecycle status of a dispute.
type DisputeStatus string

const (
	StatusOpen        DisputeStatus = "OPEN"
	StatusUnderReview DisputeStatus = "UNDER_REVIEW"
	StatusResolved    DisputeStatus = "RESOLVED"
	StatusRejected    DisputeStatus = "REJECTED"
)

// User represents a system user account.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FullName     string    `json:"full_name"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Dispute represents a financial transaction dispute case.
type Dispute struct {
	ID              string        `json:"id"`
	Title           string        `json:"title"`
	Description     string        `json:"description"`
	Amount          float64       `json:"amount"`
	Currency        string        `json:"currency"`
	Status          DisputeStatus `json:"status"`
	CreatedByID     string        `json:"created_by_id"`
	CreatedBy       *User         `json:"created_by,omitempty"`
	AssigneeID      *string       `json:"assignee_id,omitempty"`
	Assignee        *User         `json:"assignee,omitempty"`
	ResolutionNotes *string       `json:"resolution_notes,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// AuthClaims encapsulates JWT payload claims for role-based authorization.
type AuthClaims struct {
	UserID      string   `json:"user_id"`
	Email       string   `json:"email"`
	Role        Role     `json:"role"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

// AuthPayload returned upon successful JWT authentication.
type AuthPayload struct {
	Token     string `json:"token"`
	User      *User  `json:"user"`
	ExpiresIn int64  `json:"expires_in"` // seconds
}

// CreateDisputeInput input structure for creating disputes.
type CreateDisputeInput struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
}
