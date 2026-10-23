package v1

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema/*.json
var schemaFiles embed.FS

type ValidationCode string

const (
	CodeInvalidJSON        ValidationCode = "INVALID_JSON"
	CodeInvalidEnvelope    ValidationCode = "INVALID_ENVELOPE"
	CodeUnsupportedVersion ValidationCode = "UNSUPPORTED_VERSION"
	CodeUnknownEventType   ValidationCode = "UNKNOWN_EVENT_TYPE"
	CodeInvalidPayload     ValidationCode = "INVALID_PAYLOAD"
)

// ContractError carries a stable code and deliberately excludes input values
// and validator diagnostics, which may contain secrets from the payload.
type ContractError struct {
	Code ValidationCode
}

func (e *ContractError) Error() string {
	return fmt.Sprintf("contract validation failed: %s", e.Code)
}

func (e *ContractError) Is(target error) bool {
	other, ok := target.(*ContractError)
	return ok && e.Code == other.Code
}

// ValidatedEvent is returned only after both the envelope and its event
// payload have passed their canonical Section 72A schemas.
type ValidatedEvent struct {
	Header  Envelope
	Payload any
}

// Validator routes v1 envelopes to their registered payload schemas.
type Validator struct {
	envelope *jsonschema.Schema
	payloads map[EventType]*jsonschema.Schema
}

// NewValidator loads only embedded local schemas; validation does not fetch
// schemas or other resources from the network.
func NewValidator() (*Validator, error) {
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	entries, err := schemaFiles.ReadDir("schema")
	if err != nil {
		return nil, fmt.Errorf("read embedded contract schemas: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".json" {
			continue
		}
		contents, err := schemaFiles.ReadFile(path.Join("schema", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read embedded contract schema %s: %w", entry.Name(), err)
		}
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(contents))
		if err != nil {
			return nil, fmt.Errorf("parse embedded contract schema %s: %w", entry.Name(), err)
		}
		metadata, ok := document.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("embedded contract schema %s is not an object", entry.Name())
		}
		id, ok := metadata["$id"].(string)
		if !ok || !strings.HasPrefix(id, "urn:societas:1:") {
			return nil, fmt.Errorf("embedded contract schema %s has no canonical ID", entry.Name())
		}
		if err := compiler.AddResource(id, document); err != nil {
			return nil, fmt.Errorf("register embedded contract schema %s: %w", entry.Name(), err)
		}
	}

	envelopeSchema, err := compiler.Compile("urn:societas:1:envelope")
	if err != nil {
		return nil, fmt.Errorf("compile envelope schema: %w", err)
	}
	payloads := make(map[EventType]*jsonschema.Schema, len(eventDefinitions))
	for _, definition := range eventDefinitions {
		schema, err := compiler.Compile(definition.PayloadSchemaID)
		if err != nil {
			return nil, fmt.Errorf("compile payload schema for %s: %w", definition.Type, err)
		}
		if _, exists := payloads[definition.Type]; exists {
			return nil, fmt.Errorf("duplicate event type in registry: %s", definition.Type)
		}
		payloads[definition.Type] = schema
	}
	return &Validator{envelope: envelopeSchema, payloads: payloads}, nil
}

// ValidateEnvelope strictly decodes and validates a complete wire envelope,
// routes by protocol/schema version and event type, then decodes its typed
// payload. Call this before publishing or persisting an event.
func (v *Validator) ValidateEnvelope(raw []byte) (*ValidatedEvent, error) {
	var envelope Envelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return nil, validationError(CodeInvalidEnvelope)
		}
		return nil, validationError(CodeInvalidJSON)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, validationError(CodeInvalidJSON)
	}

	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, validationError(CodeInvalidJSON)
	}
	if err := v.envelope.Validate(instance); err != nil {
		return nil, validationError(CodeInvalidEnvelope)
	}
	if envelope.ProtocolVersion != ProtocolVersion || envelope.SchemaVersion != SchemaVersion {
		return nil, validationError(CodeUnsupportedVersion)
	}
	if _, ok := v.payloads[envelope.Type]; !ok {
		return nil, validationError(CodeUnknownEventType)
	}
	if err := v.validatePayload(envelope.Type, envelope.Payload); err != nil {
		return nil, err
	}
	payload, err := envelope.DecodePayload()
	if err != nil {
		return nil, validationError(CodeInvalidPayload)
	}
	return &ValidatedEvent{Header: envelope, Payload: payload}, nil
}

// ValidatePayload validates one active event payload without an envelope.
func (v *Validator) ValidatePayload(eventType EventType, raw []byte) error {
	if _, ok := v.payloads[eventType]; !ok {
		return validationError(CodeUnknownEventType)
	}
	return v.validatePayload(eventType, raw)
}

func (v *Validator) validatePayload(eventType EventType, raw []byte) error {
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return validationError(CodeInvalidJSON)
	}
	if err := v.payloads[eventType].Validate(instance); err != nil {
		return validationError(CodeInvalidPayload)
	}
	return nil
}

func validationError(code ValidationCode) error {
	return &ContractError{Code: code}
}

// IsValidationCode reports whether err is a contract validation failure with
// the supplied stable code.
func IsValidationCode(err error, code ValidationCode) bool {
	var contractErr *ContractError
	return errors.As(err, &contractErr) && contractErr.Code == code
}
