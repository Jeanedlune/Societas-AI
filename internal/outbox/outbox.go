// Package outbox dispatches only events committed to the SQLite Event Store.
package outbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Jeanedlune/Societas-AI/internal/eventstore"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

var ErrLeaseLost = errors.New("outbox delivery lease was lost")

type Record struct {
	EventID        string
	WorkspaceID    string
	RunID          string
	Sequence       uint64
	Attempts       int
	LeaseOwner     string
	LeaseExpiresAt time.Time
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("outbox requires a database")
	}
	return &Store{db: db}, nil
}

// Claim leases the earliest eligible event, preserving order within each run.
func (s *Store) Claim(ctx context.Context, owner string, lease time.Duration, maxAttempts int) (*Record, error) {
	if owner == "" || lease <= 0 || maxAttempts < 1 {
		return nil, fmt.Errorf("owner, positive lease, and positive max attempts are required")
	}
	now := time.Now().UTC()
	nowText := timestamp(now)
	expires := now.Add(lease)
	var claimed *Record
	err := storage.WithTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE event_outbox SET delivery_state='dead', lease_owner=NULL,
			lease_expires_at=NULL, last_error_code='MAX_ATTEMPTS_EXCEEDED', updated_at=?
			WHERE delivery_state='leased' AND lease_expires_at<=? AND attempts>=?`, nowText, nowText, maxAttempts); err != nil {
			return err
		}
		var row Record
		var seq int64
		err := tx.QueryRowContext(ctx, `SELECT o.event_id,o.workspace_id,o.run_id,o.sequence,o.attempts
			FROM event_outbox o
			WHERE o.attempts < ? AND ((o.delivery_state='pending' AND o.available_at<=?) OR
			(o.delivery_state='leased' AND o.lease_expires_at<=?))
			AND NOT EXISTS (SELECT 1 FROM event_outbox p WHERE p.workspace_id=o.workspace_id AND p.run_id=o.run_id
				AND p.sequence<o.sequence AND p.delivery_state!='delivered')
			ORDER BY o.available_at,o.created_at,o.workspace_id,o.run_id,o.sequence LIMIT 1`, maxAttempts, nowText, nowText).
			Scan(&row.EventID, &row.WorkspaceID, &row.RunID, &seq, &row.Attempts)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		row.Sequence = uint64(seq)
		row.LeaseOwner = owner
		row.LeaseExpiresAt = expires
		row.Attempts++
		result, err := tx.ExecContext(ctx, `UPDATE event_outbox SET delivery_state='leased',lease_owner=?,lease_expires_at=?,attempts=?,updated_at=?
			WHERE event_id=? AND ((delivery_state='pending' AND available_at<=?) OR (delivery_state='leased' AND lease_expires_at<=?))`,
			owner, timestamp(expires), row.Attempts, nowText, row.EventID, nowText, nowText)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrLeaseLost
		}
		claimed = &row
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim outbox event: %w", err)
	}
	return claimed, nil
}

func (s *Store) MarkDelivered(ctx context.Context, record *Record) error {
	if record == nil {
		return fmt.Errorf("delivery record is required")
	}
	now := timestamp(time.Now())
	result, err := s.db.ExecContext(ctx, `UPDATE event_outbox SET delivery_state='delivered',delivered_at=?,lease_owner=NULL,lease_expires_at=NULL,last_error_code=NULL,updated_at=? WHERE event_id=? AND delivery_state='leased' AND lease_owner=? AND lease_expires_at=?`, now, now, record.EventID, record.LeaseOwner, timestamp(record.LeaseExpiresAt))
	if err != nil {
		return fmt.Errorf("mark outbox event delivered: %w", err)
	}
	return requireOne(result)
}

func (s *Store) RecordFailure(ctx context.Context, record *Record, retryAt time.Time, code string, maxAttempts int) error {
	if record == nil {
		return fmt.Errorf("delivery record is required")
	}
	if code == "" {
		code = "DELIVERY_FAILED"
	}
	now := timestamp(time.Now())
	state := "pending"
	available := timestamp(retryAt)
	if record.Attempts >= maxAttempts {
		state = "dead"
		available = now
	}
	result, err := s.db.ExecContext(ctx, `UPDATE event_outbox SET delivery_state=?,available_at=?,lease_owner=NULL,lease_expires_at=NULL,last_error_code=?,updated_at=? WHERE event_id=? AND delivery_state='leased' AND lease_owner=? AND lease_expires_at=?`, state, available, code, now, record.EventID, record.LeaseOwner, timestamp(record.LeaseExpiresAt))
	if err != nil {
		return fmt.Errorf("record outbox delivery failure: %w", err)
	}
	return requireOne(result)
}

func (s *Store) RetryDead(ctx context.Context, eventID string) error {
	now := timestamp(time.Now())
	result, err := s.db.ExecContext(ctx, `UPDATE event_outbox SET delivery_state='pending',attempts=0,available_at=?,last_error_code=NULL,updated_at=? WHERE event_id=? AND delivery_state='dead'`, now, now, eventID)
	if err != nil {
		return fmt.Errorf("retry dead outbox event: %w", err)
	}
	return requireOne(result)
}

func requireOne(result sql.Result) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrLeaseLost
	}
	return nil
}

func timestamp(value time.Time) string { return value.UTC().Format("2006-01-02T15:04:05.000000000Z") }

type Deliverer interface {
	Deliver(context.Context, eventstore.StoredEvent) error
}
type deliveryCode interface{ DeliveryCode() string }

type Dispatcher struct {
	Outbox       *Store
	Events       *eventstore.Store
	Deliverer    Deliverer
	Owner        string
	Lease        time.Duration
	MaxAttempts  int
	BaseBackoff  time.Duration
	MaxBackoff   time.Duration
	PollInterval time.Duration
	OnError      func(error)
}

func (d *Dispatcher) DispatchOne(ctx context.Context) (bool, error) {
	if d.Outbox == nil || d.Events == nil || d.Deliverer == nil {
		return false, fmt.Errorf("dispatcher requires outbox, event store, and deliverer")
	}
	if d.Owner == "" || d.Lease <= 0 || d.MaxAttempts < 1 {
		return false, fmt.Errorf("dispatcher owner, lease, and max attempts are required")
	}
	record, err := d.Outbox.Claim(ctx, d.Owner, d.Lease, d.MaxAttempts)
	if err != nil || record == nil {
		return false, err
	}
	event, err := d.Events.GetByID(ctx, record.EventID)
	if err == nil {
		err = d.Deliverer.Deliver(ctx, event)
	}
	if err == nil {
		if markErr := d.Outbox.MarkDelivered(ctx, record); markErr != nil {
			return true, markErr
		}
		return true, nil
	}
	code := "DELIVERY_FAILED"
	var coded deliveryCode
	if errors.As(err, &coded) && coded.DeliveryCode() != "" {
		code = coded.DeliveryCode()
	}
	if record.Attempts >= d.MaxAttempts {
		if saveErr := d.Outbox.RecordFailure(ctx, record, time.Now(), code, d.MaxAttempts); saveErr != nil {
			return true, saveErr
		}
		return true, err
	}
	delay := d.BaseBackoff
	if delay <= 0 {
		delay = 100 * time.Millisecond
	}
	max := d.MaxBackoff
	if max <= 0 {
		max = 30 * time.Second
	}
	for i := 1; i < record.Attempts && delay < max; i++ {
		if delay > max/2 {
			delay = max
			break
		}
		delay *= 2
	}
	if delay > max {
		delay = max
	}
	if saveErr := d.Outbox.RecordFailure(ctx, record, time.Now().Add(delay), code, d.MaxAttempts); saveErr != nil {
		return true, saveErr
	}
	return true, err
}

func (d *Dispatcher) Run(ctx context.Context) error {
	interval := d.PollInterval
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	for {
		worked, err := d.DispatchOne(ctx)
		if err != nil && d.OnError != nil {
			d.OnError(err)
		}
		if worked {
			continue
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
