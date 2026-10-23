package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	v1 "github.com/Jeanedlune/Societas-AI/internal/contracts/v1"
	"github.com/Jeanedlune/Societas-AI/internal/eventstore"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

func TestLifecyclePersistsStateEventAndOutboxAtomically(t *testing.T) {
	ctx := context.Background()
	db, events := newLifecycleStore(t, ctx)
	defer db.Close()
	seedLifecycle(t, db)
	lifecycle, err := NewLifecycle(db, events)
	if err != nil {
		t.Fatal(err)
	}
	result, err := lifecycle.Transition(ctx, "ws_demo", "RUN-001", "researcher", AgentIdle, AgentWorking, lifecycleEnvelope(1, "idle", "working"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Duplicate {
		t.Fatal("first state transition marked duplicate")
	}
	state, err := lifecycle.Status(ctx, "ws_demo", "researcher")
	if err != nil || state != AgentWorking {
		t.Fatalf("state=%q err=%v", state, err)
	}
	var eventCount, outboxCount int
	if err := db.QueryRow(`SELECT count(*) FROM events WHERE event_type='agent.status_changed'`).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE delivery_state='pending'`).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 || outboxCount != 1 {
		t.Fatalf("event rows=%d pending outbox=%d", eventCount, outboxCount)
	}
	duplicate, err := lifecycle.Transition(ctx, "ws_demo", "RUN-001", "researcher", AgentIdle, AgentWorking, lifecycleEnvelope(1, "idle", "working"))
	if err != nil || !duplicate.Duplicate {
		t.Fatalf("idempotent transition duplicate=%v err=%v", duplicate.Duplicate, err)
	}
	if _, err := lifecycle.Transition(ctx, "ws_demo", "RUN-001", "researcher", AgentIdle, AgentCompleted, lifecycleEnvelope(2, "idle", "completed")); !errors.Is(err, ErrInvalidAgentTransition) {
		t.Fatalf("invalid transition error=%v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM events`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("invalid transition persisted events=%d err=%v", count, err)
	}
}

func newLifecycleStore(t *testing.T, ctx context.Context) (*sql.DB, *eventstore.Store) {
	t.Helper()
	db, err := storage.Open(ctx, filepath.Join(t.TempDir(), "lifecycle.db"))
	if err != nil {
		t.Fatal(err)
	}
	validator, err := v1.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	events, err := eventstore.New(db, validator)
	if err != nil {
		t.Fatal(err)
	}
	return db, events
}
func seedLifecycle(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{`INSERT INTO workspaces(workspace_id,name) VALUES('ws_demo','Demo')`, `INSERT INTO runs(workspace_id,run_id,goal) VALUES('ws_demo','RUN-001','test')`, `INSERT INTO agents(workspace_id,agent_id,display_name,role,status) VALUES('ws_demo','researcher','Researcher','research','idle')`} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}
func lifecycleEnvelope(number int, previous, status string) []byte {
	e := v1.Envelope{ProtocolVersion: v1.ProtocolVersion, SchemaVersion: v1.SchemaVersion, ID: v1.EventID("evt_01ARZ3NDEKTSV4RRFFQ69G5F" + string([]byte{'0', byte('0' + number)})), Type: v1.EventAgentStatusChanged, Timestamp: "2026-10-23T12:00:00Z", WorkspaceID: "ws_demo", RunID: "RUN-001", CorrelationID: "cor_01ARZ3NDEKTSV4RRFFQ69G5FAV", IdempotencyKey: "agent-status-" + status, From: "system:orchestrator", To: "topic:all", Payload: json.RawMessage(`{"agent_id":"researcher","previous_status":"` + previous + `","status":"` + status + `"}`), Principal: v1.Principal{PrincipalID: "sys_01ARZ3NDEKTSV4RRFFQ69G5FAV", PrincipalType: v1.PrincipalSystem, SessionID: "ses_01ARZ3NDEKTSV4RRFFQ69G5FAV", AuthDigest: "hmac_sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	raw, _ := json.Marshal(e)
	return raw
}
