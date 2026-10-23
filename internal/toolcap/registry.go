// Package toolcap provides a trusted, default-deny tool capability registry.
package toolcap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/Jeanedlune/Societas-AI/internal/config"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

var (
	ErrUnknownCapability = &ToolError{Code: "UNKNOWN_CAPABILITY"}
	ErrSchemaValidation  = &ToolError{Code: "SCHEMA_VALIDATION_FAILED"}
	ErrToolTimeout       = &ToolError{Code: "TOOL_TIMEOUT"}
	ErrPermissionDenied  = &ToolError{Code: "PERMISSION_DENIED"}
)

type ToolError struct{ Code string }

func (e *ToolError) Error() string { return e.Code }
func (e *ToolError) Is(target error) bool {
	other, ok := target.(*ToolError)
	return ok && e.Code == other.Code
}

type Capability struct {
	ID                   string
	Idempotent           bool
	OperationKeyEnforced bool
	Timeout              time.Duration
	ResourceScope        []string
	RequiredPermissions  []string
}
type compiledCapability struct {
	public        Capability
	input, output *jsonschema.Schema
}
type Registry struct {
	capabilities map[string]compiledCapability
	ids          []string
}
type Handler func(context.Context, json.RawMessage) (json.RawMessage, error)

func New(cfg *config.Config) (*Registry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("tool capability registry requires workspace config")
	}
	registry := &Registry{capabilities: map[string]compiledCapability{}}
	compiler := jsonschema.NewCompiler()
	for _, id := range sortedKeys(cfg.ToolCapabilities) {
		definition := cfg.ToolCapabilities[id]
		if definition.InputSchema == nil || definition.OutputSchema == nil {
			return nil, fmt.Errorf("tool capability %s requires input_schema and output_schema", id)
		}
		if err := rejectRemoteRefs(definition.InputSchema); err != nil {
			return nil, fmt.Errorf("tool capability %s input schema: %w", id, err)
		}
		if err := rejectRemoteRefs(definition.OutputSchema); err != nil {
			return nil, fmt.Errorf("tool capability %s output schema: %w", id, err)
		}
		inputID := "urn:societas:toolcap:" + id + ":input"
		outputID := "urn:societas:toolcap:" + id + ":output"
		if err := compiler.AddResource(inputID, definition.InputSchema); err != nil {
			return nil, fmt.Errorf("register input schema for %s: %w", id, err)
		}
		if err := compiler.AddResource(outputID, definition.OutputSchema); err != nil {
			return nil, fmt.Errorf("register output schema for %s: %w", id, err)
		}
		input, err := compiler.Compile(inputID)
		if err != nil {
			return nil, fmt.Errorf("compile input schema for %s: %w", id, err)
		}
		output, err := compiler.Compile(outputID)
		if err != nil {
			return nil, fmt.Errorf("compile output schema for %s: %w", id, err)
		}
		timeout := time.Duration(definition.TimeoutMS) * time.Millisecond
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		public := Capability{ID: id, Idempotent: definition.Idempotent, OperationKeyEnforced: definition.OperationKeyEnforced, Timeout: timeout, ResourceScope: append([]string(nil), definition.ResourceScope...), RequiredPermissions: append([]string(nil), definition.RequiredPermissions...)}
		registry.capabilities[id] = compiledCapability{public: public, input: input, output: output}
		registry.ids = append(registry.ids, id)
	}
	return registry, nil
}

func (r *Registry) Get(id string) (Capability, bool) {
	if r == nil {
		return Capability{}, false
	}
	item, ok := r.capabilities[id]
	if !ok {
		return Capability{}, false
	}
	public := item.public
	public.ResourceScope = append([]string(nil), public.ResourceScope...)
	public.RequiredPermissions = append([]string(nil), public.RequiredPermissions...)
	return public, true
}
func (r *Registry) IDs() []string {
	if r == nil {
		return nil
	}
	return append([]string(nil), r.ids...)
}
func (r *Registry) ValidateInput(id string, input []byte) error {
	item, ok := r.capabilities[id]
	if !ok {
		return ErrUnknownCapability
	}
	return validateJSON(item.input, input)
}
func (r *Registry) ValidateOutput(id string, output []byte) error {
	item, ok := r.capabilities[id]
	if !ok {
		return ErrUnknownCapability
	}
	return validateJSON(item.output, output)
}

func (r *Registry) Invoke(ctx context.Context, id string, input []byte, allowedPermissions []string, handler Handler) (json.RawMessage, error) {
	item, ok := r.capabilities[id]
	if !ok {
		return nil, ErrUnknownCapability
	}
	if err := validateJSON(item.input, input); err != nil {
		return nil, err
	}
	for _, required := range item.public.RequiredPermissions {
		if !contains(allowedPermissions, required) {
			return nil, ErrPermissionDenied
		}
	}
	if handler == nil {
		return nil, fmt.Errorf("tool handler is required")
	}
	callCtx, cancel := context.WithTimeout(ctx, item.public.Timeout)
	defer cancel()
	type response struct {
		result json.RawMessage
		err    error
	}
	done := make(chan response, 1)
	go func() {
		result, err := handler(callCtx, append(json.RawMessage(nil), input...))
		done <- response{result: result, err: err}
	}()
	var result json.RawMessage
	select {
	case <-callCtx.Done():
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			return nil, ErrToolTimeout
		}
		return nil, callCtx.Err()
	case response := <-done:
		if response.err != nil {
			return nil, response.err
		}
		result = response.result
	}
	if err := validateJSON(item.output, result); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), result...), nil
}

// RetrySafe reflects provider-guaranteed behavior, not a model/agent assertion.
func (r *Registry) RetrySafe(id string, operationKeyPresent, providerGuaranteesDeduplication bool) bool {
	item, ok := r.capabilities[id]
	if !ok {
		return false
	}
	return item.public.Idempotent || (item.public.OperationKeyEnforced && operationKeyPresent && providerGuaranteesDeduplication)
}

func validateJSON(schema *jsonschema.Schema, raw []byte) error {
	if len(raw) == 0 {
		return ErrSchemaValidation
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return ErrSchemaValidation
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var ignored any
	if err := decoder.Decode(&ignored); err != nil {
		return ErrSchemaValidation
	}
	if err := decoder.Decode(&ignored); err != io.EOF {
		return ErrSchemaValidation
	}
	if err := schema.Validate(document); err != nil {
		return ErrSchemaValidation
	}
	return nil
}
func rejectRemoteRefs(schema map[string]any) error {
	var visit func(any) error
	visit = func(value any) error {
		switch node := value.(type) {
		case map[string]any:
			for key, child := range node {
				if key == "$ref" {
					ref, ok := child.(string)
					if !ok || !strings.HasPrefix(ref, "#") {
						return fmt.Errorf("remote schema references are disabled")
					}
				}
				if err := visit(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range node {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(schema)
}
func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
