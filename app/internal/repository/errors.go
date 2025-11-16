package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrInvalidID is returned when an ID is invalid (e.g., <= 0).
	ErrInvalidID = errors.New("invalid id")

	// ErrNilValue is returned when a required value is nil.
	ErrNilValue = errors.New("nil value")

	// ErrNotFound is returned when a row is not found in the database.
	ErrNotFound = newProxyErr(pgx.ErrNoRows, "not found")

	// ErrDuplicate is returned when a unique constraint is violated.
	ErrDuplicate = errors.New("duplicate")

	// ErrForeignKeyViolation is returned when a foreign key constraint fails.
	ErrForeignKeyViolation = errors.New("foreign key violation")

	// ErrNoRowsAffected is returned when an update/delete affects no rows.
	ErrNoRowsAffected = errors.New("no rows affected")

	// ErrTxAborted is returned when a transaction is aborted.
	ErrTxAborted = pgx.ErrTxClosed
)

// wrapDBError converts low-level database errors into higher-level
// repository errors. It recognizes common Postgres error codes.
func wrapDBError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if errors.Is(err, pgx.ErrTxClosed) {
		return ErrTxAborted
	}

	// Postgres: transaction aborted message
	if strings.Contains(err.Error(), "current transaction is aborted") {
		return ErrTxAborted
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return ErrDuplicate
		case "23503": // foreign_key_violation
			return ErrForeignKeyViolation
		default:
			return fmt.Errorf("postgres error [%s]: %w", pgErr.Code, err)
		}
	}

	return err
}

// proxyError wraps a background error with a custom message.
type proxyError struct {
	msg        string
	background error
}

// newProxyErr returns a new proxyError.
func newProxyErr(background error, msg string) error {
	return &proxyError{msg: msg, background: background}
}

// Error returns the error message.
func (p *proxyError) Error() string { return p.msg + ": " + p.background.Error() }

// Unwrap returns the underlying error for compatibility with errors.Is/As.
func (p *proxyError) Unwrap() error { return p.background }
