package budget

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

func TestLedgerReserveSettleIdempotencyAndReconstruction(t *testing.T) {
	ctx := context.Background()
	db, ref := openLedgerDB(t, filepath.Join(t.TempDir(), "ledger.db"))
	ledger, err := NewLedger(db)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	if duplicate, err := ledger.Grant(ctx, "grant_01", ScopeTask, ref, LedgerBalance{Tokens: 100, CostUSD: 1}, now); err != nil || duplicate {
		t.Fatalf("grant duplicate=%v err=%v", duplicate, err)
	}
	if duplicate, err := ledger.Grant(ctx, "grant_01", ScopeTask, ref, LedgerBalance{Tokens: 100, CostUSD: 1}, now); err != nil || !duplicate {
		t.Fatalf("grant retry duplicate=%v err=%v", duplicate, err)
	}
	request := ReservationRequest{ID: "res_01", Scope: ScopeTask, Ref: ref, Tokens: 80, CostUSD: .6, TTL: time.Minute, Now: now}
	reservation, err := ledger.Reserve(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if reservation.Duplicate || reservation.Status != "active" {
		t.Fatalf("reservation=%#v", reservation)
	}
	if reservation, err = ledger.Reserve(ctx, request); err != nil || !reservation.Duplicate {
		t.Fatalf("reservation retry=%#v err=%v", reservation, err)
	}
	if _, err := ledger.Reserve(ctx, ReservationRequest{ID: "res_over", Scope: ScopeTask, Ref: ref, Tokens: 30, CostUSD: .1, Now: now}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("oversubscription error=%v", err)
	}
	if duplicate, err := ledger.Settle(ctx, "res_01", LedgerBalance{Tokens: 30, CostUSD: .2}, now.Add(10*time.Second)); err != nil || duplicate {
		t.Fatalf("settle duplicate=%v err=%v", duplicate, err)
	}
	if duplicate, err := ledger.Settle(ctx, "res_01", LedgerBalance{Tokens: 30, CostUSD: .2}, now.Add(11*time.Second)); err != nil || !duplicate {
		t.Fatalf("settle retry duplicate=%v err=%v", duplicate, err)
	}
	if _, err := ledger.Settle(ctx, "res_01", LedgerBalance{Tokens: 31, CostUSD: .2}, now); !errors.Is(err, ErrReservationConflict) {
		t.Fatalf("conflicting settle error=%v", err)
	}
	balance, err := ledger.Balance(ctx, ScopeTask, ref)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Tokens != 70 || balance.CostUSD < .799999 || balance.CostUSD > .800001 {
		t.Fatalf("balance=%#v", balance)
	}
	rebuilt, err := ledger.Reconstruct(ctx, ScopeTask, ref)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt != balance {
		t.Fatalf("reconstructed=%#v, balance=%#v", rebuilt, balance)
	}
	_ = db.Close()
}

func TestLedgerLeaseRecoverySurvivesRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "recovery.db")
	db, ref := openLedgerDB(t, path)
	ledger, err := NewLedger(db)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	if _, err := ledger.Grant(ctx, "grant_02", ScopeTask, ref, LedgerBalance{Tokens: 50, CostUSD: .5}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Reserve(ctx, ReservationRequest{ID: "res_crash", Scope: ScopeTask, Ref: ref, Tokens: 20, CostUSD: .2, TTL: time.Minute, Now: now}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = storage.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ledger, err = NewLedger(db)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := ledger.RecoverExpired(ctx, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 1 || recovered[0].ID != "res_crash" || recovered[0].Status != "expired" {
		t.Fatalf("recovered=%#v", recovered)
	}
	if again, err := ledger.RecoverExpired(ctx, now.Add(3*time.Minute)); err != nil || len(again) != 0 {
		t.Fatalf("second recovery=%#v err=%v", again, err)
	}
	balance, err := ledger.Balance(ctx, ScopeTask, ref)
	if err != nil {
		t.Fatal(err)
	}
	if balance != (LedgerBalance{Tokens: 50, CostUSD: .5}) {
		t.Fatalf("balance after recovery=%#v", balance)
	}
}

type recoveryProbe struct{ fail bool }

func (p recoveryProbe) AppendBudgetRecovery(ctx context.Context, tx *sql.Tx, reservation Reservation) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO recovery_probe(reservation_id) VALUES(?)`, reservation.ID); err != nil {
		return err
	}
	if p.fail {
		return errors.New("event append failed")
	}
	return nil
}

func TestRecoveryEventAndRefundShareTransaction(t *testing.T) {
	ctx := context.Background()
	db, ref := openLedgerDB(t, filepath.Join(t.TempDir(), "recovery-event.db"))
	defer db.Close()
	if _, err := db.ExecContext(ctx, `CREATE TABLE recovery_probe(reservation_id TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	base, err := NewLedger(db)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	if _, err := base.Grant(ctx, "grant_event", ScopeTask, ref, LedgerBalance{Tokens: 10, CostUSD: .1}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := base.Reserve(ctx, ReservationRequest{ID: "res_event", Scope: ScopeTask, Ref: ref, Tokens: 5, CostUSD: .05, TTL: time.Minute, Now: now}); err != nil {
		t.Fatal(err)
	}
	broken, err := NewLedger(db, recoveryProbe{fail: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broken.RecoverExpired(ctx, now.Add(2*time.Minute)); err == nil {
		t.Fatal("recovery succeeded despite event append failure")
	}
	var status string
	if err := db.QueryRowContext(ctx, `SELECT lease_status FROM budget_reservation_lease WHERE reservation_id='res_event'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM recovery_probe`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if status != "active" || count != 0 {
		t.Fatalf("failed recovery left status=%q, event rows=%d", status, count)
	}
	working, err := NewLedger(db, recoveryProbe{})
	if err != nil {
		t.Fatal(err)
	}
	if recovered, err := working.RecoverExpired(ctx, now.Add(2*time.Minute)); err != nil || len(recovered) != 1 {
		t.Fatalf("recovered=%#v err=%v", recovered, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM recovery_probe`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("committed recovery event count=%d err=%v", count, err)
	}
}

func TestConcurrentLedgerReservationsDoNotOversubscribe(t *testing.T) {
	ctx := context.Background()
	db, ref := openLedgerDB(t, filepath.Join(t.TempDir(), "concurrent.db"))
	defer db.Close()
	ledger, err := NewLedger(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Grant(ctx, "grant_03", ScopeTask, ref, LedgerBalance{Tokens: 100, CostUSD: 1}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := ledger.Reserve(ctx, ReservationRequest{ID: string(rune('a' + i)), Scope: ScopeTask, Ref: ref, Tokens: 60, CostUSD: .6, TTL: time.Minute})
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			} else if !errors.Is(err, ErrBudgetExceeded) {
				t.Errorf("reserve error=%v", err)
			}
		}(i)
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("successful concurrent reservations=%d, want 1", success)
	}
	balance, err := ledger.Balance(ctx, ScopeTask, ref)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Tokens != 40 || balance.CostUSD < .399999 || balance.CostUSD > .400001 {
		t.Fatalf("balance=%#v", balance)
	}
}

func openLedgerDB(t *testing.T, path string) (*sql.DB, ScopeRef) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`INSERT INTO workspaces(workspace_id,name) VALUES('ws_demo','Demo')`, `INSERT INTO agents(workspace_id,agent_id,display_name,role,status) VALUES('ws_demo','researcher','Researcher','research','idle')`, `INSERT INTO runs(workspace_id,run_id,goal) VALUES('ws_demo','RUN-001','goal')`, `INSERT INTO tasks(workspace_id,run_id,task_id,title,status) VALUES('ws_demo','RUN-001','TASK-001','task','pending')`} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return db, ScopeRef{WorkspaceID: "ws_demo", RunID: "RUN-001", TaskID: "TASK-001", AgentID: "researcher"}
}
