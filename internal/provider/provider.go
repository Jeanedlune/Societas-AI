// Package provider defines the vendor-neutral model provider boundary.
package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Jeanedlune/Societas-AI/internal/config"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Request struct {
	Model           string
	Messages        []Message
	MaxOutputTokens int64
	Temperature     float64
	Tools           []ToolSchema
	JSONMode        bool
}
type ToolSchema struct {
	Name   string
	Schema []byte
}
type Usage struct {
	InputTokens       int64
	OutputTokens      int64
	CachedInputTokens int64
	CostUSD           float64
}
type FinishReason string

const (
	FinishStop      FinishReason = "stop"
	FinishLength    FinishReason = "length"
	FinishToolCalls FinishReason = "tool_calls"
	FinishError     FinishReason = "error"
)

type Response struct {
	Content           string
	FinishReason      FinishReason
	Usage             Usage
	ProviderRequestID string
}
type Chunk struct {
	Text         string
	Done         bool
	Usage        *Usage
	FinishReason FinishReason
	Err          error
}
type Capabilities struct {
	Streaming bool
	ToolCalls bool
	JSONMode  bool
}

type Provider interface {
	ID() string
	Capabilities() Capabilities
	Generate(context.Context, Request) (Response, error)
	Stream(context.Context, Request) (<-chan Chunk, error)
}

type Error struct {
	Code       string
	Category   string
	Retryable  bool
	RetryAfter time.Duration
	Message    string
}

func (e *Error) Error() string { return e.Code }

func MapError(err error) *Error {
	if err == nil {
		return nil
	}
	var mapped *Error
	if errors.As(err, &mapped) {
		code, category, retryable := mapped.Code, mapped.Category, mapped.Retryable
		switch code {
		case "MODEL_TIMEOUT":
			category = "TIMEOUT"
		case "MODEL_UNAVAILABLE":
			category = "MODEL_ERROR"
		case "RATE_LIMITED":
			category = "LIMIT_ERROR"
		case "NETWORK_ERROR":
			category = "NETWORK_ERROR"
		case "INVALID_OUTPUT":
			category = "INVALID_OUTPUT"
		default:
			code, category, retryable = "MODEL_UNAVAILABLE", "MODEL_ERROR", true
		}
		return &Error{Code: code, Category: category, Retryable: retryable, RetryAfter: mapped.RetryAfter, Message: safeMessage(code)}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Code: "MODEL_TIMEOUT", Category: "TIMEOUT", Retryable: true, Message: "provider request timed out"}
	}
	if errors.Is(err, context.Canceled) {
		return &Error{Code: "MODEL_TIMEOUT", Category: "TIMEOUT", Retryable: true, Message: "provider request was cancelled"}
	}
	return &Error{Code: "MODEL_UNAVAILABLE", Category: "MODEL_ERROR", Retryable: true, Message: "provider request failed"}
}

func safeMessage(code string) string {
	switch code {
	case "MODEL_TIMEOUT":
		return "provider request timed out"
	case "RATE_LIMITED":
		return "provider rate limit reached"
	case "NETWORK_ERROR":
		return "provider network request failed"
	case "INVALID_OUTPUT":
		return "provider response did not match the requested format"
	default:
		return "provider request failed"
	}
}

type Registry struct{ providers map[string]Provider }

func NewRegistry() *Registry { return &Registry{providers: make(map[string]Provider)} }
func (r *Registry) Register(p Provider) error {
	if p == nil || strings.TrimSpace(p.ID()) == "" {
		return fmt.Errorf("provider ID is required")
	}
	if _, exists := r.providers[p.ID()]; exists {
		return fmt.Errorf("provider %q is already registered", p.ID())
	}
	r.providers[p.ID()] = p
	return nil
}
func (r *Registry) Get(id string) (Provider, bool) { p, ok := r.providers[id]; return p, ok }
func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ResolveCredential returns an env-resolved secret only to the trusted adapter.
func ResolveCredential(cfg *config.Config, providerID, key string) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("provider config is required")
	}
	return cfg.Credential(providerID, key, os.LookupEnv)
}

// Mock is a deterministic adapter for tests and local end-to-end workflows.
type Mock struct {
	Name         string
	Response     Response
	GenerateFunc func(context.Context, Request) (Response, error)
	Chunks       []Chunk
	Err          error
}

func (m *Mock) ID() string { return m.Name }
func (m *Mock) Capabilities() Capabilities {
	return Capabilities{Streaming: true, ToolCalls: true, JSONMode: true}
}
func (m *Mock) Generate(ctx context.Context, request Request) (Response, error) {
	if m.Err != nil {
		return Response{}, MapError(m.Err)
	}
	if m.GenerateFunc != nil {
		response, err := m.GenerateFunc(ctx, request)
		if err != nil {
			return Response{}, MapError(err)
		}
		return response, nil
	}
	if err := ctx.Err(); err != nil {
		return Response{}, MapError(err)
	}
	return m.Response, nil
}
func (m *Mock) Stream(ctx context.Context, request Request) (<-chan Chunk, error) {
	if !m.Capabilities().Streaming {
		return nil, &Error{Code: "MODEL_UNAVAILABLE", Category: "MODEL_ERROR", Retryable: true, Message: "streaming is not supported"}
	}
	if err := ctx.Err(); err != nil {
		return nil, MapError(err)
	}
	ch := make(chan Chunk, len(m.Chunks))
	for _, chunk := range m.Chunks {
		ch <- chunk
	}
	close(ch)
	return ch, nil
}
