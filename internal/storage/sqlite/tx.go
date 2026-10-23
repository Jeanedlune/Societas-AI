package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// WithTx runs fn in one SQL transaction and propagates every begin, callback,
// and commit error. Callbacks should contain database work only: SQLite may
// return SQLITE_BUSY after the configured busy timeout and the caller must not
// report success unless this helper returns nil.
func WithTx(ctx context.Context, db *sql.DB, options *sql.TxOptions, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, options)
	if err != nil {
		return fmt.Errorf("begin sqlite transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return fmt.Errorf("execute sqlite transaction: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite transaction: %w", err)
	}
	return nil
}
