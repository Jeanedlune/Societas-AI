// Package v1 contains versioned wire contracts defined by Doc 10, Section 72A.
// Generated schemas and event registry are extracted from that source.
package v1

import (
	"encoding/json"
	"fmt"
)

type EventType string
type SchemaID string
type ApprovalAction string
type PrincipalType string

const (
	ProtocolVersion = "societas/1"
	SchemaVersion   = "1"
)

const (
	ActionToolExecute    ApprovalAction = "tool.execute"
	ActionBudgetIncrease ApprovalAction = "budget.increase"
)

const (
	PrincipalHuman  PrincipalType = "human"
	PrincipalAgent  PrincipalType = "agent"
	PrincipalSystem PrincipalType = "system"
)

// ID types retain wire prefixes so unrelated identifiers are not interchangeable.
type WorkspaceID string
type RunID string
type TaskID string
type EventID string
type MessageID string
type ConversationID string
type ArtifactID string
type SummaryID string
type ToolCallID string
type ApprovalID string
type CorrelationID string
type AgentID string
type PrincipalID string
type SessionID string

type Principal struct {
	PrincipalID   PrincipalID   `json:"principal_id"`
	PrincipalType PrincipalType `json:"principal_type"`
	SessionID     SessionID     `json:"session_id"`
	AuthDigest    string        `json:"auth_digest"`
}

// Envelope is the shared message/event envelope from Section 72A.3. The
// generated event registry maps Type to exactly one payload schema and type.
type Envelope struct {
	ProtocolVersion string          `json:"protocol_version"`
	SchemaVersion   string          `json:"schema_version"`
	ID              EventID         `json:"id"`
	Type            EventType       `json:"type"`
	Timestamp       string          `json:"ts"`
	Sequence        *uint64         `json:"seq,omitempty"`
	WorkspaceID     WorkspaceID     `json:"workspace_id"`
	RunID           RunID           `json:"run_id"`
	TaskID          *TaskID         `json:"task_id,omitempty"`
	CorrelationID   CorrelationID   `json:"correlation_id"`
	CausationID     *EventID        `json:"causation_id,omitempty"`
	IdempotencyKey  string          `json:"idempotency_key,omitempty"`
	TraceID         string          `json:"trace_id,omitempty"`
	From            string          `json:"from"`
	To              string          `json:"to"`
	Payload         json.RawMessage `json:"payload"`
	Principal       Principal       `json:"principal"`
}

type EventDefinition struct {
	Type            EventType
	PayloadSchemaID string
	From            string
	To              string
}

// DecodedEvent pairs a canonical event header with its payload type.
type DecodedEvent[P any] struct {
	Header  Envelope
	Payload P
}

// DecodePayload allocates and decodes the payload struct assigned to this
// event type. It does not validate the schema; untrusted input must first pass
// Validator.ValidateEnvelope.
func (e Envelope) DecodePayload() (any, error) {
	definition, ok := Definition(e.Type)
	if !ok {
		return nil, fmt.Errorf("unknown event type %q", e.Type)
	}
	payload, err := NewPayload(e.Type)
	if err != nil {
		return nil, err
	}
	if len(e.Payload) == 0 {
		return nil, fmt.Errorf("event %q has no payload", e.Type)
	}
	if err := json.Unmarshal(e.Payload, payload); err != nil {
		return nil, fmt.Errorf("decode payload for %q (%s): %w", e.Type, definition.PayloadSchemaID, err)
	}
	return payload, nil
}
