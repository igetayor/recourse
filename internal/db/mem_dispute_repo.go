package db

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/igetayor/recourse/internal/models"
)

// MemDisputeRepository is a thread-safe in-memory replacement for DisputeRepository.
type MemDisputeRepository struct {
	mu       sync.RWMutex
	disputes map[string]*models.Dispute
}

// NewMemDisputeRepository constructs an empty in-memory dispute store.
func NewMemDisputeRepository() *MemDisputeRepository {
	return &MemDisputeRepository{
		disputes: make(map[string]*models.Dispute),
	}
}

func (r *MemDisputeRepository) CreateDispute(_ context.Context, d *models.Dispute) (*models.Dispute, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	clone := *d
	r.disputes[d.ID] = &clone
	out := clone
	return &out, nil
}

func (r *MemDisputeRepository) GetDisputeByID(_ context.Context, id string) (*models.Dispute, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	d, ok := r.disputes[id]
	if !ok {
		return nil, ErrNotFound
	}
	out := *d
	return &out, nil
}

func (r *MemDisputeRepository) ListDisputes(_ context.Context, status *models.DisputeStatus) ([]*models.Dispute, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*models.Dispute
	for _, d := range r.disputes {
		if status == nil || d.Status == *status {
			out := *d
			result = append(result, &out)
		}
	}
	// Sort by created_at DESC to match Postgres ORDER BY behaviour.
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (r *MemDisputeRepository) UpdateDisputeStatus(_ context.Context, id string, status models.DisputeStatus, notes *string, assigneeID *string) (*models.Dispute, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	d, ok := r.disputes[id]
	if !ok {
		return nil, ErrNotFound
	}
	d.Status = status
	d.UpdatedAt = time.Now()
	if notes != nil {
		d.ResolutionNotes = notes
	}
	if assigneeID != nil {
		d.AssigneeID = assigneeID
	}
	out := *d
	return &out, nil
}

func (r *MemDisputeRepository) DeleteDispute(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.disputes[id]; !ok {
		return ErrNotFound
	}
	delete(r.disputes, id)
	return nil
}
