// Package taskstate owns canonical task lifecycle transitions and persistence.
package taskstate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	v1 "github.com/Jeanedlune/Societas-AI/internal/contracts/v1"
	"github.com/Jeanedlune/Societas-AI/internal/eventstore"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

type Status string

const (
	Pending          Status = "pending"
	Ready            Status = "ready"
	Running          Status = "running"
	Pausing          Status = "pausing"
	Paused           Status = "paused"
	Interrupted      Status = "interrupted"
	Blocked          Status = "blocked"
	AwaitingApproval Status = "awaiting_approval"
	Completed        Status = "completed"
	Failed           Status = "failed"
	Cancelled        Status = "cancelled"
)

var (
	ErrInvalidTransition = errors.New("task state transition is not allowed")
	ErrTaskNotFound      = errors.New("task was not found")
	ErrTaskAlreadyExists = errors.New("task already exists")
)

type Store struct {
	db     *sql.DB
	events *eventstore.Store
}

func New(db *sql.DB, events *eventstore.Store) (*Store, error) {
	if db == nil || events == nil {
		return nil, fmt.Errorf("task state requires database and event store")
	}
	return &Store{db: db, events: events}, nil
}

// Create stores a task.created event and its operational row atomically.
func (s *Store) Create(ctx context.Context, raw []byte) (eventstore.AppendResult, error) {
	var envelope v1.Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return eventstore.AppendResult{}, fmt.Errorf("decode task.created envelope: %w", err)
	}
	if envelope.Type != v1.EventTaskCreated || envelope.TaskID == nil {
		return eventstore.AppendResult{}, fmt.Errorf("task creation requires task.created and task_id")
	}
	var payload struct {
		Task struct {
			ID           string          `json:"id"`
			RunID        string          `json:"run_id"`
			ParentTaskID json.RawMessage `json:"parent_task_id"`
			Title        string          `json:"title"`
			Owner        string          `json:"owner"`
			Status       Status          `json:"status"`
			Priority     string          `json:"priority"`
			RiskTier     *string         `json:"risk_tier"`
			CreatedAt    string          `json:"created_at"`
			UpdatedAt    string          `json:"updated_at"`
		} `json:"task"`
	}
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return eventstore.AppendResult{}, fmt.Errorf("decode task.created payload: %w", err)
	}
	t := payload.Task
	if t.ID != string(*envelope.TaskID) || t.RunID != string(envelope.RunID) {
		return eventstore.AppendResult{}, fmt.Errorf("task.created identity differs between envelope and payload")
	}
	if t.Status != Pending && t.Status != Ready {
		return eventstore.AppendResult{}, fmt.Errorf("task.created must use pending or ready status")
	}
	var parent any
	if len(t.ParentTaskID) > 0 && string(t.ParentTaskID) != "null" {
		var value string
		if err := json.Unmarshal(t.ParentTaskID, &value); err != nil {
			return eventstore.AppendResult{}, err
		}
		parent = value
	}
	var result eventstore.AppendResult
	err := storage.WithTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var exists int
		err := tx.QueryRowContext(ctx, `SELECT 1 FROM tasks WHERE workspace_id=? AND run_id=? AND task_id=?`, envelope.WorkspaceID, envelope.RunID, t.ID).Scan(&exists)
		if err == nil {
			result, err = s.events.AppendInTx(ctx, tx, raw)
			if err != nil {
				return err
			}
			if result.Duplicate {
				return nil
			}
			return ErrTaskAlreadyExists
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO tasks(workspace_id,run_id,task_id,parent_task_id,title,status,priority,risk_tier,owner_agent_id,data_json,created_at,updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, envelope.WorkspaceID, envelope.RunID, t.ID, parent, t.Title, t.Status, t.Priority, t.RiskTier, t.Owner, string(envelope.Payload), t.CreatedAt, t.UpdatedAt)
		if err != nil {
			return fmt.Errorf("insert task row: %w", err)
		}
		result, err = s.events.AppendInTx(ctx, tx, raw)
		return err
	})
	if err != nil {
		return eventstore.AppendResult{}, fmt.Errorf("create task: %w", err)
	}
	return result, nil
}

