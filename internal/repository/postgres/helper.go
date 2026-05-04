package postgres

import (
	"github.com/jackc/pgx/v5/pgconn"
)

// scannable is a common interface for pgx.Row and pgx.Rows.
// Allows scanTransaction() to work with both types.
type scannable interface {
	Scan(dest ...any) error
}

// isUniqueViolation returns true if the error is a PostgreSQL unique constraint violation.
// PostgreSQL error code 23505 = unique_violation.
func isUniqueViolation(err error) bool {
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr.Code == "23505"
	}
	return false
}
