package eventstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Jeanedlune/Societas-AI/internal/contracts/v1"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
	modernsqlite "modernc.org/sqlite"
)

const maxReadLimit = 10000

// Store persists validated v1 events. It has no API for updating or deleting
// events; successful Append returns only after the SQLite commit succeeds.
type Store struct {
	db        *sql.DB
	validator *v1.Validator
}

// StoredEvent is a validated event with the sequence assigned by the store.
type StoredEvent struct {
	Event    v1.ValidatedEvent
	Sequence uint64
	Raw      json.RawMessage
}

// AppendResult returns the previously persisted event for a duplicate event
// ID or idempotency key, with Duplicate set to true.
type AppendResult struct {
	Event     StoredEvent
	Duplicate bool
}

// New creates an Event Store over a migrated SQLite database.
func New(db *sql.DB, validator *v1.Validator) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("event store requires a database")
	}
	if validator == nil {
		return nil, fmt.Errorf("event store requires a contract validator")
	}
	return &Store{db: db, validator: validator}, nil
}

// Append validates and durably stores one envelope. It assigns the next
// sequence in the workspace/run stream. A duplicate event ID or idempotency
// key returns its existing record without creating a second row.
func (s *Store) Append(ctx context.Context, rawEnvelope []byte) (AppendResult, error) {
	validated, err := s.validator.ValidateEnvelope(rawEnvelope)
	if err != nil {
		return AppendResult{}, err
	}
	if validated.Header.Sequence != nil {
		return AppendResult{}, ErrSequenceAlreadyAssigned
	}

	backoff := 5 * time.Millisecond
	for {
		result, err := s.appendOnce(ctx, validated)
		if err == nil {
			return result, nil
		}
		if previous, found, lookupErr := s.findDuplicate(ctx, s.db, validated.Header); lookupErr == nil && found {
			return AppendResult{Event: previous, Duplicate: true}, nil
		}
		if !isSQLiteBusy(err) {
			return AppendResult{}, fmt.Errorf("append event: %w", err)
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return AppendResult{}, fmt.Errorf("append event stopped while waiting for sqlite: %w", ctx.Err())
		case <-timer.C:
		}
		if backoff < 100*time.Millisecond {
			backoff *= 2
			if backoff > 100*time.Millisecond {
				backoff = 100 * time.Millisecond
			}
		}
	}
}

func (s *Store) appendOnce(ctx context.Context, incoming *v1.ValidatedEvent) (AppendResult, error) {
	var result AppendResult
	err := storage.WithTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		previous, found, err := s.findDuplicate(ctx, tx, incoming.Header)
		if err != nil {
			return err
		}
		if found {
			result = AppendResult{Event: previous, Duplicate: true}
			return nil
		}

		header := incoming.Header
		var current int64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence), 0) FROM events WHERE workspace_id=? AND run_id=?`, header.WorkspaceID, header.RunID).Scan(&current); err != nil {
			return fmt.Errorf("read run sequence: %w", err)
		}
		if current == math.MaxInt64 {
			return ErrSequenceExhausted
		}
		sequence := uint64(current + 1)
		header.Sequence = &sequence
		envelopeJSON, err := json.Marshal(header)
		if err != nil {
			return fmt.Errorf("encode stored event envelope: %w", err)
		}
		var idempotency any
		if header.IdempotencyKey != "" {
			idempotency = header.IdempotencyKey
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO events (
			event_id, workspace_id, run_id, task_id, sequence, event_type, schema_version,
			correlation_id, causation_id, idempotency_key, payload_json, envelope_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			header.ID, header.WorkspaceID, header.RunID, nullableString(header.TaskID), sequence,
			header.Type, header.SchemaVersion, header.CorrelationID, nullableString(header.CausationID),
			idempotency, string(header.Payload), string(envelopeJSON), header.Timestamp,
		)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO event_outbox (
			event_id, workspace_id, run_id, sequence, delivery_state, attempts,
			available_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, 'pending', 0, ?, ?, ?)`, header.ID, header.WorkspaceID,
			header.RunID, sequence, header.Timestamp, header.Timestamp, header.Timestamp)
		if err != nil {
			return fmt.Errorf("insert event outbox row: %w", err)
		}
		stored, err := s.decodeStored(sequence, envelopeJSON)
		if err != nil {
			return err
		}
		result = AppendResult{Event: stored}
		return nil
	})
	if err != nil {
		return AppendResult{}, err
	}
	return result, nil
}

