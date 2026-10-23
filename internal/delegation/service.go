// Package delegation validates agent requests and routes task creation through the Orchestrator.
package delegation

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

var (
	ErrUnknownAssignee  = errors.New("unknown assignee")
	ErrPermissionDenied = errors.New("delegation permission denied")
	ErrDepthLimit       = errors.New("delegation depth limit exceeded")
	ErrFanOutLimit      = errors.New("delegation fan-out limit exceeded")
	ErrBudgetExceeded   = errors.New("delegation budget exceeds remaining parent budget")
	ErrNoRoutableAgent  = errors.New("no active agent can accept delegation")
)

type Agent struct {
	ID              string
	Active          bool
	ConcurrentTasks int
}
type Registry interface {
	IDs() []string
	Has(string) bool
}
type Policy interface {
	CanDelegate(requester, target string) bool
}
type Orchestrator interface {
	CreateDelegatedTask(context.Context, Decision) error
}

type Budget struct {
	Tokens         int64
	CostUSD        float64
	ModelCalls     int64
	ToolCalls      int64
	ParallelAgents int64
}
type Limits struct {
	MaxDepth          int
	MaxFanOut         int
	MaxChildTasks     int
	MaxParallelAgents int64
}
type Request struct {
	Requester         string
	RequestedAssignee *string
	Title             string
	Goal              string
	Rationale         string
	Priority          string
	Dependencies      []string
	Depth             int
	CurrentFanOut     int
	CurrentChildTasks int
	ParentRemaining   Budget
	RequestedBudget   Budget
}
type Decision struct {
	Request  Request
	Assignee string
	Reason   string
	Blind    bool
}
type Service struct {
	Registry     Registry
	Policy       Policy
	Orchestrator Orchestrator
	Agents       map[string]Agent
	Limits       Limits
}

func (s *Service) Delegate(ctx context.Context, request Request) (Decision, error) {
	if s.Registry == nil || s.Policy == nil || s.Orchestrator == nil {
		return Decision{}, fmt.Errorf("delegation service requires registry, policy, and orchestrator")
	}
	if request.Title == "" || request.Goal == "" || request.Rationale == "" || request.Priority == "" {
		return Decision{}, reject(ErrPermissionDenied, "invalid_delegation_request")
	}
	if s.Limits.MaxDepth < 1 || s.Limits.MaxFanOut < 1 || s.Limits.MaxChildTasks < 1 || s.Limits.MaxParallelAgents < 1 {
		return Decision{}, fmt.Errorf("delegation limits must be positive")
	}
	if request.Requester == "" || !s.Registry.Has(request.Requester) {
		return Decision{}, reject(ErrPermissionDenied, "requester_not_registered")
	}
	if request.Depth < 0 || request.Depth >= s.Limits.MaxDepth {
		return Decision{}, reject(ErrDepthLimit, "depth_limit_exceeded")
	}
	if request.CurrentFanOut >= s.Limits.MaxFanOut {
		return Decision{}, reject(ErrFanOutLimit, "fan_out_limit_exceeded")
	}
	if request.CurrentChildTasks >= s.Limits.MaxChildTasks {
		return Decision{}, reject(ErrFanOutLimit, "child_task_limit_exceeded")
	}
	if exceeds(request.RequestedBudget, request.ParentRemaining) {
		return Decision{}, reject(ErrBudgetExceeded, "budget_exceeded")
	}
	assignee, blind, err := s.selectAssignee(request)
	if err != nil {
		return Decision{}, err
	}
	decision := Decision{Request: request, Assignee: assignee, Blind: blind, Reason: "explicit_assignee"}
	if blind {
		decision.Reason = "blind_route_lowest_load_then_agent_id"
	}
	if err := s.Orchestrator.CreateDelegatedTask(ctx, decision); err != nil {
		return Decision{}, fmt.Errorf("orchestrator rejected delegated task: %w", err)
	}
	return decision, nil
}

func (s *Service) selectAssignee(request Request) (string, bool, error) {
	if request.RequestedAssignee != nil {
		id := *request.RequestedAssignee
		if !s.Registry.Has(id) {
			return "", false, reject(ErrUnknownAssignee, "unknown_assignee")
		}
		agent, ok := s.Agents[id]
		if !ok || !agent.Active {
			return "", false, reject(ErrUnknownAssignee, "assignee_inactive")
		}
		if agent.ConcurrentTasks >= int(s.Limits.MaxParallelAgents) {
			return "", false, reject(ErrNoRoutableAgent, "assignee_at_parallel_limit")
		}
		if !s.Policy.CanDelegate(request.Requester, id) {
			return "", false, reject(ErrPermissionDenied, "permission_denied")
		}
		return id, false, nil
	}
	candidates := make([]Agent, 0)
	for _, id := range s.Registry.IDs() {
		agent, ok := s.Agents[id]
		if !ok || !agent.Active || agent.ConcurrentTasks >= int(s.Limits.MaxParallelAgents) || !s.Policy.CanDelegate(request.Requester, id) {
			continue
		}
		candidates = append(candidates, agent)
	}
	if len(candidates) == 0 {
		return "", true, reject(ErrNoRoutableAgent, "no_routable_agent")
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].ConcurrentTasks != candidates[j].ConcurrentTasks {
			return candidates[i].ConcurrentTasks < candidates[j].ConcurrentTasks
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates[0].ID, true, nil
}

func exceeds(requested, remaining Budget) bool {
	return requested.Tokens < 0 || requested.CostUSD < 0 || requested.ModelCalls < 0 || requested.ToolCalls < 0 || requested.ParallelAgents < 0 || requested.Tokens > remaining.Tokens || requested.CostUSD > remaining.CostUSD || requested.ModelCalls > remaining.ModelCalls || requested.ToolCalls > remaining.ToolCalls || requested.ParallelAgents > remaining.ParallelAgents
}

type Rejection struct {
	Cause error
	Code  string
}

func (e *Rejection) Error() string          { return e.Code }
func (e *Rejection) Unwrap() error          { return e.Cause }
func reject(cause error, code string) error { return &Rejection{Cause: cause, Code: code} }
