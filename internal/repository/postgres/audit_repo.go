package postgres

import (
	"context"
	"fmt"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type auditRepository struct {
	db *pgxpool.Pool
}

// NewAuditRepository creates a new PostgreSQL-backed audit log repository.
func NewAuditRepository(db *pgxpool.Pool) domain.AuditRepository {
	return &auditRepository{db: db}
}

func (r *auditRepository) Create(ctx context.Context, entry *domain.TransactionAuditLog) error {
	// INSERT only — the database rule prevents UPDATE and DELETE on this table
	query := `
		INSERT INTO transaction_audit_logs (
			id, transaction_id, from_status, to_status,
			source, raw_payload, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query,
		entry.ID,
		entry.TransactionID,
		entry.FromStatus,
		entry.ToStatus,
		entry.Source,
		entry.RawPayload,
		entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("auditRepository.Create: %w", err)
	}
	return nil
}

func (r *auditRepository) GetByTransactionID(ctx context.Context, txID uuid.UUID) ([]*domain.TransactionAuditLog, error) {
	query := `
		SELECT id, transaction_id, from_status, to_status,
		       source, raw_payload, created_at
		FROM transaction_audit_logs
		WHERE transaction_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, txID)
	if err != nil {
		return nil, fmt.Errorf("auditRepository.GetByTransactionID: %w", err)
	}
	defer rows.Close()

	var logs []*domain.TransactionAuditLog
	for rows.Next() {
		entry := &domain.TransactionAuditLog{}
		err := rows.Scan(
			&entry.ID,
			&entry.TransactionID,
			&entry.FromStatus,
			&entry.ToStatus,
			&entry.Source,
			&entry.RawPayload,
			&entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("auditRepository.GetByTransactionID scan: %w", err)
		}
		logs = append(logs, entry)
	}
	return logs, rows.Err()
}
