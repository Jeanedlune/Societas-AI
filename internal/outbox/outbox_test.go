package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/Jeanedlune/Societas-AI/internal/contracts/v1"
	"github.com/Jeanedlune/Societas-AI/internal/eventstore"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

type capture struct {
	ids  []string
	fail bool
}

func (c *capture) Deliver(_ context.Context, event eventstore.StoredEvent) error {
	c.ids = append(c.ids, string(event.Event.Header.ID))
	if c.fail {
		return errors.New("temporary")
	}
	return nil
}

func TestCommittedOutboxRecoversAfterRestart(t *testing.T) {
	ctx := context.Background()
	filename := filepath.Join(t.TempDir(), "outbox.db")
	db, events := openTest(t, ctx, filename)
	seed(t, db)
	appended, err := events.Append(ctx, envelope(1, "stable-key"))
	if err != nil {
		t.Fatal(err)
	}
	var outboxRows int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM event_outbox WHERE event_id=?", appended.Event.Event.Header.ID).Scan(&outboxRows); err != nil || outboxRows != 1 {
		t.Fatalf("atomic outbox row count=%d err=%v", outboxRows, err)
	}
	out, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := out.Claim(ctx, "crashed-worker", 5*time.Millisecond, 3)
	if err != nil || claim == nil {
		t.Fatalf("claim before simulated crash: claim=%#v err=%v", claim, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(8 * time.Millisecond)
	db, events = openTest(t, ctx, filename)
	defer db.Close()
	out, err = NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	deliverer := &capture{}
	d := Dispatcher{Outbox: out, Events: events, Deliverer: deliverer, Owner: "worker-1", Lease: time.Second, MaxAttempts: 3}
	worked, err := d.DispatchOne(ctx)
	if err != nil || !worked {
		t.Fatalf("dispatch after restart worked=%v err=%v", worked, err)
	}
	if len(deliverer.ids) != 1 || deliverer.ids[0] != string(appended.Event.Event.Header.ID) {
		t.Fatalf("delivered IDs=%v", deliverer.ids)
	}
	var state string
	if err := db.QueryRowContext(ctx, "SELECT delivery_state FROM event_outbox WHERE event_id=?", deliverer.ids[0]).Scan(&state); err != nil || state != "delivered" {
		t.Fatalf("delivery state=%q err=%v", state, err)
	}
}

func TestRetryKeepsIdentityAndStopsAtDeadLetter(t *testing.T) {
	ctx := context.Background()
	db, events := openTest(t, ctx, filepath.Join(t.TempDir(), "retry.db"))
	defer db.Close()
	seed(t, db)
	result, err := events.Append(ctx, envelope(2, "stable-retry-key"))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := NewStore(db)
	delivery := &capture{fail: true}
	d := Dispatcher{Outbox: out, Events: events, Deliverer: delivery, Owner: "worker", Lease: time.Second, MaxAttempts: 2, BaseBackoff: time.Millisecond, MaxBackoff: time.Millisecond}
	if worked, err := d.DispatchOne(ctx); !worked || err == nil {
		t.Fatalf("first attempt worked=%v err=%v", worked, err)
	}
	time.Sleep(3 * time.Millisecond)
	if worked, err := d.DispatchOne(ctx); !worked || err == nil {
		t.Fatalf("second attempt worked=%v err=%v", worked, err)
	}
	if len(delivery.ids) != 2 || delivery.ids[0] != delivery.ids[1] || delivery.ids[0] != string(result.Event.Event.Header.ID) {
		t.Fatalf("retry identity changed: %v", delivery.ids)
	}
	var state, code string
	if err := db.QueryRowContext(ctx, "SELECT delivery_state,last_error_code FROM event_outbox WHERE event_id=?", delivery.ids[0]).Scan(&state, &code); err != nil || state != "dead" || code != "DELIVERY_FAILED" {
		t.Fatalf("state=%q code=%q err=%v", state, code, err)
	}
	if err := out.RetryDead(ctx, delivery.ids[0]); err != nil {
		t.Fatalf("RetryDead(): %v", err)
	}
}

func openTest(t *testing.T, ctx context.Context, path string) (*sql.DB, *eventstore.Store) {
	t.Helper()
	db, err := storage.Open(ctx, path)
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
func seed(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO workspaces(workspace_id,name) VALUES('ws_demo','Demo')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO runs(workspace_id,run_id,goal) VALUES('ws_demo','RUN-001','test')`); err != nil {
		t.Fatal(err)
	}
}
func envelope(n int, key string) []byte {
	e := v1.Envelope{ProtocolVersion: v1.ProtocolVersion, SchemaVersion: v1.SchemaVersion, ID: v1.EventID(fmt.Sprintf("evt_01ARZ3NDEKTSV4RRFFQ69G5F%02d", n)), Type: v1.EventRunCreated, Timestamp: "2026-10-23T12:00:00Z", WorkspaceID: "ws_demo", RunID: "RUN-001", CorrelationID: "cor_01ARZ3NDEKTSV4RRFFQ69G5FAV", IdempotencyKey: key, From: "human:user", To: "system:orchestrator", Payload: json.RawMessage(`{"goal":"test"}`), Principal: v1.Principal{PrincipalID: "usr_01ARZ3NDEKTSV4RRFFQ69G5FAV", PrincipalType: v1.PrincipalHuman, SessionID: "ses_01ARZ3NDEKTSV4RRFFQ69G5FAV", AuthDigest: "hmac_sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	b, _ := json.Marshal(e)
	return b
}
