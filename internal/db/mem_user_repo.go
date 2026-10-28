package db

import (
	"context"
	"sync"
	"time"

	"github.com/igetayor/recourse/internal/models"
)

// MemUserRepository is a thread-safe in-memory replacement for UserRepository.
// It is used automatically when Postgres is unavailable (demo / local mode).
type MemUserRepository struct {
	mu    sync.RWMutex
	byID  map[string]*models.User
	byEmail map[string]*models.User
}

// NewMemUserRepository constructs an empty in-memory user store.
func NewMemUserRepository() *MemUserRepository {
	return &MemUserRepository{
		byID:    make(map[string]*models.User),
		byEmail: make(map[string]*models.User),
	}
}

func (r *MemUserRepository) CreateUser(_ context.Context, u *models.User) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byEmail[u.Email]; exists {
		return nil, ErrDuplicate
	}
	clone := *u
	r.byID[u.ID] = &clone
	r.byEmail[u.Email] = &clone
	out := clone
	return &out, nil
}

func (r *MemUserRepository) GetUserByEmail(_ context.Context, email string) (*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	u, ok := r.byEmail[email]
	if !ok {
		return nil, ErrNotFound
	}
	out := *u
	return &out, nil
}

func (r *MemUserRepository) GetUserByID(_ context.Context, id string) (*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	u, ok := r.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	out := *u
	return &out, nil
}

func (r *MemUserRepository) ListUsers(_ context.Context) ([]*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]*models.User, 0, len(r.byID))
	for _, u := range r.byID {
		out := *u
		users = append(users, &out)
	}
	return users, nil
}

func (r *MemUserRepository) UpdateUserRole(_ context.Context, userID string, role models.Role) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.byID[userID]
	if !ok {
		return nil, ErrNotFound
	}
	u.Role = role
	u.UpdatedAt = time.Now()
	r.byEmail[u.Email] = u
	out := *u
	return &out, nil
}
