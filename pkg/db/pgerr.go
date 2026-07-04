// Package db holds small, shared helpers for the Postgres data layer so every
// repository handles driver-specific concerns (error codes, tx boilerplate) the
// same way. Keeping this tiny and dependency-scoped means the domain packages
// never import pgx directly.
package db

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrorCode returns the Postgres SQLSTATE for an error, or "" if it isn't a
// Postgres error. Callers compare against known codes (e.g. "23505" =
// unique_violation, "40001" = serialization_failure) rather than string-matching
// messages.
func ErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// IsUniqueViolation reports whether err is a Postgres unique-constraint violation.
func IsUniqueViolation(err error) bool { return ErrorCode(err) == "23505" }
