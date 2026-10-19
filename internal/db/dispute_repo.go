package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/igetayor/recourse/internal/models"
)

// DisputeRepository handles dispute persistence.
type DisputeRepository struct {
	pool *Pool
}

// NewDisputeRepository constructs a DisputeRepository.
func NewDisputeRepository(pool *Pool) *DisputeRepository {
	return &DisputeRepository{pool: pool}
}

// CreateDispute inserts a new dispute record.
func (r *DisputeRepository) CreateDispute(ctx context.Context, d *models.Dispute) (*models.Dispute, error) {
	const q = `
		INSERT INTO disputes (id, title, description, amount, currency, status, created_by_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, title, description, amount, currency, status, created_by_id, assignee_id, resolution_notes, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		d.ID, d.Title, d.Description, d.Amount, d.Currency, d.Status, d.CreatedByID, d.CreatedAt, d.UpdatedAt)

	return scanDispute(row)
}

// GetDisputeByID fetches a dispute by primary key.
func (r *DisputeRepository) GetDisputeByID(ctx context.Context, id string) (*models.Dispute, error) {
	const q = `
		SELECT id, title, description, amount, currency, status, created_by_id, assignee_id, resolution_notes, created_at, updated_at
		FROM disputes WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	d, err := scanDispute(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get dispute: %w", err)
	}
	return d, nil
}

// ListDisputes returns all disputes with optional status filter.
func (r *DisputeRepository) ListDisputes(ctx context.Context, status *models.DisputeStatus) ([]*models.Dispute, error) {
	var (
		q    string
		args []any
	)
	if status != nil {
		q = `SELECT id, title, description, amount, currency, status, created_by_id, assignee_id, resolution_notes, created_at, updated_at
			 FROM disputes WHERE status = $1 ORDER BY created_at DESC`
		args = []any{*status}
	} else {
		q = `SELECT id, title, description, amount, currency, status, created_by_id, assignee_id, resolution_notes, created_at, updated_at
			 FROM disputes ORDER BY created_at DESC`
	}

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list disputes: %w", err)
	}
	defer rows.Close()

	var disputes []*models.Dispute
	for rows.Next() {
		var d models.Dispute
		if err := rows.Scan(
			&d.ID, &d.Title, &d.Description, &d.Amount, &d.Currency,
			&d.Status, &d.CreatedByID, &d.AssigneeID, &d.ResolutionNotes,
			&d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		disputes = append(disputes, &d)
	}
	return disputes, rows.Err()
}

// UpdateDisputeStatus transitions a dispute to a new status with optional notes.
func (r *DisputeRepository) UpdateDisputeStatus(ctx context.Context, id string, status models.DisputeStatus, notes *string, assigneeID *string) (*models.Dispute, error) {
	const q = `
		UPDATE disputes
		SET status = $1, resolution_notes = COALESCE($2, resolution_notes),
		    assignee_id = COALESCE($3, assignee_id), updated_at = $4
		WHERE id = $5
		RETURNING id, title, description, amount, currency, status, created_by_id, assignee_id, resolution_notes, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q, status, notes, assigneeID, time.Now(), id)
	d, err := scanDispute(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update dispute status: %w", err)
	}
	return d, nil
}

// DeleteDispute removes a dispute record.
func (r *DisputeRepository) DeleteDispute(ctx context.Context, id string) error {
	const q = `DELETE FROM disputes WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete dispute: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanDispute(row pgx.Row) (*models.Dispute, error) {
	var d models.Dispute
	err := row.Scan(
		&d.ID, &d.Title, &d.Description, &d.Amount, &d.Currency,
		&d.Status, &d.CreatedByID, &d.AssigneeID, &d.ResolutionNotes,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}
