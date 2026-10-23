package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	modernsqlite "modernc.org/sqlite"
)

func TestOpenMigratesCoreSchemaAndEnablesPragmas(t *testing.T) {
	ctx := context.Background()
	filename := filepath.Join(t.TempDir(), "societas.db")
	db, err := Open(ctx, filename)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	defer db.Close()

	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 5 {
		t.Fatalf("schema version = %d, want 5", version)
	}
	for _, table := range []string{
		"workspaces", "agents", "runs", "tasks", "events", "messages",
		"artifacts", "approvals", "memories", "usage_entries",
		"budget_ledger", "budget_reservation_lease", "event_outbox", "schema_migrations",
	} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("table %q not found", table)
		}
	}
	for _, index := range []string{"idx_tasks_run_status_priority", "idx_events_run_sequence", "idx_artifacts_task", "idx_lease_recovery"} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?`, index).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("index %q not found", index)
		}
	}
	var foreignKeys int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		t.Fatalf("journal_mode = %q, want WAL", journalMode)
	}

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("reapplying migrations: %v", err)
	}
	var history int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&history); err != nil {
		t.Fatal(err)
	}
	if history != 5 {
		t.Fatalf("migration history has %d rows, want 5", history)
	}
}

func TestUpgradeFromPreviousMigrationIsDeterministic(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	migrations, err := loadMigrations(migrationFiles)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyMigration(ctx, db, migrations[0]); err != nil {
		t.Fatalf("apply initial migration: %v", err)
	}
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("initial schema version = %d, want 1", version)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("upgrade from version 1: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("repeat upgrade: %v", err)
	}
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 5 {
		t.Fatalf("upgraded schema version = %d, want 5", version)
	}
}

func TestFailedMigrationRollsBackDDLAndVersion(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	bad := migration{version: 6, name: "broken", sql: "CREATE TABLE rollback_probe(id INTEGER); CREATE TABLE workspaces(id INTEGER);"}
	if err := applyMigration(ctx, db, bad); err == nil {
		t.Fatal("expected migration to fail")
	}
	var count, version int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='rollback_probe'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("DDL from failed migration was not rolled back")
	}
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 5 {
		t.Fatalf("schema version after rollback = %d, want 5", version)
	}
	var history int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE version=6").Scan(&history); err != nil {
		t.Fatal(err)
	}
	if history != 0 {
		t.Fatal("failed migration was recorded in migration history")
	}
}

func TestForeignKeysAndTaskStatusConstraints(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "constraints.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `INSERT INTO runs(workspace_id, run_id, goal) VALUES ('ws_missing', 'RUN-001', 'goal')`); err == nil {
		t.Fatal("run with unknown workspace was accepted")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO workspaces(workspace_id, name) VALUES ('ws_demo', 'Demo')`); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`INSERT INTO agents(workspace_id, agent_id, display_name, role) VALUES ('ws_demo', 'planner', 'Planner', 'coordinator')`,
		`INSERT INTO runs(workspace_id, run_id, goal, entry_agent_id) VALUES ('ws_demo', 'RUN-001', 'goal', 'planner')`,
		`INSERT INTO tasks(workspace_id, run_id, task_id, title, status, owner_agent_id) VALUES ('ws_demo', 'RUN-001', 'TASK-001', 'Draft', 'running', 'planner')`,
		`INSERT INTO events(event_id, workspace_id, run_id, task_id, sequence, event_type, schema_version, correlation_id, payload_json, envelope_json, created_at) VALUES ('evt_1', 'ws_demo', 'RUN-001', 'TASK-001', 1, 'task.started', '1', 'cor_1', '{}', '{}', CURRENT_TIMESTAMP)`,
		`INSERT INTO messages(workspace_id, message_id, run_id, task_id, conversation_id, sender, recipient, body, created_at) VALUES ('ws_demo', 'msg_1', 'RUN-001', 'TASK-001', 'conv_1', 'agent:planner', 'human:user', 'hello', CURRENT_TIMESTAMP)`,
		`INSERT INTO artifacts(workspace_id, artifact_id, run_id, version, task_id, name, kind, relative_path, checksum, size_bytes, created_by, created_at) VALUES ('ws_demo', 'art_1', 'RUN-001', 1, 'TASK-001', 'draft.md', 'markdown', 'artifacts/draft.md', 'sha256:test', 10, 'agent:planner', CURRENT_TIMESTAMP)`,
		`INSERT INTO approvals(workspace_id, approval_id, run_id, task_id, action, status, bound_hash, snapshot_ref_json, created_at, updated_at) VALUES ('ws_demo', 'apr_1', 'RUN-001', 'TASK-001', 'tool.execute', 'pending', 'sha256:test', '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		`INSERT INTO memories(workspace_id, memory_id, source_artifact_id, source_artifact_version, content_json) VALUES ('ws_demo', 'mem_1', 'art_1', 1, '{"text":"memory"}')`,
		`INSERT INTO usage_entries(workspace_id, usage_id, run_id, task_id, agent_id, source, input_tokens, output_tokens, cost_usd, created_at) VALUES ('ws_demo', 'usage_1', 'RUN-001', 'TASK-001', 'planner', 'model.call', 10, 5, 0.01, CURRENT_TIMESTAMP)`,
		`INSERT INTO budget_ledger(ledger_id, workspace_id, run_id, task_id, agent_id, scope, entry_type, amount_tokens, amount_cost_usd, balance_tokens, balance_cost_usd) VALUES ('led_1', 'ws_demo', 'RUN-001', 'TASK-001', 'planner', 'task', 'grant', 100, 1, 100, 1)`,
		`INSERT INTO budget_reservation_lease(reservation_id, workspace_id, run_id, task_id, agent_id, reserved_tokens, reserved_cost_usd, lease_status, expires_at) VALUES ('res_1', 'ws_demo', 'RUN-001', 'TASK-001', 'planner', 10, 0.1, 'active', CURRENT_TIMESTAMP)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("insert valid core fixture: %v\nSQL: %s", err, statement)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO tasks(workspace_id, run_id, task_id, title, status) VALUES ('ws_demo', 'RUN-001', 'TASK-002', 'Bad', 'BLOCKED')`); err == nil {
		t.Fatal("non-canonical task status was accepted")
	}
	if _, err := db.ExecContext(ctx, `UPDATE budget_ledger SET amount_tokens=amount_tokens WHERE ledger_id='led_1'`); err == nil {
		t.Fatal("budget ledger update was accepted")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM budget_ledger WHERE ledger_id='led_1'`); err == nil {
		t.Fatal("budget ledger delete was accepted")
	}
	for _, table := range []string{"summaries"} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("table %q found=%d err=%v", table, count, err)
		}
	}
}

