package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transactionRepository struct {
	db *pgxpool.Pool
}

// NewTransactionRepository creates a new PostgreSQL-backed transaction repository.
func NewTransactionRepository(db *pgxpool.Pool) domain.TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	query := `
		INSERT INTO transactions (
			id, user_id, external_id, idempotency_key,
			provider, amount, currency, status,
			payment_method, metadata, created_at, updated_at, expired_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11, $12, $13
		)
	`
	_, err := r.db.Exec(ctx, query,
		tx.ID,
		tx.UserID,
		tx.ExternalID,
		tx.IdempotencyKey,
		tx.Provider,
		tx.Amount,
		tx.Currency,
		tx.Status,
		tx.PaymentMethod,
		tx.Metadata,
		tx.CreatedAt,
		tx.UpdatedAt,
		tx.ExpiredAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrTransactionDuplicate
		}
		return fmt.Errorf("transactionRepository.Create: %w", err)
	}
	return nil
}

func (r *transactionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	query := `
		SELECT id, user_id, external_id, idempotency_key,
		       provider, amount, currency, status,
		       payment_method, metadata, created_at, updated_at, expired_at
		FROM transactions
		WHERE id = $1
	`
	tx, err := r.scanTransaction(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("transactionRepository.GetByID: %w", err)
	}
	return tx, nil
}

func (r *transactionRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	query := `
		SELECT id, user_id, external_id, idempotency_key,
		       provider, amount, currency, status,
		       payment_method, metadata, created_at, updated_at, expired_at
		FROM transactions
		WHERE idempotency_key = $1
	`
	tx, err := r.scanTransaction(r.db.QueryRow(ctx, query, key))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("transactionRepository.GetByIdempotencyKey: %w", err)
	}
	return tx, nil
}

func (r *transactionRepository) GetByExternalID(ctx context.Context, externalID string, provider domain.PaymentProvider) (*domain.Transaction, error) {
	query := `
		SELECT id, user_id, external_id, idempotency_key,
		       provider, amount, currency, status,
		       payment_method, metadata, created_at, updated_at, expired_at
		FROM transactions
		WHERE external_id = $1 AND provider = $2
	`
	tx, err := r.scanTransaction(r.db.QueryRow(ctx, query, externalID, provider))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("transactionRepository.GetByExternalID: %w", err)
	}
	return tx, nil
}

func (r *transactionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.TransactionStatus) error {
	query := `
		UPDATE transactions
		SET status = $1, updated_at = $2
		WHERE id = $3
	`
	result, err := r.db.Exec(ctx, query, status, time.Now(), id)
	if err != nil {
		return fmt.Errorf("transactionRepository.UpdateStatus: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrTransactionNotFound
	}
	return nil
}

func (r *transactionRepository) List(ctx context.Context, userID uuid.UUID, filter domain.TransactionFilter) ([]*domain.Transaction, int64, error) {
	// Build dynamic WHERE clause safely with parameterized queries
	conditions := []string{"user_id = $1"}
	args := []any{userID}
	argIdx := 2

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.Provider != nil {
		conditions = append(conditions, fmt.Sprintf("provider = $%d", argIdx))
		args = append(args, *filter.Provider)
		argIdx++
	}
	if filter.Currency != nil {
		conditions = append(conditions, fmt.Sprintf("currency = $%d", argIdx))
		args = append(args, *filter.Currency)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	// Count total for pagination
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM transactions WHERE %s", where)
	var total int64
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("transactionRepository.List count: %w", err)
	}

	// Pagination
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := (filter.Page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, user_id, external_id, idempotency_key,
		       provider, amount, currency, status,
		       payment_method, metadata, created_at, updated_at, expired_at
		FROM transactions
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("transactionRepository.List query: %w", err)
	}
	defer rows.Close()

	var txs []*domain.Transaction
	for rows.Next() {
		tx, err := r.scanTransaction(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("transactionRepository.List scan: %w", err)
		}
		txs = append(txs, tx)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("transactionRepository.List rows: %w", err)
	}

	return txs, total, nil
}

func (r *transactionRepository) GetPendingByProvider(ctx context.Context, provider domain.PaymentProvider, since time.Time) ([]*domain.Transaction, error) {
	query := `
		SELECT id, user_id, external_id, idempotency_key,
		       provider, amount, currency, status,
		       payment_method, metadata, created_at, updated_at, expired_at
		FROM transactions
		WHERE provider = $1
		  AND status = 'pending'
		  AND created_at >= $2
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, provider, since)
	if err != nil {
		return nil, fmt.Errorf("transactionRepository.GetPendingByProvider: %w", err)
	}
	defer rows.Close()

	var txs []*domain.Transaction
	for rows.Next() {
		tx, err := r.scanTransaction(rows)
		if err != nil {
			return nil, fmt.Errorf("transactionRepository.GetPendingByProvider scan: %w", err)
		}
		txs = append(txs, tx)
	}
	return txs, rows.Err()
}

// scanTransaction is a helper that scans a row into a Transaction struct.
// Accepts both pgx.Row and pgx.Rows via the scannable interface.
func (r *transactionRepository) scanTransaction(row scannable) (*domain.Transaction, error) {
	tx := &domain.Transaction{}
	err := row.Scan(
		&tx.ID,
		&tx.UserID,
		&tx.ExternalID,
		&tx.IdempotencyKey,
		&tx.Provider,
		&tx.Amount,
		&tx.Currency,
		&tx.Status,
		&tx.PaymentMethod,
		&tx.Metadata,
		&tx.CreatedAt,
		&tx.UpdatedAt,
		&tx.ExpiredAt,
	)
	return tx, err
}
