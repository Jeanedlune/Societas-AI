package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

const (
	defaultBusyTimeout = 5 * time.Second
	defaultMaxOpen     = 4
)

var memoryDBSequence atomic.Uint64

// OpenOptions configures a local operational database connection pool.
type OpenOptions struct {
	BusyTimeout  time.Duration
	MaxOpenConns int
}

// Open opens the SQLite database, enables integrity/durability pragmas, and
// applies all pending migrations before returning the handle.
func Open(ctx context.Context, filename string) (*sql.DB, error) {
	return OpenWithOptions(ctx, filename, OpenOptions{})
}

// OpenWithOptions opens and migrates a database using explicit pool settings.
func OpenWithOptions(ctx context.Context, filename string, options OpenOptions) (*sql.DB, error) {
	if filename == "" {
		return nil, fmt.Errorf("sqlite filename is required")
	}
	if options.BusyTimeout == 0 {
		options.BusyTimeout = defaultBusyTimeout
	}
	if options.BusyTimeout < time.Millisecond {
		return nil, fmt.Errorf("sqlite busy timeout must be at least 1ms")
	}
	if options.MaxOpenConns == 0 {
		options.MaxOpenConns = defaultMaxOpen
	}
	if options.MaxOpenConns < 1 {
		return nil, fmt.Errorf("sqlite max open connections must be positive")
	}

	dsn, err := dataSourceName(filename, options.BusyTimeout)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(options.MaxOpenConns)
	db.SetMaxIdleConns(options.MaxOpenConns)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect sqlite database: %w", err)
	}
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate sqlite database: %w", err)
	}
	return db, nil
}

func dataSourceName(filename string, busyTimeout time.Duration) (string, error) {
	query := make(url.Values)
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeout.Milliseconds()))
	query.Add("_pragma", "synchronous(FULL)")

	if filename == ":memory:" {
		query.Add("mode", "memory")
		query.Add("cache", "shared")
		return fmt.Sprintf("file:societas-memory-%d?%s", memoryDBSequence.Add(1), query.Encode()), nil
	}
	abs, err := filepath.Abs(filename)
	if err != nil {
		return "", fmt.Errorf("resolve sqlite path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return "", fmt.Errorf("create sqlite directory: %w", err)
	}
	query.Add("_pragma", "journal_mode(WAL)")
	fileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}).String()
	return fileURL + "?" + query.Encode(), nil
}