func TestBusyTimeoutSurfacesErrorWithoutLosingCommittedData(t *testing.T) {
	ctx := context.Background()
	filename := filepath.Join(t.TempDir(), "busy.db")
	options := OpenOptions{BusyTimeout: 100 * time.Millisecond, MaxOpenConns: 2}
	db1, err := OpenWithOptions(ctx, filename, options)
	if err != nil {
		t.Fatal(err)
	}
	defer db1.Close()
	db2, err := OpenWithOptions(ctx, filename, options)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	first, err := db1.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.ExecContext(ctx, `INSERT INTO workspaces(workspace_id, name) VALUES ('ws_first', 'First')`); err != nil {
		_ = first.Rollback()
		t.Fatal(err)
	}
	blocked := WithTx(ctx, db2, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO workspaces(workspace_id, name) VALUES ('ws_second', 'Second')`)
		return err
	})
	if blocked == nil {
		_ = first.Rollback()
		t.Fatal("write lock did not surface a busy error")
	}
	if !isSQLiteBusy(blocked) {
		_ = first.Rollback()
		t.Fatalf("busy transaction error = %v", blocked)
	}
	if err := first.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := WithTx(ctx, db2, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO workspaces(workspace_id, name) VALUES ('ws_second', 'Second')`)
		return err
	}); err != nil {
		t.Fatalf("retry after lock release: %v", err)
	}
	var count int
	if err := db1.QueryRowContext(ctx, `SELECT count(*) FROM workspaces WHERE workspace_id IN ('ws_first', 'ws_second')`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("persisted rows = %d, want 2", count)
	}
}

func TestWithTxRollsBackCallbackError(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "tx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	wantErr := errors.New("stop transaction")
	err = WithTx(ctx, db, nil, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workspaces(workspace_id, name) VALUES ('ws_rollback', 'Rollback')`); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WithTx() error = %v, want callback error", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM workspaces WHERE workspace_id='ws_rollback'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("callback failure left a partial database change")
	}
}

func isSQLiteBusy(err error) bool {
	var sqliteErr *modernsqlite.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code() == 5 || sqliteErr.Code() == 6 // SQLITE_BUSY or SQLITE_LOCKED
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "database is busy")
}
