package replay

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	v1 "github.com/Jeanedlune/Societas-AI/internal/contracts/v1"
	"github.com/Jeanedlune/Societas-AI/internal/eventstore"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

func TestReplayRebuildsTaskStateDeterministically(t *testing.T) {
	state := RunState{WorkspaceID: "ws_demo", RunID: "RUN-001", Tasks: make(map[string]TaskState)}
	events := []eventstore.StoredEvent{
		synthetic(1, v1.EventTaskCreated, nil, `{"task":{"id":"TASK-001","title":"Research","goal":"Find evidence","owner":"researcher","status":"ready","attempt":1}}`),
		synthetic(2, v1.EventTaskStarted, ptrTask("TASK-001"), `{"attempt":1}`),
		synthetic(3, v1.EventTaskRetryScheduled, ptrTask("TASK-001"), `{"attempt":2,"delay_ms":0,"error":{"code":"MODEL_ERROR","category":"provider","message":"retry"}}`),
	}
	for _, event := range events {
		if err := apply(&state, event); err != nil {
			t.Fatal(err)
		}
	}
	task := state.Tasks["TASK-001"]
	if task.Status != "ready" || task.Attempt != 2 || task.LastSequence != 3 {
		t.Fatalf("projected task=%#v", task)
	}
	first, err := state.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	rebuilt := RunState{WorkspaceID: "ws_demo", RunID: "RUN-001", Tasks: make(map[string]TaskState)}
	for _, event := range events {
		if err := apply(&rebuilt, event); err != nil {
			t.Fatal(err)
		}
	}
	second, err := rebuilt.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("replay differs:\n%s\n%s", first, second)
	}
}

func TestReplayRunSurvivesStoreRestart(t *testing.T) {
	ctx := context.Background()
	filename := filepath.Join(t.TempDir(), "replay.db")
	db, store := openReplayStore(t, ctx, filename)
	seedReplayRun(t, db)
	if _, err := store.Append(ctx, replayEnvelope(1, `{"goal":"Plan an editorial strategy"}`)); err != nil {
		t.Fatal(err)
	}
	want, err := ReplayRun(ctx, store, "ws_demo", "RUN-001")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, store = openReplayStore(t, ctx, filename)
	defer db.Close()
	got, err := ReplayRun(ctx, store, "ws_demo", "RUN-001")
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, _ := want.CanonicalJSON()
	gotJSON, _ := got.CanonicalJSON()
	if string(wantJSON) != string(gotJSON) {
		t.Fatalf("restart replay differs: want %s got %s", wantJSON, gotJSON)
	}
}

func synthetic(sequence uint64, typ v1.EventType, taskID *v1.TaskID, payload string) eventstore.StoredEvent {
	env := v1.ValidatedEvent{Header: v1.Envelope{ID: v1.EventID(fmt.Sprintf("evt_01ARZ3NDEKTSV4RRFFQ69G5F%02d", sequence)), Type: typ, WorkspaceID: "ws_demo", RunID: "RUN-001", TaskID: taskID, Payload: json.RawMessage(payload)}}
	return eventstore.StoredEvent{Event: env, Sequence: sequence}
}
func ptrTask(value v1.TaskID) *v1.TaskID { return &value }
func openReplayStore(t *testing.T, ctx context.Context, path string) (*sql.DB, *eventstore.Store) {
	t.Helper()
	db, err := storage.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	validator, err := v1.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	store, err := eventstore.New(db, validator)
	if err != nil {
		t.Fatal(err)
	}
	return db, store
}
func seedReplayRun(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO workspaces(workspace_id,name) VALUES('ws_demo','Demo')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO runs(workspace_id,run_id,goal) VALUES('ws_demo','RUN-001','test')`); err != nil {
		t.Fatal(err)
	}
}
func replayEnvelope(n int, payload string) []byte {
	e := v1.Envelope{ProtocolVersion: v1.ProtocolVersion, SchemaVersion: v1.SchemaVersion, ID: v1.EventID(fmt.Sprintf("evt_01ARZ3NDEKTSV4RRFFQ69G5F%02d", n)), Type: v1.EventRunCreated, Timestamp: "2026-10-23T12:00:00Z", WorkspaceID: "ws_demo", RunID: "RUN-001", CorrelationID: "cor_01ARZ3NDEKTSV4RRFFQ69G5FAV", IdempotencyKey: "run-create", From: "human:user", To: "system:orchestrator", Payload: json.RawMessage(payload), Principal: v1.Principal{PrincipalID: "usr_01ARZ3NDEKTSV4RRFFQ69G5FAV", PrincipalType: v1.PrincipalHuman, SessionID: "ses_01ARZ3NDEKTSV4RRFFQ69G5FAV", AuthDigest: "hmac_sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	raw, _ := json.Marshal(e)
	return raw
}
