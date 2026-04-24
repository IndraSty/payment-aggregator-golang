package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type reconciliationRepository struct {
	db *pgxpool.Pool
}

// NewReconciliationRepository creates a new PostgreSQL-backed reconciliation repository.
func NewReconciliationRepository(db *pgxpool.Pool) domain.ReconciliationRepository {
	return &reconciliationRepository{db: db}
}

func (r *reconciliationRepository) Create(ctx context.Context, report *domain.ReconciliationReport) error {
	query := `
		INSERT INTO reconciliation_reports (id, provider, date, total_checked, discrepancies, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query,
		report.ID,
		report.Provider,
		report.Date,
		report.TotalChecked,
		report.Discrepancies,
		report.Status,
		report.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("reconciliationRepository.Create: %w", err)
	}
	return nil
}

func (r *reconciliationRepository) Update(ctx context.Context, report *domain.ReconciliationReport) error {
	query := `
		UPDATE reconciliation_reports
		SET total_checked = $1, discrepancies = $2, status = $3
		WHERE id = $4
	`
	result, err := r.db.Exec(ctx, query,
		report.TotalChecked,
		report.Discrepancies,
		report.Status,
		report.ID,
	)
	if err != nil {
		return fmt.Errorf("reconciliationRepository.Update: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("reconciliationRepository.Update: report not found")
	}
	return nil
}

func (r *reconciliationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.ReconciliationReport, error) {
	query := `
		SELECT id, provider, date, total_checked, discrepancies, status, created_at
		FROM reconciliation_reports
		WHERE id = $1
	`
	report := &domain.ReconciliationReport{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&report.ID,
		&report.Provider,
		&report.Date,
		&report.TotalChecked,
		&report.Discrepancies,
		&report.Status,
		&report.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("reconciliation report not found")
		}
		return nil, fmt.Errorf("reconciliationRepository.GetByID: %w", err)
	}
	return report, nil
}

func (r *reconciliationRepository) List(ctx context.Context, filter domain.ReconciliationFilter) ([]*domain.ReconciliationReport, int64, error) {
	conditions := []string{"1=1"}
	args := []any{}
	argIdx := 1

	if filter.Provider != nil {
		conditions = append(conditions, fmt.Sprintf("provider = $%d", argIdx))
		args = append(args, *filter.Provider)
		argIdx++
	}
	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM reconciliation_reports WHERE %s", where)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("reconciliationRepository.List count: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := (filter.Page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, provider, date, total_checked, discrepancies, status, created_at
		FROM reconciliation_reports
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("reconciliationRepository.List query: %w", err)
	}
	defer rows.Close()

	var reports []*domain.ReconciliationReport
	for rows.Next() {
		report := &domain.ReconciliationReport{}
		err := rows.Scan(
			&report.ID,
			&report.Provider,
			&report.Date,
			&report.TotalChecked,
			&report.Discrepancies,
			&report.Status,
			&report.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("reconciliationRepository.List scan: %w", err)
		}
		reports = append(reports, report)
	}
	return reports, total, rows.Err()
}
