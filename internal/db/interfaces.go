package db

import (
	"context"

	"github.com/igetayor/recourse/internal/models"
)

// UserRepo is the interface satisfied by both UserRepository (Postgres) and
// MemUserRepository (in-memory). Add new methods here if you extend the repos.
type UserRepo interface {
	CreateUser(ctx context.Context, u *models.User) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id string) (*models.User, error)
	ListUsers(ctx context.Context) ([]*models.User, error)
	UpdateUserRole(ctx context.Context, userID string, role models.Role) (*models.User, error)
}

// DisputeRepo is the interface satisfied by both DisputeRepository (Postgres) and
// MemDisputeRepository (in-memory).
type DisputeRepo interface {
	CreateDispute(ctx context.Context, d *models.Dispute) (*models.Dispute, error)
	GetDisputeByID(ctx context.Context, id string) (*models.Dispute, error)
	ListDisputes(ctx context.Context, status *models.DisputeStatus) ([]*models.Dispute, error)
	UpdateDisputeStatus(ctx context.Context, id string, status models.DisputeStatus, notes *string, assigneeID *string) (*models.Dispute, error)
	DeleteDispute(ctx context.Context, id string) error
}
