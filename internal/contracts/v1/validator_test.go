package v1

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidatorAcceptsEventsAcrossMVPCategories(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatalf("NewValidator(): %v", err)
	}

	valid := map[EventType]any{
		EventWorkspaceCreated:  map[string]any{"name": "demo"},
		EventAgentStarted:      map[string]any{"agent_id": "market_researcher"},
		EventRunCreated:        map[string]any{"goal": "Prepare a content strategy"},
		EventTaskStarted:       map[string]any{"attempt": 1},
		EventMessageSent:       map[string]any{"message_id": "msg_01ARZ3NDEKTSV4RRFFQ69G5FAV", "conversation_id": "conv_01ARZ3NDEKTSV4RRFFQ69G5FAV", "text": "Research the market"},
		EventModelSelected:     map[string]any{"agent_id": "market_researcher", "model": "mock", "tier": "cheap", "reason": "fixture"},
		EventBudgetReserved:    map[string]any{"scope": "run", "scope_id": "RUN-001", "reservation_id": "reserve-1", "max_tokens": 1000, "max_cost_usd": 0.1},
		EventPolicyEvaluated:   map[string]any{"action": "tool.call:browser.search", "decision": "allow", "rule_id": "allow-search", "reason": "fixture"},
		EventToolCallRequested: map[string]any{"tool_call_id": "tc_01ARZ3NDEKTSV4RRFFQ69G5FAV", "tool": "browser.search", "arguments": map[string]any{"query": "market"}},
		EventArtifactCreated:   artifactCreatedPayload("content-strategy.md", "TASK-001", "agent:content_writer"),
		EventSummaryCreated:    map[string]any{"summary": map[string]any{"id": "sum_01ARZ3NDEKTSV4RRFFQ69G5FAV", "artifact_id": "art_01ARZ3NDEKTSV4RRFFQ69G5FAV", "artifact_version": 1, "token_estimate": 5, "generated_by": "system:summarizer", "model": "mock", "body": map[string]any{"text": "Short summary"}}},
		EventReviewRequested:   map[string]any{"artifact_ids": []string{"art_01ARZ3NDEKTSV4RRFFQ69G5FAV"}},
		EventApprovalRequested: approvalRequestedPayload(),
	}
	for eventType, payload := range valid {
		t.Run(string(eventType), func(t *testing.T) {
			payloadJSON, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			raw := validEnvelope(eventType, payloadJSON)
			validated, err := validator.ValidateEnvelope(raw)
			if err != nil {
				t.Fatalf("ValidateEnvelope(): %v", err)
			}
			if validated.Header.Type != eventType || validated.Payload == nil {
				t.Fatalf("unexpected validated event: %#v", validated)
			}
		})
	}
}

func TestValidatorRejectsInvalidEnvelopeAndPayload(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatalf("NewValidator(): %v", err)
	}

	tests := []struct {
		name string
		raw  string
		code ValidationCode
	}{
		{
			name: "missing principal",
			raw:  `{"protocol_version":"societas/1","schema_version":"1","id":"evt_01ARZ3NDEKTSV4RRFFQ69G5FAV","type":"workspace.created","ts":"2026-10-23T12:00:00Z","workspace_id":"ws_demo","run_id":"RUN-001","correlation_id":"cor_01ARZ3NDEKTSV4RRFFQ69G5FAV","from":"human:user","to":"system:orchestrator","payload":{"name":"demo"}}`,
			code: CodeInvalidEnvelope,
		},
		{
			name: "unknown event",
			raw:  string(validEnvelope("future.created", []byte(`{"name":"demo"}`))),
			code: CodeUnknownEventType,
		},
		{
			name: "unsupported schema version",
			raw:  strings.Replace(string(validEnvelope(EventWorkspaceCreated, []byte(`{"name":"demo"}`))), `"schema_version":"1"`, `"schema_version":"2"`, 1),
			code: CodeUnsupportedVersion,
		},
		{
			name: "unknown envelope field",
			raw:  strings.Replace(string(validEnvelope(EventWorkspaceCreated, []byte(`{"name":"demo"}`))), `"payload":`, `"future":"secret-envelope-value","payload":`, 1),
			code: CodeInvalidEnvelope,
		},
		{
			name: "unknown payload field",
			raw:  string(validEnvelope(EventWorkspaceCreated, []byte(`{"name":"demo","future_secret":"secret-payload-value"}`))),
			code: CodeInvalidPayload,
		},
		{
			name: "missing required payload field",
			raw:  string(validEnvelope(EventWorkspaceCreated, []byte(`{}`))),
			code: CodeInvalidPayload,
		},
		{
			name: "malformed json",
			raw:  `{"type":`,
			code: CodeInvalidJSON,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := validator.ValidateEnvelope([]byte(test.raw))
			if !IsValidationCode(err, test.code) {
				t.Fatalf("got error %v, want code %s", err, test.code)
			}
			if strings.Contains(test.name, "payload field") && strings.Contains(err.Error(), "secret-payload-value") {
				t.Fatalf("validation error disclosed payload data: %v", err)
			}
		})
	}
}

func TestValidatorRejectsUnknownEventAndPayloadType(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatalf("NewValidator(): %v", err)
	}
	if err := validator.ValidatePayload("future.created", []byte(`{"name":"demo"}`)); !IsValidationCode(err, CodeUnknownEventType) {
		t.Fatalf("ValidatePayload() error = %v, want %s", err, CodeUnknownEventType)
	}
	if err := validator.ValidatePayload(EventWorkspaceCreated, []byte(`{"name":"demo"}`)); err != nil {
		t.Fatalf("ValidatePayload() error = %v", err)
	}
}

func validEnvelope(eventType EventType, payload json.RawMessage) []byte {
	principal := Principal{
		PrincipalID:   "usr_01ARZ3NDEKTSV4RRFFQ69G5FAV",
		PrincipalType: PrincipalHuman,
		SessionID:     "ses_01ARZ3NDEKTSV4RRFFQ69G5FAV",
		AuthDigest:    "hmac_sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	envelope := Envelope{
		ProtocolVersion: ProtocolVersion,
		SchemaVersion:   SchemaVersion,
		ID:              "evt_01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Type:            eventType,
		Timestamp:       "2026-10-23T12:00:00Z",
		WorkspaceID:     "ws_demo",
		RunID:           "RUN-001",
		CorrelationID:   "cor_01ARZ3NDEKTSV4RRFFQ69G5FAV",
		From:            "human:user",
		To:              "system:orchestrator",
		Payload:         payload,
		Principal:       principal,
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		panic(err)
	}
	return raw
}

func artifactCreatedPayload(name, taskID, actor string) map[string]any {
	return map[string]any{"artifact": map[string]any{
		"id":         "art_01ARZ3NDEKTSV4RRFFQ69G5FAV",
		"name":       name,
		"kind":       "markdown",
		"checksum":   "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"size_bytes": 12,
		"version":    1,
		"task_id":    taskID,
		"created_by": actor,
		"created_at": "2026-10-23T12:00:00Z",
	}}
}

func approvalRequestedPayload() map[string]any {
	return map[string]any{
		"approval_id":   "apr_01ARZ3NDEKTSV4RRFFQ69G5FAV",
		"action":        "tool.execute",
		"reason":        "Publish the reviewed draft",
		"trigger":       "policy",
		"risk":          "high",
		"delivery_mode": "immediate",
		"bound_hash":    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"snapshot_ref": map[string]any{
			"artifact_id": "art_01ARZ3NDEKTSV4RRFFQ69G5FAV",
			"version":     1,
			"checksum":    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
	}
}
