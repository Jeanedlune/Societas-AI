package eventstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	v1 "github.com/Jeanedlune/Societas-AI/internal/contracts/v1"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

func TestAppendPersistsBeforeReturnAndReplaySurvivesRestart(t *testing.T) {
	ctx := context.Background()
	filename := filepath.Join(t.TempDir(), "events.db")
	db, store := newTestStore(t, filename)
	seedRun(t, db, "RUN-001")

	result, err := store.Append(ctx, eventEnvelope(1, "RUN-001", "request-1", "original"))
	if err != nil {
		t.Fatalf("Append(): %v", err)
	}
	if result.Duplicate || result.Event.Sequence != 1 {
		t.Fatalf("unexpected append result: %#v", result)
	}
	if result.Event.Event.Header.Sequence == nil || *result.Event.Event.Header.Sequence != 1 {
		t.Fatalf("stored envelope missing assigned sequence: %#v", result.Event.Event.Header)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = storage.Open(ctx, filename)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer db.Close()
	validator, err := v1.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	store, err = New(db, validator)
	if err != nil {
		t.Fatal(err)
	}
	byID, err := store.GetByID(ctx, eventID(1))
	if err != nil {
		t.Fatalf("GetByID(): %v", err)
	}
	bySequence, err := store.GetBySequence(ctx, "ws_demo", "RUN-001", 1)
	if err != nil {
		t.Fatalf("GetBySequence(): %v", err)
	}
	if byID.Sequence != 1 || bySequence.Event.Header.ID != v1.EventID(eventID(1)) {
		t.Fatalf("unexpected lookup results: byID=%#v bySequence=%#v", byID, bySequence)
	}
	replay, err := store.ReadRun(ctx, "ws_demo", "RUN-001", 0, 10)
	if err != nil {
		t.Fatalf("ReadRun(): %v", err)
	}
	if len(replay) != 1 || replay[0].Sequence != 1 {
		t.Fatalf("unexpected replay: %#v", replay)
	}
}

func TestAppendDeduplicatesEventIDAndIdempotencyPerRun(t *testing.T) {
	ctx := context.Background()
	db, store := newTestStore(t, filepath.Join(t.TempDir(), "dedupe.db"))
	defer db.Close()
	seedRun(t, db, "RUN-001")
	seedRun(t, db, "RUN-002")

	first, err := store.Append(ctx, eventEnvelope(2, "RUN-001", "same-key", "first"))
	if err != nil {
		t.Fatal(err)
	}
	duplicateKey, err := store.Append(ctx, eventEnvelope(3, "RUN-001", "same-key", "first"))
	if err != nil {
		t.Fatal(err)
	}
	if !duplicateKey.Duplicate || duplicateKey.Event.Event.Header.ID != v1.EventID(eventID(2)) {
		t.Fatalf("idempotency retry did not return original event: %#v", duplicateKey)
	}
	if _, err := store.Append(ctx, eventEnvelope(2, "RUN-001", "different-key", "different event body")); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("event ID reused with different request error=%v, want conflict", err)
	}
	if _, err := store.Append(ctx, eventEnvelope(4, "RUN-001", "same-key", "different retry body")); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("key reused with different payload error=%v, want conflict", err)
	}
	otherRun, err := store.Append(ctx, eventEnvelope(4, "RUN-002", "same-key", "other run"))
	if err != nil {
		t.Fatalf("same key in separate run: %v", err)
	}
	if otherRun.Duplicate || otherRun.Event.Sequence != 1 {
		t.Fatalf("idempotency key was not run-scoped: %#v", otherRun)
	}
	if first.Event.Sequence != 1 {
		t.Fatalf("first sequence = %d, want 1", first.Event.Sequence)
	}
}

func TestConcurrentAppendAssignsContiguousRunSequence(t *testing.T) {
	ctx := context.Background()
	db, store := newTestStore(t, filepath.Join(t.TempDir(), "concurrent.db"))
	defer db.Close()
	seedRun(t, db, "RUN-001")

	const writers = 48
	var wait sync.WaitGroup
	errCh := make(chan error, writers)
	for i := 1; i <= writers; i++ {
		wait.Add(1)
		go func(eventNumber int) {
			defer wait.Done()
			_, err := store.Append(ctx, eventEnvelope(eventNumber, "RUN-001", fmt.Sprintf("key-%d", eventNumber), "concurrent"))
			errCh <- err
		}(i)
	}
	wait.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent Append(): %v", err)
		}
	}

	events, err := store.ReadRun(ctx, "ws_demo", "RUN-001", 0, writers)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != writers {
		t.Fatalf("stored %d events, want %d", len(events), writers)
	}
	for index, event := range events {
		want := uint64(index + 1)
		if event.Sequence != want || event.Event.Header.Sequence == nil || *event.Event.Header.Sequence != want {
			t.Fatalf("event at index %d has sequence %d/header %#v; want %d", index, event.Sequence, event.Event.Header.Sequence, want)
		}
	}
	page, err := store.ReadRun(ctx, "ws_demo", "RUN-001", writers/2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 3 || page[0].Sequence != writers/2+1 {
		t.Fatalf("sequence-paged replay = %#v", page)
	}
}