// ReadRun returns events in sequence order strictly after afterSequence.
func (s *Store) ReadRun(ctx context.Context, workspaceID, runID string, afterSequence uint64, limit int) ([]StoredEvent, error) {
	if limit < 1 || limit > maxReadLimit {
		return nil, fmt.Errorf("event read limit must be between 1 and %d", maxReadLimit)
	}
	if afterSequence > math.MaxInt64 {
		return nil, fmt.Errorf("event sequence is out of range")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT sequence, envelope_json FROM events
		WHERE workspace_id=? AND run_id=? AND sequence>?
		ORDER BY sequence ASC LIMIT ?`, workspaceID, runID, afterSequence, limit)
	if err != nil {
		return nil, fmt.Errorf("read run events: %w", err)
	}
	defer rows.Close()
	events := make([]StoredEvent, 0)
	for rows.Next() {
		var sequence int64
		var raw []byte
		if err := rows.Scan(&sequence, &raw); err != nil {
			return nil, fmt.Errorf("scan run event: %w", err)
		}
		if sequence < 1 {
			return nil, ErrCorruptEventRecord
		}
		event, err := s.decodeStored(uint64(sequence), raw)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run events: %w", err)
	}
	return events, nil
}

// GetBySequence looks up one event by workspace, run, and assigned sequence.
func (s *Store) GetBySequence(ctx context.Context, workspaceID, runID string, sequence uint64) (StoredEvent, error) {
	if sequence < 1 || sequence > math.MaxInt64 {
		return StoredEvent{}, sql.ErrNoRows
	}
	return s.getOne(ctx, `SELECT sequence, envelope_json FROM events WHERE workspace_id=? AND run_id=? AND sequence=?`, workspaceID, runID, sequence)
}

// GetByID looks up one event by its globally unique event ID.
func (s *Store) GetByID(ctx context.Context, eventID string) (StoredEvent, error) {
	return s.getOne(ctx, `SELECT sequence, envelope_json FROM events WHERE event_id=?`, eventID)
}

func (s *Store) getOne(ctx context.Context, query string, args ...any) (StoredEvent, error) {
	var sequence int64
	var raw []byte
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&sequence, &raw); err != nil {
		return StoredEvent{}, err
	}
	if sequence < 1 {
		return StoredEvent{}, ErrCorruptEventRecord
	}
	return s.decodeStored(uint64(sequence), raw)
}

func (s *Store) decodeStored(sequence uint64, raw []byte) (StoredEvent, error) {
	validated, err := s.validator.ValidateEnvelope(raw)
	if err != nil {
		return StoredEvent{}, fmt.Errorf("stored event failed contract validation: %w", ErrCorruptEventRecord)
	}
	if validated.Header.Sequence == nil || *validated.Header.Sequence != sequence {
		return StoredEvent{}, ErrCorruptEventRecord
	}
	return StoredEvent{
		Event:    *validated,
		Sequence: sequence,
		Raw:      append(json.RawMessage(nil), raw...),
	}, nil
}

type rowQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Store) findDuplicate(ctx context.Context, queryer rowQueryer, incoming v1.Envelope) (StoredEvent, bool, error) {
	if event, err := s.getDuplicate(ctx, queryer, `SELECT sequence, envelope_json FROM events WHERE event_id=?`, incoming.ID); err == nil {
		return event, true, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return StoredEvent{}, false, err
	}
	if incoming.IdempotencyKey == "" {
		return StoredEvent{}, false, nil
	}
	event, err := s.getDuplicate(ctx, queryer, `SELECT sequence, envelope_json FROM events WHERE workspace_id=? AND run_id=? AND idempotency_key=?`, incoming.WorkspaceID, incoming.RunID, incoming.IdempotencyKey)
	if errors.Is(err, sql.ErrNoRows) {
		return StoredEvent{}, false, nil
	}
	if err != nil {
		return StoredEvent{}, false, err
	}
	return event, true, nil
}

func (s *Store) getDuplicate(ctx context.Context, queryer rowQueryer, query string, args ...any) (StoredEvent, error) {
	var sequence int64
	var raw []byte
	if err := queryer.QueryRowContext(ctx, query, args...).Scan(&sequence, &raw); err != nil {
		return StoredEvent{}, err
	}
	if sequence < 1 {
		return StoredEvent{}, ErrCorruptEventRecord
	}
	return s.decodeStored(uint64(sequence), raw)
}

func nullableString[T ~string](value *T) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func isSQLiteBusy(err error) bool {
	var sqliteErr *modernsqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	code := sqliteErr.Code() & 0xff
	return code == 5 || code == 6 // SQLITE_BUSY or SQLITE_LOCKED
}

var (
	ErrSequenceAlreadyAssigned = errors.New("event sequence is assigned by the store")
	ErrSequenceExhausted       = errors.New("event sequence exhausted")
	ErrCorruptEventRecord      = errors.New("stored event record is corrupt")
)
