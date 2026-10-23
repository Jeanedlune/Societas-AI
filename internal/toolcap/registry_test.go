package toolcap

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Jeanedlune/Societas-AI/internal/config"
)

func testRegistry(t *testing.T, timeoutMS int64) *Registry {
	t.Helper()
	cfg := &config.Config{ToolCapabilities: map[string]config.ToolCapabilityConfig{
		"browser.search": {
			InputSchema:  map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}, "required": []any{"query"}, "additionalProperties": false},
			OutputSchema: map[string]any{"type": "object", "properties": map[string]any{"count": map[string]any{"type": "integer"}}, "required": []any{"count"}, "additionalProperties": false},
			TimeoutMS:    timeoutMS, ResourceScope: []string{"workspace"}, RequiredPermissions: []string{"browser.search"},
			OperationKeyEnforced: true,
		},
		"artifact.read": {
			InputSchema: map[string]any{"type": "object"}, OutputSchema: map[string]any{"type": "object"}, Idempotent: true,
		},
	}}
	registry, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func TestRegistryValidatesInputOutputAndDefaultsToDeny(t *testing.T) {
	registry := testRegistry(t, 100)
	if err := registry.ValidateInput("browser.search", []byte(`{"query":"news"}`)); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	if err := registry.ValidateInput("browser.search", []byte(`{"query":42}`)); !errors.Is(err, ErrSchemaValidation) {
		t.Fatalf("invalid input error = %v", err)
	}
	if err := registry.ValidateOutput("browser.search", []byte(`{"count":2}`)); err != nil {
		t.Fatalf("valid output rejected: %v", err)
	}
	if err := registry.ValidateOutput("browser.search", []byte(`{"count":"two"}`)); !errors.Is(err, ErrSchemaValidation) {
		t.Fatalf("invalid output error = %v", err)
	}
	if err := registry.ValidateInput("unknown", []byte(`{}`)); !errors.Is(err, ErrUnknownCapability) {
		t.Fatalf("unknown capability error = %v", err)
	}
	if _, err := registry.Invoke(context.Background(), "browser.search", []byte(`{"query":"news"}`), nil, func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"count":0}`), nil
	}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("missing permission error = %v", err)
	}
}

func TestInvokeEnforcesOutputSchemaPermissionAndTimeout(t *testing.T) {
	registry := testRegistry(t, 5)
	result, err := registry.Invoke(context.Background(), "browser.search", []byte(`{"query":"news"}`), []string{"browser.search"}, func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"count":1}`), nil
	})
	if err != nil || string(result) != `{"count":1}` {
		t.Fatalf("valid invoke result=%s err=%v", result, err)
	}
	if _, err := registry.Invoke(context.Background(), "browser.search", []byte(`{"query":"news"}`), []string{"browser.search"}, func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"count":"bad"}`), nil
	}); !errors.Is(err, ErrSchemaValidation) {
		t.Fatalf("invalid output error = %v", err)
	}
	if _, err := registry.Invoke(context.Background(), "browser.search", []byte(`{"query":"news"}`), []string{"browser.search"}, func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}); !errors.Is(err, ErrToolTimeout) {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestRetrySafetyUsesOnlyCapabilityAndProviderGuarantees(t *testing.T) {
	registry := testRegistry(t, 100)
	if !registry.RetrySafe("artifact.read", false, false) {
		t.Fatal("idempotent operation should be retry safe")
	}
	if registry.RetrySafe("browser.search", true, false) || !registry.RetrySafe("browser.search", true, true) {
		t.Fatal("operation-key retry safety ignored provider guarantee")
	}
	if registry.RetrySafe("unknown", true, true) {
		t.Fatal("unknown capability must not be retry safe")
	}
	capability, ok := registry.Get("browser.search")
	if !ok || capability.Timeout != 100*time.Millisecond || capability.ResourceScope[0] != "workspace" {
		t.Fatalf("capability metadata = %#v, found=%v", capability, ok)
	}
	capability.ResourceScope[0] = "mutated"
	if registry.IDs()[0] == "mutated" {
		t.Fatal("registry exposed mutable state")
	}
}

func TestNewRejectsMissingAndRemoteSchemas(t *testing.T) {
	_, err := New(&config.Config{ToolCapabilities: map[string]config.ToolCapabilityConfig{"x": {InputSchema: map[string]any{"type": "object"}}}})
	if err == nil {
		t.Fatal("missing output schema accepted")
	}
	_, err = New(&config.Config{ToolCapabilities: map[string]config.ToolCapabilityConfig{"x": {
		InputSchema: map[string]any{"$ref": "https://example.com/schema.json"}, OutputSchema: map[string]any{"type": "object"},
	}}})
	if err == nil {
		t.Fatal("remote reference accepted")
	}
}
