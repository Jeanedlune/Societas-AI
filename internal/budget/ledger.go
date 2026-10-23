package budget

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Jeanedlune/Societas-AI/internal/ids"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
	modernsqlite "modernc.org/sqlite"
)

var (
	ErrReservationConflict = errors.New("budget reservation ID conflicts with an existing request")
	ErrReservationState    = errors.New("budget reservation is not active")
)

// RecoveryEventAppender writes the canonical budget.settled recovery event in
// the same transaction as the expired lease refund.
type RecoveryEventAppender interface {
	AppendBudgetRecovery(context.Context, *sql.Tx, Reservation) error
}

type Ledger struct {
	db       *sql.DB
	recovery RecoveryEventAppender
}

type ScopeRef struct {
	WorkspaceID string
	RunID       string
	TaskID      string
	AgentID     string
}

type ReservationRequest struct {
	ID      string
	Scope   Scope
	Ref     ScopeRef
	Tokens  int64
	CostUSD float64
	TTL     time.Duration
	Now     time.Time
}

type Reservation struct {
	ID        string
	Scope     Scope
	Ref       ScopeRef
	Tokens    int64
	CostUSD   float64
	Status    string
	ExpiresAt time.Time
	Duplicate bool
}

type LedgerBalance struct {
	Tokens  int64
	CostUSD float64
}

func NewLedger(db *sql.DB, recovery ...RecoveryEventAppender) (*Ledger, error) {
	if db == nil {
		return nil, fmt.Errorf("budget ledger requires database")
	}
	if len(recovery) > 1 {
		return nil, fmt.Errorf("at most one recovery event appender may be provided")
	}
	ledger := &Ledger{db: db}
	if len(recovery) == 1 {
		ledger.recovery = recovery[0]
	}
	return ledger, nil
}

