package taskstate

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
	"github.com/Jeanedlune/Societas-AI/internal/eventstore"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

func TestTaskLifecycleIsAtomicIdempotentAndTerminal(t *testing.T) {
	ctx := context.Background()
	db, events, store := newTaskStore(t, ctx)
	defer db.Close()
	seedTaskRun(t, db)
	created, err := store.Create(ctx, taskEvent(1, "created", v1.EventTaskCreated, `{"task":`+string(taskObject())+`}`, true))
	if err != nil {
		t.Fatal(err)
	}
	if created.Duplicate {
		t.Fatal("task create was duplicate")
	}
	status, err := store.Status(ctx, "ws_demo", "RUN-001", "TASK-001")
	if err != nil || status != Ready {
		t.Fatalf("initial status=%q err=%v", status, err)
	}
	start := taskEvent(2, "start", v1.EventTaskStarted, `{"attempt":1}`, true)
	first, err := store.Transition(ctx, start)
	if err != nil || first.Duplicate {
		t.Fatalf("start transition duplicate=%v err=%v", first.Duplicate, err)
	}
	duplicate, err := store.Transition(ctx, start)
	if err != nil || !duplicate.Duplicate {
		t.Fatalf("start retry duplicate=%v err=%v", duplicate.Duplicate, err)
	}
	status, err = store.Status(ctx, "ws_demo", "RUN-001", "TASK-001")
	if err != nil || status != Running {
		t.Fatalf("running status=%q err=%v", status, err)
	}
	completed, err := store.Transition(ctx, taskEvent(3, "complete", v1.EventTaskCompleted, `{"result_status":"success"}`, true))
	if err != nil || completed.Duplicate {
		t.Fatalf("complete transition duplicate=%v err=%v", completed.Duplicate, err)
	}
	if _, err := store.Transition(ctx, taskEvent(4, "resume", v1.EventTaskResumed, `{"initiated_by":"system:orchestrator","from_status":"paused"}`, true)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("terminal reopen error=%v", err)
	}
	status, err = store.Status(ctx, "ws_demo", "RUN-001", "TASK-001")
	if err != nil || status != Completed {
		t.Fatalf("terminal status=%q err=%v", status, err)
	}
	var eventCount, outboxCount int
	if err := db.QueryRow(`SELECT count(*) FROM events WHERE task_id='TASK-001'`).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE delivery_state='pending'`).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 3 || outboxCount != 3 {
		t.Fatalf("events=%d pending outbox=%d, rejected/duplicate command changed rows", eventCount, outboxCount)
	}
	_ = events
}

func TestTaskTransitionCASDoesNotOverwriteConcurrentWriter(t *testing.T) {
	ctx := context.Background()
	db, _, store := newTaskStore(t, ctx)
	defer db.Close()
	seedTaskRun(t, db)
	if _, err := store.Create(ctx, taskEvent(1, "created", v1.EventTaskCreated, `{"task":`+string(taskObject())+`}`, true)); err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for i := 2; i <= 3; i++ {
		wait.Add(1)
		go func(n int) {
			defer wait.Done()
			_, err := store.Transition(ctx, taskEvent(n, fmt.Sprintf("start-%d", n), v1.EventTaskStarted, `{"attempt":1}`, true))
			results <- err
		}(i)
	}
	wait.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	status, err := store.Status(ctx, "ws_demo", "RUN-001", "TASK-001")
	if err != nil || status != Running {
		t.Fatalf("final status=%q err=%v", status, err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM events WHERE event_type='task.started'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if successes != 1 || count != 1 {
		t.Fatalf("successful transitions=%d start events=%d", successes, count)
	}
}

func TestTransitionTableCoversPauseApprovalRetryAndTerminal(t *testing.T) {
	cases := []struct {
		from, to Status
		allowed  bool
	}{{Running, Pausing, true}, {Pausing, Paused, true}, {Paused, Running, true}, {Running, AwaitingApproval, true}, {AwaitingApproval, Ready, true}, {Running, Ready, true}, {Pending, Failed, true}, {Completed, Running, false}, {Failed, Ready, false}, {Cancelled, Pending, false}}
	for _, test := range cases {
		if got := CanTransition(test.from, test.to); got != test.allowed {
			t.Errorf("CanTransition(%s,%s)=%v want %v", test.from, test.to, got, test.allowed)
		}
	}
}

func newTaskStore(t *testing.T, ctx context.Context) (*sql.DB, *eventstore.Store, *Store) {
	t.Helper()
	db, err := storage.Open(ctx, filepath.Join(t.TempDir(), "tasks.db"))
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
	store, err := New(db, events)
	if err != nil {
		t.Fatal(err)
	}
	return db, events, store
}
func seedTaskRun(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{`INSERT INTO workspaces(workspace_id,name) VALUES('ws_demo','Demo')`, `INSERT INTO agents(workspace_id,agent_id,display_name,role) VALUES('ws_demo','coordinator','Coordinator','coordination')`, `INSERT INTO runs(workspace_id,run_id,goal,entry_agent_id) VALUES('ws_demo','RUN-001','test','coordinator')`} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}
func taskObject() []byte {
	task := map[string]any{"id": "TASK-001", "run_id": "RUN-001", "parent_task_id": nil, "title": "Prepare brief", "goal": "Create a short evidence-based brief", "owner": "coordinator", "created_by": "system:orchestrator", "priority": "normal", "status": "ready", "dependencies": []string{}, "depth": 0, "attempt": 1, "expected_output": map[string]any{"kind": "report"}, "budget": map[string]any{"granted": map[string]any{}, "used": map[string]any{"input_tokens": 0, "output_tokens": 0, "model_calls": 0, "tool_calls": 0, "estimated_cost_usd": 0}}, "working_directory": "/workspace/artifacts/RUN-001/TASK-001", "created_at": "2026-10-23T12:00:00Z", "updated_at": "2026-10-23T12:00:00Z"}
	raw, _ := json.Marshal(task)
	return raw
}
func taskEvent(number int, key string, eventType v1.EventType, payload string, withTask bool) []byte {
	taskID := v1.TaskID("TASK-001")
	var taskPtr *v1.TaskID
	if withTask {
		taskPtr = &taskID
	}
	from, to := "system:orchestrator", "agent:coordinator"
	principal := v1.Principal{PrincipalID: "sys_01ARZ3NDEKTSV4RRFFQ69G5FAV", PrincipalType: v1.PrincipalSystem, SessionID: "ses_01ARZ3NDEKTSV4RRFFQ69G5FAV", AuthDigest: "hmac_sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if eventType == v1.EventTaskStarted || eventType == v1.EventTaskCompleted {
		from, to = "agent:coordinator", "system:orchestrator"
		principal.PrincipalID = "agt_01ARZ3NDEKTSV4RRFFQ69G5FAV"
		principal.PrincipalType = v1.PrincipalAgent
	}
	event := v1.Envelope{ProtocolVersion: v1.ProtocolVersion, SchemaVersion: v1.SchemaVersion, ID: v1.EventID(fmt.Sprintf("evt_01ARZ3NDEKTSV4RRFFQ69G5F%02d", number)), Type: eventType, Timestamp: "2026-10-23T12:00:00Z", WorkspaceID: "ws_demo", RunID: "RUN-001", TaskID: taskPtr, CorrelationID: "cor_01ARZ3NDEKTSV4RRFFQ69G5FAV", IdempotencyKey: key, From: from, To: to, Payload: json.RawMessage(payload), Principal: principal}
	raw, _ := json.Marshal(event)
	return raw
}