// Transition derives the target state from a canonical lifecycle event, then
// compare-and-swaps SQLite state and appends event/outbox in the same tx.
func (s *Store) Transition(ctx context.Context, raw []byte) (eventstore.AppendResult, error) {
	var envelope v1.Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return eventstore.AppendResult{}, fmt.Errorf("decode task transition envelope: %w", err)
	}
	if envelope.TaskID == nil {
		return eventstore.AppendResult{}, fmt.Errorf("task transition requires task_id")
	}
	next, err := targetStatus(envelope)
	if err != nil {
		return eventstore.AppendResult{}, err
	}
	var result eventstore.AppendResult
	err = storage.WithTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		var err error
		result, err = s.events.AppendInTx(ctx, tx, raw)
		if err != nil {
			return err
		}
		if result.Duplicate {
			return nil
		}
		var current Status
		if err := tx.QueryRowContext(ctx, `SELECT status FROM tasks WHERE workspace_id=? AND run_id=? AND task_id=?`, envelope.WorkspaceID, envelope.RunID, *envelope.TaskID).Scan(&current); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrTaskNotFound
			}
			return err
		}
		if !CanTransition(current, next) {
			return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current, next)
		}
		updated := time.Now().UTC().Format(time.RFC3339Nano)
		var owner any
		if envelope.Type == v1.EventTaskAssigned {
			var assigned struct {
				AgentID string `json:"agent_id"`
			}
			if err := json.Unmarshal(envelope.Payload, &assigned); err != nil {
				return err
			}
			owner = assigned.AgentID
		}
		query := `UPDATE tasks SET status=?,updated_at=? WHERE workspace_id=? AND run_id=? AND task_id=? AND status=?`
		args := []any{next, updated, envelope.WorkspaceID, envelope.RunID, *envelope.TaskID, current}
		if owner != nil {
			query = `UPDATE tasks SET status=?,owner_agent_id=?,updated_at=? WHERE workspace_id=? AND run_id=? AND task_id=? AND status=?`
			args = []any{next, owner, updated, envelope.WorkspaceID, envelope.RunID, *envelope.TaskID, current}
		}
		updatedResult, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return err
		}
		rows, err := updatedResult.RowsAffected()
		if err != nil {
			return err
		}
		if rows != 1 {
			return ErrInvalidTransition
		}
		return nil
	})
	if err != nil {
		return eventstore.AppendResult{}, fmt.Errorf("transition task: %w", err)
	}
	return result, nil
}

func (s *Store) Status(ctx context.Context, workspaceID, runID, taskID string) (Status, error) {
	var status Status
	err := s.db.QueryRowContext(ctx, `SELECT status FROM tasks WHERE workspace_id=? AND run_id=? AND task_id=?`, workspaceID, runID, taskID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTaskNotFound
	}
	return status, err
}

func targetStatus(e v1.Envelope) (Status, error) {
	var payload map[string]json.RawMessage
	_ = json.Unmarshal(e.Payload, &payload)
	switch e.Type {
	case v1.EventTaskAssigned:
		return Ready, nil
	case v1.EventTaskStarted:
		return Running, nil
	case v1.EventTaskBlocked, v1.EventTaskSemanticConflict:
		return Blocked, nil
	case v1.EventTaskPaused:
		var phase string
		_ = json.Unmarshal(payload["phase"], &phase)
		if phase == "requested" {
			return Pausing, nil
		}
		if phase == "completed" {
			return Paused, nil
		}
		return "", ErrInvalidTransition
	case v1.EventTaskResumed:
		return Running, nil
	case v1.EventTaskInterrupted:
		return Interrupted, nil
	case v1.EventTaskCompleted:
		return Completed, nil
	case v1.EventTaskFailed:
		return Failed, nil
	case v1.EventTaskCancelled:
		return Cancelled, nil
	case v1.EventTaskRetryScheduled:
		return Ready, nil
	case v1.EventApprovalRequested:
		return AwaitingApproval, nil
	case v1.EventApprovalGranted, v1.EventApprovalInvalidated:
		return Ready, nil
	case v1.EventApprovalRejected:
		return Failed, nil
	default:
		return "", fmt.Errorf("event %q is not a task state transition", e.Type)
	}
}

func CanTransition(from, to Status) bool {
	allowed := map[Status][]Status{
		Pending: {Ready, Failed, Cancelled}, Ready: {Running, Cancelled},
		Running: {Blocked, AwaitingApproval, Pausing, Paused, Interrupted, Ready, Completed, Failed, Cancelled},
		Pausing: {Paused, Running, Interrupted, Failed, Cancelled}, Paused: {Running, Cancelled},
		Interrupted: {Running, Ready, Failed, Cancelled}, Blocked: {Running, Failed, Cancelled},
		AwaitingApproval: {Ready, Failed, Cancelled},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return true
		}
	}
	return false
}
