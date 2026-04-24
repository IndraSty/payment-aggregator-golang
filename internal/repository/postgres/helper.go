package postgres

import (
	"strings"

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

// isForeignKeyViolation returns true if the error is a PostgreSQL foreign key violation.
// PostgreSQL error code 23503 = foreign_key_violation.
func isForeignKeyViolation(err error) bool {
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr.Code == "23503"
	}
	return false
}

// sanitizeForLog removes newlines and tabs from strings to prevent log injection.
func sanitizeForLog(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", "")
	return s
}
