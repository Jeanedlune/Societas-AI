// Package modelrouter selects only configured models/providers under risk and rate policy.
package modelrouter

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Jeanedlune/Societas-AI/internal/agent"
	"github.com/Jeanedlune/Societas-AI/internal/provider"
)

type Tier string

const (
	Cheap  Tier = "cheap"
	Medium Tier = "medium"
	Strong Tier = "strong"
)

type Model struct {
	ID         string
	ProviderID string
	Tier       Tier
	Enabled    bool
}
type Limits struct {
	RequestsPerMinute int
	TokensPerMinute   int64
}
type Request struct {
	AgentID         string
	Complexity      string
	Risk            string
	EstimatedTokens int64
}
type Route struct {
	AgentID    string
	ModelID    string
	ProviderID string
	Tier       Tier
	Reason     string
	Fallback   bool
}

var (
	ErrAgentInactive    = errors.New("agent is not active in workspace config")
	ErrModelUnavailable = errors.New("no configured model/provider is available")
	ErrModelDisabled    = errors.New("selected model is disabled")
	ErrRateLimited      = errors.New("provider rate limit reached")
)

type Router struct {
	Agents      *agent.Registry
	Providers   *provider.Registry
	Models      map[string]Model
	AgentModels map[string]string
	TierModels  map[Tier]string
	Fallbacks   map[string][]string
	Limiter     *RateLimiter
}

func (r *Router) Select(request Request, now time.Time) (Route, error) {
	if r.Agents == nil || r.Providers == nil {
		return Route{}, fmt.Errorf("router requires active agent and provider registries")
	}
	agentConfig, ok := r.Agents.Get(request.AgentID)
	if !ok {
		return Route{}, ErrAgentInactive
	}
	minimum := minimumTier(request.Risk)
	desired := complexityTier(request.Complexity)
	if tierRank(desired) < tierRank(minimum) {
		desired = minimum
	}
	preferred := r.AgentModels[request.AgentID]
	if preferred == "" {
		preferred = agentConfig.Model
	}
	primary := preferred
	if model, exists := r.Models[preferred]; !exists || !model.Enabled || tierRank(model.Tier) < tierRank(desired) {
		primary = r.TierModels[desired]
	}
	if primary == "" {
		return Route{}, ErrModelUnavailable
	}
	candidates := append([]string{primary}, r.Fallbacks[primary]...)
	if primary != preferred {
		candidates = appendUnique(candidates, r.Fallbacks[preferred]...)
	}
	seen := map[string]bool{}
	var rateWait time.Duration
	for index, id := range candidates {
		if seen[id] {
			continue
		}
		seen[id] = true
		model, exists := r.Models[id]
		if !exists || !model.Enabled || tierRank(model.Tier) < tierRank(minimum) {
			continue
		}
		if _, ok := r.Providers.Get(model.ProviderID); !ok {
			continue
		}
		if r.Limiter != nil {
			allowed, retry := r.Limiter.Allow(model.ProviderID, request.EstimatedTokens, now)
			if !allowed {
				if rateWait == 0 || retry < rateWait {
					rateWait = retry
				}
				continue
			}
		}
		reason := fmt.Sprintf("tier=%s complexity=%s risk=%s", model.Tier, request.Complexity, request.Risk)
		return Route{AgentID: request.AgentID, ModelID: model.ID, ProviderID: model.ProviderID, Tier: model.Tier, Reason: reason, Fallback: index > 0 || model.ID != preferred}, nil
	}
	if rateWait > 0 {
		return Route{}, &RateLimitError{RetryAfter: rateWait}
	}
	if model, ok := r.Models[primary]; ok && !model.Enabled {
		return Route{}, ErrModelDisabled
	}
	return Route{}, ErrModelUnavailable
}

type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string        { return ErrRateLimited.Error() }
func (e *RateLimitError) Unwrap() error        { return ErrRateLimited }
func (e *RateLimitError) DeliveryCode() string { return "RATE_LIMITED" }

type requestStamp struct {
	at     time.Time
	tokens int64
}
type RateLimiter struct {
	mu       sync.Mutex
	limits   map[string]Limits
	requests map[string][]requestStamp
}

func NewRateLimiter(limits map[string]Limits) *RateLimiter {
	return &RateLimiter{limits: limits, requests: make(map[string][]requestStamp)}
}

// Allow implements a per-provider sliding-minute RPM/TPM limiter. A rejected
// request receives bounded retry metadata and is never queued in an unbounded loop.
func (l *RateLimiter) Allow(providerID string, tokens int64, now time.Time) (bool, time.Duration) {
	if l == nil {
		return true, 0
	}
	if tokens < 0 {
		return false, time.Minute
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	limit, ok := l.limits[providerID]
	if !ok || limit.RequestsPerMinute < 1 || limit.TokensPerMinute < 1 {
		return false, time.Minute
	}
	cutoff := now.Add(-time.Minute)
	active := l.requests[providerID][:0]
	var total int64
	for _, stamp := range l.requests[providerID] {
		if !stamp.at.Before(cutoff) {
			active = append(active, stamp)
			total += stamp.tokens
		}
	}
	l.requests[providerID] = active
	if tokens > limit.TokensPerMinute {
		return false, time.Minute
	}
	if len(active) >= limit.RequestsPerMinute || total+tokens > limit.TokensPerMinute {
		if len(active) == 0 {
			return false, time.Minute
		}
		retry := active[0].at.Add(time.Minute).Sub(now)
		if retry < 0 {
			retry = 0
		}
		return false, retry
	}
	l.requests[providerID] = append(active, requestStamp{at: now, tokens: tokens})
	return true, 0
}

func minimumTier(risk string) Tier {
	if strings.EqualFold(risk, "critical") {
		return Strong
	}
	if strings.EqualFold(risk, "high") {
		return Medium
	}
	return Cheap
}
func complexityTier(complexity string) Tier {
	switch strings.ToLower(complexity) {
	case "high":
		return Strong
	case "medium":
		return Medium
	default:
		return Cheap
	}
}
func tierRank(t Tier) int {
	switch t {
	case Cheap:
		return 1
	case Medium:
		return 2
	case Strong:
		return 3
	default:
		return 0
	}
}
func appendUnique(values []string, more ...string) []string {
	seen := map[string]bool{}
	for _, v := range values {
		seen[v] = true
	}
	for _, v := range more {
		if !seen[v] {
			values = append(values, v)
			seen[v] = true
		}
	}
	return values
}
func SortedModelIDs(models map[string]Model) []string {
	ids := make([]string, 0, len(models))
	for id := range models {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
