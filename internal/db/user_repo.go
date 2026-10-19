package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/igetayor/recourse/internal/models"
)

var ErrNotFound = errors.New("record not found")
var ErrDuplicate = errors.New("record already exists")

// UserRepository handles user persistence operations.
type UserRepository struct {
	pool *Pool
}

// NewUserRepository constructs a UserRepository.
func NewUserRepository(pool *Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// CreateUser inserts a new user and returns the persisted record.
func (r *UserRepository) CreateUser(ctx context.Context, u *models.User) (*models.User, error) {
	const q = `
		INSERT INTO users (id, email, password_hash, full_name, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, email, full_name, role, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		u.ID, u.Email, u.PasswordHash, u.FullName, u.Role, u.CreatedAt, u.UpdatedAt)

	var out models.User
	if err := row.Scan(&out.ID, &out.Email, &out.FullName, &out.Role, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &out, nil
}

// GetUserByEmail fetches a user including password_hash for auth.
func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	const q = `
		SELECT id, email, password_hash, full_name, role, created_at, updated_at
		FROM users WHERE email = $1`

	row := r.pool.QueryRow(ctx, q, email)
	var u models.User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

// GetUserByID fetches a user by primary key.
func (r *UserRepository) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	const q = `
		SELECT id, email, full_name, role, created_at, updated_at
		FROM users WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	var u models.User
	if err := row.Scan(&u.ID, &u.Email, &u.FullName, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

// ListUsers returns all users ordered by creation date.
func (r *UserRepository) ListUsers(ctx context.Context) ([]*models.User, error) {
	const q = `
		SELECT id, email, full_name, role, created_at, updated_at
		FROM users ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.FullName, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, rows.Err()
}

// UpdateUserRole atomically updates a user's role.
func (r *UserRepository) UpdateUserRole(ctx context.Context, userID string, role models.Role) (*models.User, error) {
	const q = `
		UPDATE users SET role = $1, updated_at = $2
		WHERE id = $3
		RETURNING id, email, full_name, role, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q, role, time.Now(), userID)
	var u models.User
	if err := row.Scan(&u.ID, &u.Email, &u.FullName, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update role: %w", err)
	}
	return &u, nil
}
