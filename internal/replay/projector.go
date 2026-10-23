// Package replay rebuilds read-only run/task state from immutable events.
package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	v1 "github.com/Jeanedlune/Societas-AI/internal/contracts/v1"
	"github.com/Jeanedlune/Societas-AI/internal/eventstore"
)

type TaskState struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Goal         string `json:"goal"`
	Owner        string `json:"owner,omitempty"`
	Status       string `json:"status"`
	Attempt      int    `json:"attempt"`
	LastEventID  string `json:"last_event_id"`
	LastSequence uint64 `json:"last_sequence"`
}

type RunState struct {
	WorkspaceID string               `json:"workspace_id"`
	RunID       string               `json:"run_id"`
	Goal        string               `json:"goal,omitempty"`
	Status      string               `json:"status,omitempty"`
	Sequence    uint64               `json:"sequence"`
	Tasks       map[string]TaskState `json:"tasks"`
}

// ReplayRun reads every event in pages and deterministically reduces the run
// and task lifecycle events into a snapshot. Unknown event types are ignored.
func ReplayRun(ctx context.Context, store *eventstore.Store, workspaceID, runID string) (RunState, error) {
	if store == nil {
		return RunState{}, fmt.Errorf("event store is required")
	}
	state := RunState{WorkspaceID: workspaceID, RunID: runID, Tasks: make(map[string]TaskState)}
	const pageSize = 10000
	var after uint64
	for {
		page, err := store.ReadRun(ctx, workspaceID, runID, after, pageSize)
		if err != nil {
			return RunState{}, err
		}
		if len(page) == 0 {
			break
		}
		for _, event := range page {
			if event.Sequence <= after {
				return RunState{}, fmt.Errorf("event sequence did not advance")
			}
			if err := apply(&state, event); err != nil {
				return RunState{}, err
			}
			after = event.Sequence
		}
		if len(page) < pageSize {
			break
		}
	}
	return state, nil
}

func (s RunState) CanonicalJSON() ([]byte, error) {
	ids := make([]string, 0, len(s.Tasks))
	for id := range s.Tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	tasks := make([]TaskState, 0, len(ids))
	for _, id := range ids {
		tasks = append(tasks, s.Tasks[id])
	}
	return json.Marshal(struct {
		WorkspaceID string      `json:"workspace_id"`
		RunID       string      `json:"run_id"`
		Goal        string      `json:"goal,omitempty"`
		Status      string      `json:"status,omitempty"`
		Sequence    uint64      `json:"sequence"`
		Tasks       []TaskState `json:"tasks"`
	}{s.WorkspaceID, s.RunID, s.Goal, s.Status, s.Sequence, tasks})
}

func apply(state *RunState, event eventstore.StoredEvent) error {
	h := event.Event.Header
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(h.Payload, &payload); err != nil {
		return fmt.Errorf("decode event %s during replay: %w", h.ID, err)
	}
	state.Sequence = event.Sequence
	switch h.Type {
	case v1.EventRunCreated:
		if err := json.Unmarshal(payload["goal"], &state.Goal); err != nil {
			return fmt.Errorf("decode run.created goal: %w", err)
		}
		state.Status = "created"
	case v1.EventRunStopped:
		state.Status = "stopped"
	case v1.EventTaskCreated:
		var created struct {
			Task struct {
				ID      string `json:"id"`
				Title   string `json:"title"`
				Goal    string `json:"goal"`
				Owner   string `json:"owner"`
				Status  string `json:"status"`
				Attempt int    `json:"attempt"`
			} `json:"task"`
		}
		if err := json.Unmarshal(h.Payload, &created); err != nil {
			return fmt.Errorf("decode task.created: %w", err)
		}
		t := created.Task
		if t.ID == "" {
			return fmt.Errorf("task.created event %s has no task ID", h.ID)
		}
		state.Tasks[t.ID] = TaskState{ID: t.ID, Title: t.Title, Goal: t.Goal, Owner: t.Owner, Status: t.Status, Attempt: t.Attempt, LastEventID: string(h.ID), LastSequence: event.Sequence}
	default:
		taskID := stringValue(payload["task_id"])
		if taskID == "" && h.TaskID != nil {
			taskID = string(*h.TaskID)
		}
		if taskID == "" {
			return nil
		}
		t, ok := state.Tasks[taskID]
		if !ok {
			return nil
		}
		switch h.Type {
		case v1.EventTaskAssigned:
			t.Status = "ready"
			t.Owner = stringValue(payload["agent_id"])
		case v1.EventTaskStarted:
			t.Status = "running"
			t.Attempt = intValue(payload["attempt"])
		case v1.EventTaskBlocked:
			t.Status = "blocked"
		case v1.EventTaskPaused:
			if stringValue(payload["phase"]) == "requested" {
				t.Status = "pausing"
			} else {
				t.Status = "paused"
			}
		case v1.EventTaskResumed:
			t.Status = "running"
		case v1.EventTaskInterrupted:
			t.Status = "interrupted"
		case v1.EventTaskCompleted:
			t.Status = "completed"
		case v1.EventTaskFailed:
			t.Status = "failed"
		case v1.EventTaskCancelled:
			t.Status = "cancelled"
		case v1.EventTaskRetryScheduled:
			t.Status = "ready"
			if attempt := intValue(payload["attempt"]); attempt > 0 {
				t.Attempt = attempt
			}
		case v1.EventApprovalRequested:
			t.Status = "awaiting_approval"
		case v1.EventApprovalGranted, v1.EventApprovalInvalidated:
			t.Status = "ready"
		case v1.EventApprovalRejected:
			t.Status = "failed"
		default:
			return nil
		}
		t.LastEventID = string(h.ID)
		t.LastSequence = event.Sequence
		state.Tasks[taskID] = t
	}
	return nil
}

func stringValue(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return value
}
func intValue(raw json.RawMessage) int { var value int; _ = json.Unmarshal(raw, &value); return value }