func TestAppendTransactionRollbackLeavesNoSequenceGap(t *testing.T) {
	ctx := context.Background()
	db, store := newTestStore(t, filepath.Join(t.TempDir(), "rollback.db"))
	defer db.Close()
	seedRun(t, db, "RUN-001")
	if _, err := db.ExecContext(ctx, `CREATE TRIGGER reject_event BEFORE INSERT ON events BEGIN SELECT RAISE(ABORT, 'forced test failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(ctx, eventEnvelope(5, "RUN-001", "rollback-key", "rollback")); err == nil {
		t.Fatal("expected the trigger to fail the append")
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM events WHERE run_id='RUN-001'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed append left %d records", count)
	}
	if _, err := db.ExecContext(ctx, `DROP TRIGGER reject_event`); err != nil {
		t.Fatal(err)
	}
	result, err := store.Append(ctx, eventEnvelope(6, "RUN-001", "next-key", "after rollback"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Event.Sequence != 1 {
		t.Fatalf("sequence after rollback = %d, want 1", result.Event.Sequence)
	}
	if _, err := db.ExecContext(ctx, `UPDATE events SET event_type='run.stopped' WHERE event_id=?`, eventID(6)); err == nil {
		t.Fatal("stored event update was accepted")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM events WHERE event_id=?`, eventID(6)); err == nil {
		t.Fatal("stored event deletion was accepted")
	}
}

func TestAppendRejectsPreassignedSequenceAndInvalidEnvelope(t *testing.T) {
	ctx := context.Background()
	db, store := newTestStore(t, filepath.Join(t.TempDir(), "validation.db"))
	defer db.Close()
	seedRun(t, db, "RUN-001")

	var envelope v1.Envelope
	if err := json.Unmarshal(eventEnvelope(7, "RUN-001", "preassigned", "payload"), &envelope); err != nil {
		t.Fatal(err)
	}
	sequence := uint64(99)
	envelope.Sequence = &sequence
	preassigned, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(ctx, preassigned); err != ErrSequenceAlreadyAssigned {
		t.Fatalf("preassigned sequence error = %v", err)
	}
	if _, err := store.Append(ctx, []byte(`{"invalid":true}`)); err == nil {
		t.Fatal("invalid envelope was accepted")
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM events").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("invalid append wrote %d events", count)
	}
}

func newTestStore(t *testing.T, filename string) (*sql.DB, *Store) {
	t.Helper()
	db, err := storage.Open(context.Background(), filename)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	validator, err := v1.NewValidator()
	if err != nil {
		db.Close()
		t.Fatalf("create contract validator: %v", err)
	}
	store, err := New(db, validator)
	if err != nil {
		db.Close()
		t.Fatalf("create event store: %v", err)
	}
	return db, store
}

func seedRun(t *testing.T, db *sql.DB, runID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO workspaces(workspace_id, name) VALUES ('ws_demo', 'Demo')`); err != nil {
		t.Fatalf("insert test workspace: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO runs(workspace_id, run_id, goal) VALUES ('ws_demo', ?, 'event store test')`, runID); err != nil {
		t.Fatalf("insert test run: %v", err)
	}
}

func eventEnvelope(number int, runID, idempotencyKey, goal string) []byte {
	key := idempotencyKey
	envelope := v1.Envelope{
		ProtocolVersion: v1.ProtocolVersion,
		SchemaVersion:   v1.SchemaVersion,
		ID:              v1.EventID(eventID(number)),
		Type:            v1.EventRunCreated,
		Timestamp:       "2026-10-23T12:00:00Z",
		WorkspaceID:     "ws_demo",
		RunID:           v1.RunID(runID),
		CorrelationID:   "cor_01ARZ3NDEKTSV4RRFFQ69G5FAV",
		IdempotencyKey:  key,
		From:            "human:user",
		To:              "system:orchestrator",
		Payload:         json.RawMessage(fmt.Sprintf(`{"goal":%q}`, goal)),
		Principal: v1.Principal{
			PrincipalID:   "usr_01ARZ3NDEKTSV4RRFFQ69G5FAV",
			PrincipalType: v1.PrincipalHuman,
			SessionID:     "ses_01ARZ3NDEKTSV4RRFFQ69G5FAV",
			AuthDigest:    "hmac_sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		panic(err)
	}
	return raw
}

func eventID(number int) string {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	return "evt_01ARZ3NDEKTSV4RRFFQ69G5F" + string([]byte{alphabet[(number/32)%32], alphabet[number%32]})
}
