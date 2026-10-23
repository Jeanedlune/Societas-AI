package eventbus

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
	"github.com/Jeanedlune/Societas-AI/internal/outbox"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

func TestPublishPersistsThenDispatchesInOrderAndUnsubscribe(t *testing.T) {
	ctx := context.Background()
	db, events, out, bus := newBus(t, ctx)
	defer db.Close()
	seedBus(t, db)
	if _, err := bus.Publish(ctx, []byte(`{"type":"run.created"}`)); err == nil {
		t.Fatal("invalid event was published")
	}
	var stored int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM events").Scan(&stored); err != nil || stored != 0 {
		t.Fatalf("invalid publish persisted %d events; err=%v", stored, err)
	}
	if err := bus.Deliver(ctx, eventstore.StoredEvent{}); !errors.Is(err, ErrNotPersisted) {
		t.Fatalf("unpersisted delivery error=%v, want %v", err, ErrNotPersisted)
	}
	sub, err := bus.Subscribe(SubscriptionSpec{ID: "ordered", WorkspaceID: "ws_demo", RunID: "RUN-001", Addresses: []string{"system:orchestrator"}, Buffer: 2})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		if _, err := bus.Publish(ctx, busEnvelope(i)); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-sub.Events():
		t.Fatal("subscriber saw an event before dispatcher read committed outbox")
	default:
	}
	d := outbox.Dispatcher{Outbox: out, Events: events, Deliverer: bus, Owner: "worker", Lease: time.Second, MaxAttempts: 2}
	for i := 0; i < 2; i++ {
		worked, err := d.DispatchOne(ctx)
		if err != nil || !worked {
			t.Fatalf("DispatchOne worked=%v err=%v", worked, err)
		}
	}
	for i := 1; i <= 2; i++ {
		select {
		case event := <-sub.Events():
			if event.Sequence != uint64(i) {
				t.Fatalf("sequence=%d want %d", event.Sequence, i)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event")
		}
	}
	sub.Close()
	if _, ok := <-sub.Events(); ok {
		t.Fatal("subscription channel remained open after unsubscribe")
	}
}

func TestTopicSubscriptionsRouteOnlyMatchingTopic(t *testing.T) {
	ctx := context.Background()
	db, _, _, bus := newBus(t, ctx)
	defer db.Close()
	seedBus(t, db)
	news, err := bus.Subscribe(SubscriptionSpec{ID: "news", Addresses: []string{"topic:news"}, Buffer: 1})
	if err != nil {
		t.Fatal(err)
	}
	all, err := bus.Subscribe(SubscriptionSpec{ID: "all-topics", Addresses: []string{"topic:all"}, Buffer: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bus.Publish(ctx, busEnvelopeTo(1, "topic:news")); err != nil {
		t.Fatal(err)
	}
	if _, err := bus.Publish(ctx, busEnvelopeTo(2, "topic:sports")); err != nil {
		t.Fatal(err)
	}
	d := outbox.Dispatcher{Outbox: bus.outbox, Events: bus.events, Deliverer: bus, Owner: "worker", Lease: time.Second, MaxAttempts: 2}
	for i := 0; i < 2; i++ {
		if worked, err := d.DispatchOne(ctx); err != nil || !worked {
			t.Fatalf("dispatch worked=%v err=%v", worked, err)
		}
	}
	if got := <-news.Events(); got.Event.Header.To != "topic:news" {
		t.Fatalf("news subscriber got %q", got.Event.Header.To)
	}
	if got := <-all.Events(); got.Event.Header.To != "topic:news" {
		t.Fatalf("topic:all first got %q", got.Event.Header.To)
	}
	if got := <-all.Events(); got.Event.Header.To != "topic:sports" {
		t.Fatalf("topic:all second got %q", got.Event.Header.To)
	}
	select {
	case got := <-news.Events():
		t.Fatalf("news subscriber got unrelated topic %q", got.Event.Header.To)
	default:
	}
	news.Close()
	all.Close()
}

func TestBackpressureIsBoundedAndRetryDoesNotDuplicate(t *testing.T) {
	ctx := context.Background()
	db, events, out, bus := newBus(t, ctx)
	defer db.Close()
	seedBus(t, db)
	sub, err := bus.Subscribe(SubscriptionSpec{ID: "slow", Buffer: 1})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		if _, err := bus.Publish(ctx, busEnvelope(i)); err != nil {
			t.Fatal(err)
		}
	}
	d := outbox.Dispatcher{Outbox: out, Events: events, Deliverer: bus, Owner: "worker", Lease: time.Second, MaxAttempts: 3, BaseBackoff: time.Millisecond, MaxBackoff: time.Millisecond}
	if worked, err := d.DispatchOne(ctx); !worked || err != nil {
		t.Fatalf("first dispatch worked=%v err=%v", worked, err)
	}
	if worked, err := d.DispatchOne(ctx); !worked || !errors.Is(err, ErrBackpressure) {
		t.Fatalf("backpressure dispatch worked=%v err=%v", worked, err)
	}
	first := <-sub.Events()
	if first.Sequence != 1 {
		t.Fatalf("first sequence=%d", first.Sequence)
	}
	time.Sleep(3 * time.Millisecond)
	if worked, err := d.DispatchOne(ctx); !worked || err != nil {
		t.Fatalf("retry worked=%v err=%v", worked, err)
	}
	second := <-sub.Events()
	if second.Sequence != 2 {
		t.Fatalf("second sequence=%d", second.Sequence)
	}
	select {
	case extra := <-sub.Events():
		t.Fatalf("duplicate delivery: %#v", extra)
	default:
	}
	sub.Close()
}

func newBus(t *testing.T, ctx context.Context) (*sql.DB, *eventstore.Store, *outbox.Store, *Bus) {
	t.Helper()
	db, err := storage.Open(ctx, filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := v1.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	events, err := eventstore.New(db, v)
	if err != nil {
		t.Fatal(err)
	}
	out, err := outbox.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	bus, err := New(events, out)
	if err != nil {
		t.Fatal(err)
	}
	return db, events, out, bus
}
func seedBus(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO workspaces(workspace_id,name) VALUES('ws_demo','Demo')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO runs(workspace_id,run_id,goal) VALUES('ws_demo','RUN-001','test')`); err != nil {
		t.Fatal(err)
	}
}
func busEnvelope(n int) []byte {
	return busEnvelopeTo(n, "system:orchestrator")
}

func busEnvelopeTo(n int, to string) []byte {
	e := v1.Envelope{ProtocolVersion: v1.ProtocolVersion, SchemaVersion: v1.SchemaVersion, ID: v1.EventID(fmt.Sprintf("evt_01ARZ3NDEKTSV4RRFFQ69G5F%02d", n)), Type: v1.EventRunCreated, Timestamp: "2026-10-23T12:00:00Z", WorkspaceID: "ws_demo", RunID: "RUN-001", CorrelationID: "cor_01ARZ3NDEKTSV4RRFFQ69G5FAV", IdempotencyKey: fmt.Sprintf("key-%d", n), From: "human:user", To: "system:orchestrator", Payload: json.RawMessage(`{"goal":"test"}`), Principal: v1.Principal{PrincipalID: "usr_01ARZ3NDEKTSV4RRFFQ69G5FAV", PrincipalType: v1.PrincipalHuman, SessionID: "ses_01ARZ3NDEKTSV4RRFFQ69G5FAV", AuthDigest: "hmac_sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	e.To = to
	b, _ := json.Marshal(e)
	return b
}
