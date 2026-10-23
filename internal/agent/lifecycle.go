package agent

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

type LifecycleStatus string

const (
	AgentCreated          LifecycleStatus = "created"
	AgentIdle             LifecycleStatus = "idle"
	AgentWorking          LifecycleStatus = "working"
	AgentWaiting          LifecycleStatus = "waiting"
	AgentBlocked          LifecycleStatus = "blocked"
	AgentApprovalRequired LifecycleStatus = "approval_required"
	AgentCompleted        LifecycleStatus = "completed"
	AgentStopped          LifecycleStatus = "stopped"
)

var (
	ErrInvalidAgentTransition = errors.New("invalid agent lifecycle transition")
	ErrAgentStateConflict     = errors.New("agent lifecycle state changed concurrently")
)

type Lifecycle struct {
	db     *sql.DB
	events *eventstore.Store
}

func NewLifecycle(db *sql.DB, events *eventstore.Store) (*Lifecycle, error) {
	if db == nil || events == nil {
		return nil, fmt.Errorf("agent lifecycle requires database and event store")
	}
	return &Lifecycle{db: db, events: events}, nil
}

// Transition is an Orchestrator-only state command. Agent state, status event,
// and outbox delivery row commit together; task status remains independent.
func (l *Lifecycle) Transition(ctx context.Context, workspaceID, runID, agentID string, from, to LifecycleStatus, rawEvent []byte) (eventstore.AppendResult, error) {
	if !canTransitionAgent(from, to) {
		return eventstore.AppendResult{}, ErrInvalidAgentTransition
	}
	var envelope v1.Envelope
	if err := json.Unmarshal(rawEvent, &envelope); err != nil {
		return eventstore.AppendResult{}, fmt.Errorf("decode agent lifecycle event: %w", err)
	}
	if envelope.Type != v1.EventAgentStatusChanged || envelope.WorkspaceID != v1.WorkspaceID(workspaceID) || envelope.RunID != v1.RunID(runID) || envelope.From != "system:orchestrator" || envelope.To != "topic:all" || envelope.Principal.PrincipalType != v1.PrincipalSystem || envelope.Principal.PrincipalID == "" {
		return eventstore.AppendResult{}, fmt.Errorf("agent lifecycle event identity does not match transition")
	}
	var payload struct {
		AgentID  string `json:"agent_id"`
		Previous string `json:"previous_status"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return eventstore.AppendResult{}, err
	}
	if payload.AgentID != agentID || payload.Previous != string(from) || payload.Status != string(to) {
		return eventstore.AppendResult{}, fmt.Errorf("agent lifecycle payload does not match transition")
	}
	var appended eventstore.AppendResult
	err := storage.WithTx(ctx, l.db, nil, func(tx *sql.Tx) error {
		var err error
		appended, err = l.events.AppendInTx(ctx, tx, rawEvent)
		if err != nil {
			return err
		}
		if appended.Duplicate {
			return nil
		}
		var current string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM agents WHERE workspace_id=? AND agent_id=?`, workspaceID, agentID).Scan(&current); err != nil {
			return err
		}
		if current != string(from) {
			return ErrAgentStateConflict
		}
		result, err := tx.ExecContext(ctx, `UPDATE agents SET status=?,updated_at=? WHERE workspace_id=? AND agent_id=? AND status=?`, to, time.Now().UTC().Format(time.RFC3339Nano), workspaceID, agentID, from)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrAgentStateConflict
		}
		return nil
	})
	if err != nil {
		return eventstore.AppendResult{}, fmt.Errorf("transition agent state: %w", err)
	}
	return appended, nil
}

func (l *Lifecycle) Status(ctx context.Context, workspaceID, agentID string) (LifecycleStatus, error) {
	var status LifecycleStatus
	err := l.db.QueryRowContext(ctx, `SELECT status FROM agents WHERE workspace_id=? AND agent_id=?`, workspaceID, agentID).Scan(&status)
	return status, err
}

func canTransitionAgent(from, to LifecycleStatus) bool {
	allowed := map[LifecycleStatus][]LifecycleStatus{
		AgentCreated: {AgentIdle, AgentStopped}, AgentIdle: {AgentWorking, AgentStopped},
		AgentWorking: {AgentWaiting, AgentBlocked, AgentApprovalRequired, AgentCompleted, AgentStopped},
		AgentWaiting: {AgentWorking, AgentIdle, AgentStopped}, AgentBlocked: {AgentWorking, AgentIdle, AgentStopped},
		AgentApprovalRequired: {AgentWorking, AgentIdle, AgentStopped}, AgentCompleted: {AgentIdle, AgentStopped},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return true
		}
	}
	return false
}
