package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRegistryAndMockProvider(t *testing.T) {
	registry := NewRegistry()
	mock := &Mock{Name: "mock", Response: Response{Content: "deterministic", FinishReason: FinishStop, Usage: Usage{InputTokens: 3, OutputTokens: 2}}}
	if err := registry.Register(mock); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(mock); err == nil {
		t.Fatal("duplicate provider registered")
	}
	got, ok := registry.Get("mock")
	if !ok {
		t.Fatal("registered mock missing")
	}
	response, err := got.Generate(context.Background(), Request{Model: "mock/model"})
	if err != nil || response.Content != "deterministic" || response.Usage.OutputTokens != 2 {
		t.Fatalf("response=%#v err=%v", response, err)
	}
	if !got.Capabilities().Streaming || !got.Capabilities().JSONMode {
		t.Fatalf("capabilities=%#v", got.Capabilities())
	}
}

func TestProviderErrorsUseStableSafeMapping(t *testing.T) {
	err := MapError(&Error{Code: "MODEL_TIMEOUT", Category: "TIMEOUT", Retryable: true, Message: "provider returned secret-key-value"})
	if err.Code != "MODEL_TIMEOUT" || err.Category != "TIMEOUT" || !err.Retryable {
		t.Fatalf("mapped error=%#v", err)
	}
	if strings.Contains(err.Error(), "secret-key-value") || strings.Contains(err.Message, "secret-key-value") {
		t.Fatalf("provider secret leaked: %#v", err)
	}
	unknown := MapError(errors.New("authorization: secret-key-value"))
	if unknown.Code != "MODEL_UNAVAILABLE" || strings.Contains(unknown.Error(), "secret-key-value") {
		t.Fatalf("unknown error mapping=%#v", unknown)
	}
}

func TestMockProviderMapsContextDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mock := &Mock{Name: "mock"}
	_, err := mock.Generate(ctx, Request{})
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != "MODEL_TIMEOUT" {
		t.Fatalf("Generate error=%v", err)
	}
}