func (l *Ledger) Grant(ctx context.Context, id string, scope Scope, ref ScopeRef, amount LedgerBalance, now time.Time) (bool, error) {
	if id == "" {
		return false, fmt.Errorf("grant ID is required")
	}
	if err := validateScopeRef(scope, ref); err != nil {
		return false, err
	}
	if err := validateBalance(amount); err != nil {
		return false, err
	}
	now = utcNow(now)
	duplicate := false
	err := l.withTxRetry(ctx, func(tx *sql.Tx) error {
		var workspace, run, task, agent, existingScope string
		var tokens int64
		var cost float64
		err := tx.QueryRowContext(ctx, `SELECT workspace_id,run_id,COALESCE(task_id,''),COALESCE(agent_id,''),scope,amount_tokens,amount_cost_usd FROM budget_ledger WHERE ledger_id=?`, id).Scan(&workspace, &run, &task, &agent, &existingScope, &tokens, &cost)
		if err == nil {
			if workspace != ref.WorkspaceID || run != ref.RunID || task != ref.TaskID || agent != ref.AgentID || existingScope != string(scope) || tokens != amount.Tokens || cost != amount.CostUSD {
				return ErrReservationConflict
			}
			duplicate = true
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		balance, err := balanceOrZeroTx(ctx, tx, scope, ref)
		if err != nil {
			return err
		}
		balance.Tokens += amount.Tokens
		balance.CostUSD += amount.CostUSD
		return insertLedger(ctx, tx, id, scope, ref, "grant", "", amount.Tokens, amount.CostUSD, balance, now)
	})
	return duplicate, err
}

// Reserve atomically reduces available tokens/cost and creates a durable TTL lease.
// Callers must supply a stable reservation ID so retries are idempotent.
func (l *Ledger) Reserve(ctx context.Context, request ReservationRequest) (Reservation, error) {
	if request.ID == "" {
		return Reservation{}, fmt.Errorf("reservation ID is required")
	}
	if err := validateScopeRef(request.Scope, request.Ref); err != nil {
		return Reservation{}, err
	}
	if request.Tokens < 0 || math.IsNaN(request.CostUSD) || math.IsInf(request.CostUSD, 0) || request.CostUSD < 0 {
		return Reservation{}, fmt.Errorf("reservation estimate must be finite and non-negative")
	}
	if request.TTL == 0 {
		request.TTL = 5 * time.Minute
	}
	if request.TTL < time.Second {
		return Reservation{}, fmt.Errorf("reservation TTL must be at least one second")
	}
	now := utcNow(request.Now)
	expires := now.Add(request.TTL)
	duplicate := false
	err := l.withTxRetry(ctx, func(tx *sql.Tx) error {
		var workspace, run, task, agent, status, expiresAt string
		var scope string
		var tokens int64
		var cost float64
		err := tx.QueryRowContext(ctx, `SELECT l.workspace_id,l.run_id,l.task_id,l.agent_id,b.scope,l.reserved_tokens,l.reserved_cost_usd,l.lease_status,l.expires_at FROM budget_reservation_lease l JOIN budget_ledger b ON b.reservation_id=l.reservation_id AND b.entry_type='reserve' WHERE l.reservation_id=?`, request.ID).Scan(&workspace, &run, &task, &agent, &scope, &tokens, &cost, &status, &expiresAt)
		if err == nil {
			if workspace != request.Ref.WorkspaceID || run != request.Ref.RunID || task != request.Ref.TaskID || agent != request.Ref.AgentID || scope != string(request.Scope) || tokens != request.Tokens || cost != request.CostUSD {
				return ErrReservationConflict
			}
			duplicate = true
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		balance, err := balanceTx(ctx, tx, request.Scope, request.Ref)
		if err != nil {
			return err
		}
		if request.Tokens > balance.Tokens || request.CostUSD > balance.CostUSD+1e-12 {
			return ErrBudgetExceeded
		}
		balance.Tokens -= request.Tokens
		balance.CostUSD -= request.CostUSD
		ttlSeconds := int64(math.Ceil(request.TTL.Seconds()))
		if _, err := tx.ExecContext(ctx, `INSERT INTO budget_reservation_lease(reservation_id,workspace_id,run_id,task_id,agent_id,reserved_tokens,reserved_cost_usd,lease_status,lease_ttl_seconds,expires_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,'active',?,?,?,?)`, request.ID, request.Ref.WorkspaceID, request.Ref.RunID, request.Ref.TaskID, request.Ref.AgentID, request.Tokens, request.CostUSD, ttlSeconds, expires.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("insert reservation lease: %w", err)
		}
		return insertLedger(ctx, tx, "", request.Scope, request.Ref, "reserve", request.ID, -request.Tokens, -request.CostUSD, balance, now)
	})
	if err != nil {
		return Reservation{}, err
	}
	status := "active"
	var existingExpires time.Time
	if duplicate {
		var raw string
		_ = l.db.QueryRowContext(ctx, `SELECT lease_status,expires_at FROM budget_reservation_lease WHERE reservation_id=?`, request.ID).Scan(&status, &raw)
		existingExpires, _ = time.Parse(time.RFC3339Nano, raw)
	}
	if duplicate && !existingExpires.IsZero() {
		expires = existingExpires
	}
	return Reservation{ID: request.ID, Scope: request.Scope, Ref: request.Ref, Tokens: request.Tokens, CostUSD: request.CostUSD, Status: status, ExpiresAt: expires, Duplicate: duplicate}, nil
}

// Settle consumes actual usage and refunds the unused part in the same transaction.
func (l *Ledger) Settle(ctx context.Context, reservationID string, actual LedgerBalance, now time.Time) (bool, error) {
	if reservationID == "" {
		return false, fmt.Errorf("reservation ID is required")
	}
	if err := validateBalance(actual); err != nil {
		return false, err
	}
	now = utcNow(now)
	duplicate := false
	err := l.withTxRetry(ctx, func(tx *sql.Tx) error {
		var ref ScopeRef
		var scope, status string
		var reservedTokens int64
		var reservedCost float64
		err := tx.QueryRowContext(ctx, `SELECT l.workspace_id,l.run_id,l.task_id,l.agent_id,b.scope,l.reserved_tokens,l.reserved_cost_usd,l.lease_status FROM budget_reservation_lease l JOIN budget_ledger b ON b.reservation_id=l.reservation_id AND b.entry_type='reserve' WHERE l.reservation_id=?`, reservationID).Scan(&ref.WorkspaceID, &ref.RunID, &ref.TaskID, &ref.AgentID, &scope, &reservedTokens, &reservedCost, &status)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrReservationState
		}
		if err != nil {
			return err
		}
		if status == "settled" {
			var tokens int64
			var cost float64
			err := tx.QueryRowContext(ctx, `SELECT amount_tokens,amount_cost_usd FROM budget_ledger WHERE reservation_id=? AND entry_type='settle'`, reservationID).Scan(&tokens, &cost)
			if err != nil {
				return err
			}
			if tokens != actual.Tokens || cost != actual.CostUSD {
				return ErrReservationConflict
			}
			duplicate = true
			return nil
		}
		if status != "active" {
			return ErrReservationState
		}
		if actual.Tokens > reservedTokens || actual.CostUSD > reservedCost+1e-12 {
			return ErrBudgetExceeded
		}
		balance, err := balanceTx(ctx, tx, Scope(scope), ref)
		if err != nil {
			return err
		}
		if err := insertLedger(ctx, tx, "", Scope(scope), ref, "settle", reservationID, actual.Tokens, actual.CostUSD, balance, now); err != nil {
			return err
		}
		refundTokens := reservedTokens - actual.Tokens
		refundCost := reservedCost - actual.CostUSD
		if refundTokens > 0 || refundCost > 1e-12 {
			balance.Tokens += refundTokens
			balance.CostUSD += refundCost
			if err := insertLedger(ctx, tx, "", Scope(scope), ref, "refund", reservationID, refundTokens, refundCost, balance, now); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE budget_reservation_lease SET lease_status='settled',updated_at=? WHERE reservation_id=? AND lease_status='active'`, now.Format(time.RFC3339Nano), reservationID)
		return err
	})
	return duplicate, err
}

// RecoverExpired returns reservations whose provider call did not settle before lease expiry.
// Each lease transition and refund is atomic, so repeated sweeps are harmless.
func (l *Ledger) RecoverExpired(ctx context.Context, now time.Time) ([]Reservation, error) {
	now = utcNow(now)
	rows, err := l.db.QueryContext(ctx, `SELECT l.reservation_id,l.workspace_id,l.run_id,l.task_id,l.agent_id,b.scope,l.reserved_tokens,l.reserved_cost_usd,l.expires_at FROM budget_reservation_lease l JOIN budget_ledger b ON b.reservation_id=l.reservation_id AND b.entry_type='reserve' WHERE l.lease_status='active' AND l.expires_at<=? ORDER BY l.expires_at,l.reservation_id`, now.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	type pending struct {
		r   Reservation
		exp string
	}
	var found []pending
	for rows.Next() {
		var p pending
		var scope string
		if err := rows.Scan(&p.r.ID, &p.r.Ref.WorkspaceID, &p.r.Ref.RunID, &p.r.Ref.TaskID, &p.r.Ref.AgentID, &scope, &p.r.Tokens, &p.r.CostUSD, &p.exp); err != nil {
			rows.Close()
			return nil, err
		}
		p.r.Scope = Scope(scope)
		p.r.Status = "expired"
		found = append(found, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	recovered := make([]Reservation, 0, len(found))
	for _, item := range found {
		p := item
		didRecover := false
		err := l.withTxRetry(ctx, func(tx *sql.Tx) error {
			var status string
			if err := tx.QueryRowContext(ctx, `SELECT lease_status FROM budget_reservation_lease WHERE reservation_id=?`, p.r.ID).Scan(&status); err != nil {
				return err
			}
			if status != "active" {
				return nil
			}
			balance, err := balanceTx(ctx, tx, p.r.Scope, p.r.Ref)
			if err != nil {
				return err
			}
			balance.Tokens += p.r.Tokens
			balance.CostUSD += p.r.CostUSD
			if err := insertLedger(ctx, tx, "", p.r.Scope, p.r.Ref, "refund", p.r.ID, p.r.Tokens, p.r.CostUSD, balance, now); err != nil {
				return err
			}
			if l.recovery != nil {
				if err := l.recovery.AppendBudgetRecovery(ctx, tx, p.r); err != nil {
					return fmt.Errorf("append budget recovery event: %w", err)
				}
			}
			_, err = tx.ExecContext(ctx, `UPDATE budget_reservation_lease SET lease_status='expired',updated_at=? WHERE reservation_id=? AND lease_status='active'`, now.Format(time.RFC3339Nano), p.r.ID)
			if err == nil {
				didRecover = true
			}
			return err
		})
		if err != nil {
			return recovered, err
		}
		if didRecover {
			p.r.ExpiresAt, _ = time.Parse(time.RFC3339Nano, p.exp)
			recovered = append(recovered, p.r)
		}
	}
	return recovered, nil
}

func (l *Ledger) Balance(ctx context.Context, scope Scope, ref ScopeRef) (LedgerBalance, error) {
	if err := validateScopeRef(scope, ref); err != nil {
		return LedgerBalance{}, err
	}
	return balanceQuery(ctx, l.db, scope, ref)
}

func (l *Ledger) Reconstruct(ctx context.Context, scope Scope, ref ScopeRef) (LedgerBalance, error) {
	if err := validateScopeRef(scope, ref); err != nil {
		return LedgerBalance{}, err
	}
	where, args, err := scopePredicate(scope, ref)
	if err != nil {
		return LedgerBalance{}, err
	}
	rows, err := l.db.QueryContext(ctx, `SELECT entry_type,amount_tokens,amount_cost_usd FROM budget_ledger WHERE `+where+` ORDER BY rowid`, args...)
	if err != nil {
		return LedgerBalance{}, err
	}
	defer rows.Close()
	var balance LedgerBalance
	for rows.Next() {
		var typ string
		var tokens int64
		var cost float64
		if err := rows.Scan(&typ, &tokens, &cost); err != nil {
			return LedgerBalance{}, err
		}
		if typ != "settle" {
			balance.Tokens += tokens
			balance.CostUSD += cost
		}
	}
	return balance, rows.Err()
}

func (l *Ledger) withTxRetry(ctx context.Context, fn func(*sql.Tx) error) error {
	delay := 5 * time.Millisecond
	for {
		err := storage.WithTx(ctx, l.db, nil, fn)
		if err == nil || !isSQLiteBusy(err) {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if delay < 100*time.Millisecond {
			delay *= 2
			if delay > 100*time.Millisecond {
				delay = 100 * time.Millisecond
			}
		}
	}
}

func balanceTx(ctx context.Context, tx *sql.Tx, scope Scope, ref ScopeRef) (LedgerBalance, error) {
	return balanceQuery(ctx, tx, scope, ref)
}
func balanceOrZeroTx(ctx context.Context, tx *sql.Tx, scope Scope, ref ScopeRef) (LedgerBalance, error) {
	balance, err := balanceQuery(ctx, tx, scope, ref)
	if errors.Is(err, ErrBudgetExceeded) {
		return LedgerBalance{}, nil
	}
	return balance, err
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func balanceQuery(ctx context.Context, q queryRower, scope Scope, ref ScopeRef) (LedgerBalance, error) {
	where, args, err := scopePredicate(scope, ref)
	if err != nil {
		return LedgerBalance{}, err
	}
	var b LedgerBalance
	err = q.QueryRowContext(ctx, `SELECT balance_tokens,balance_cost_usd FROM budget_ledger WHERE `+where+` ORDER BY rowid DESC LIMIT 1`, args...).Scan(&b.Tokens, &b.CostUSD)
	if errors.Is(err, sql.ErrNoRows) {
		return b, ErrBudgetExceeded
	}
	return b, err
}

func insertLedger(ctx context.Context, tx *sql.Tx, id string, scope Scope, ref ScopeRef, entry, reservation string, tokens int64, cost float64, balance LedgerBalance, now time.Time) error {
	if id == "" {
		var err error
		id, err = newID("led_")
		if err != nil {
			return err
		}
	}
	var reservationValue any
	if reservation != "" {
		reservationValue = reservation
	}
	var taskValue any
	if ref.TaskID != "" {
		taskValue = ref.TaskID
	}
	var agentValue any
	if ref.AgentID != "" {
		agentValue = ref.AgentID
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO budget_ledger(ledger_id,workspace_id,run_id,task_id,agent_id,scope,entry_type,reservation_id,amount_tokens,amount_cost_usd,balance_tokens,balance_cost_usd,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, ref.WorkspaceID, ref.RunID, taskValue, agentValue, scope, entry, reservationValue, tokens, cost, balance.Tokens, balance.CostUSD, now.Format(time.RFC3339Nano))
	return err
}

func scopePredicate(scope Scope, ref ScopeRef) (string, []any, error) {
	where := `workspace_id=? AND run_id=? AND scope=?`
	args := []any{ref.WorkspaceID, ref.RunID, scope}
	switch scope {
	case ScopeWorkspace:
		where = `workspace_id=? AND scope=?`
		args = []any{ref.WorkspaceID, scope}
	case ScopeRun:
	case ScopeTask:
		where += ` AND task_id=?`
		args = append(args, ref.TaskID)
	case ScopeAgent:
		where += ` AND agent_id=?`
		args = append(args, ref.AgentID)
	default:
		return "", nil, fmt.Errorf("unknown budget scope %q", scope)
	}
	return where, args, nil
}

func validateScopeRef(scope Scope, ref ScopeRef) error {
	if ref.WorkspaceID == "" || ref.RunID == "" {
		return fmt.Errorf("workspace and run IDs are required")
	}
	switch scope {
	case ScopeWorkspace, ScopeRun:
	case ScopeTask:
		if ref.TaskID == "" {
			return fmt.Errorf("task ID is required for task budget scope")
		}
	case ScopeAgent:
		if ref.AgentID == "" {
			return fmt.Errorf("agent ID is required for agent budget scope")
		}
	default:
		return fmt.Errorf("unknown budget scope %q", scope)
	}
	return nil
}
func validateBalance(b LedgerBalance) error {
	if b.Tokens < 0 || math.IsNaN(b.CostUSD) || math.IsInf(b.CostUSD, 0) || b.CostUSD < 0 {
		return fmt.Errorf("budget amount must be finite and non-negative")
	}
	return nil
}
func utcNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}
func newID(prefix string) (string, error) { return ids.New(prefix) }
func isSQLiteBusy(err error) bool {
	var sqliteErr *modernsqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	code := sqliteErr.Code() & 0xff
	return code == 5 || code == 6
}
