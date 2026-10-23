package sqlite

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"database/sql"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type migration struct {
	version  int
	name     string
	sql      string
	checksum string
}

// Migrate applies every pending embedded migration in ascending version order.
// Each migration and its schema-version record commit in the same transaction.
func Migrate(ctx context.Context, db *sql.DB) error {
	migrations, err := loadMigrations(migrationFiles)
	if err != nil {
		return err
	}
	version, err := currentVersion(ctx, db)
	if err != nil {
		return err
	}
	if version > len(migrations) {
		return fmt.Errorf("database schema version %d is newer than supported version %d", version, len(migrations))
	}
	if err := verifyMigrationHistory(ctx, db, migrations, version); err != nil {
		return err
	}
	for _, item := range migrations {
		if item.version <= version {
			continue
		}
		if err := applyMigration(ctx, db, item); err != nil {
			return fmt.Errorf("apply migration %04d_%s: %w", item.version, item.name, err)
		}
	}
	return nil
}

func loadMigrations(files fs.FS) ([]migration, error) {
	names, err := fs.Glob(files, "migrations/*.sql")
	if err != nil {
		return nil, fmt.Errorf("list sqlite migrations: %w", err)
	}
	sort.Strings(names)
	migrations := make([]migration, 0, len(names))
	for _, filename := range names {
		base := strings.TrimSuffix(path.Base(filename), ".sql")
		if len(base) < 6 || base[4] != '_' {
			return nil, fmt.Errorf("invalid migration filename %q", filename)
		}
		version, err := strconv.Atoi(base[:4])
		if err != nil || version < 1 {
			return nil, fmt.Errorf("invalid migration version in %q", filename)
		}
		contents, err := fs.ReadFile(files, filename)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", filename, err)
		}
		digest := sha256.Sum256(contents)
		migrations = append(migrations, migration{
			version:  version,
			name:     base[5:],
			sql:      string(contents),
			checksum: hex.EncodeToString(digest[:]),
		})
	}
	for index, item := range migrations {
		if item.version != index+1 {
			return nil, fmt.Errorf("sqlite migrations must be contiguous from 0001; found version %04d", item.version)
		}
	}
	return migrations, nil
}

func currentVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("read sqlite schema version: %w", err)
	}
	return version, nil
}

func verifyMigrationHistory(ctx context.Context, db *sql.DB, migrations []migration, version int) error {
	if version == 0 {
		return nil
	}
	rows, err := db.QueryContext(ctx, `SELECT version, name, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("read sqlite migration history: %w", err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var applied migration
		if err := rows.Scan(&applied.version, &applied.name, &applied.checksum); err != nil {
			return fmt.Errorf("scan sqlite migration history: %w", err)
		}
		if applied.version < 1 || applied.version > version || applied.version > len(migrations) {
			return fmt.Errorf("sqlite migration history contains unexpected version %d", applied.version)
		}
		expected := migrations[applied.version-1]
		if applied.name != expected.name || applied.checksum != expected.checksum {
			return fmt.Errorf("sqlite migration %04d differs from the applied migration", applied.version)
		}
		seen++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read sqlite migration history: %w", err)
	}
	if seen != version {
		return fmt.Errorf("sqlite schema version is %d but migration history contains %d entries", version, seen)
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, item migration) error {
	statements, err := splitStatements(item.sql)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("execute migration statement: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		checksum TEXT NOT NULL,
		applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create migration history: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, name, checksum) VALUES (?, ?, ?)`, item.version, item.name, item.checksum); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", item.version)); err != nil {
		return fmt.Errorf("set sqlite schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration transaction: %w", err)
	}
	committed = true
	return nil
}

// splitStatements recognizes SQL quotes and comments so semicolons inside
// strings or comments do not split a migration statement.
func splitStatements(script string) ([]string, error) {
	if !utf8.ValidString(script) {
		return nil, fmt.Errorf("migration is not valid UTF-8")
	}
	statements := make([]string, 0)
	start := 0
	var quote byte
	lineComment, blockComment := false, false
	for index := 0; index < len(script); index++ {
		current := script[index]
		next := byte(0)
		if index+1 < len(script) {
			next = script[index+1]
		}
		if lineComment {
			if current == '\n' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if current == '*' && next == '/' {
				blockComment = false
				index++
			}
			continue
		}
		if quote != 0 {
			if quote == ']' && current == ']' {
				quote = 0
				continue
			}
			if current == quote {
				if next == quote && quote != ']' {
					index++
					continue
				}
				quote = 0
			}
			continue
		}
		if current == '-' && next == '-' {
			lineComment = true
			index++
			continue
		}
		if current == '/' && next == '*' {
			blockComment = true
			index++
			continue
		}
		switch current {
		case '\'', '"', '`':
			quote = current
		case '[':
			quote = ']'
		case ';':
			if statement := strings.TrimSpace(script[start:index]); statement != "" {
				statements = append(statements, statement)
			}
			start = index + 1
		}
	}
	if quote != 0 || blockComment {
		return nil, fmt.Errorf("migration contains an unterminated quote or comment")
	}
	if statement := strings.TrimSpace(script[start:]); statement != "" {
		statements = append(statements, statement)
	}
	return statements, nil
}
