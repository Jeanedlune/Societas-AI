// Package budget provides finite budget defaults and concurrency-safe child allocation.
package budget

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/Jeanedlune/Societas-AI/internal/config"
)

var (
	ErrBudgetExceeded     = errors.New("BUDGET_EXCEEDED")
	ErrAllocationConflict = errors.New("budget allocation key conflicts with an existing request")
	ErrUnknownAccount     = errors.New("budget account was not found")
)

type Budget struct {
	InputTokens    int64   `json:"max_input_tokens"`
	OutputTokens   int64   `json:"max_output_tokens"`
	TotalTokens    int64   `json:"max_total_tokens"`
	ModelCalls     int64   `json:"max_model_calls"`
	ToolCalls      int64   `json:"max_tool_calls"`
	AgentMessages  int64   `json:"max_agent_messages"`
	TimeSeconds    int64   `json:"max_time_seconds"`
	CostUSD        float64 `json:"max_cost_usd"`
	ParallelAgents int64   `json:"max_parallel_agents"`
	Rebuttals      int64   `json:"max_rebuttals"`
}

type Scope string

const (
	ScopeWorkspace Scope = "workspace"
	ScopeRun       Scope = "run"
	ScopeTask      Scope = "task"
	ScopeAgent     Scope = "agent"
)

type Usage struct {
	InputTokens    int64
	OutputTokens   int64
	TotalTokens    int64
	ModelCalls     int64
	ToolCalls      int64
	AgentMessages  int64
	TimeSeconds    int64
	CostUSD        float64
	ParallelAgents int64
	Rebuttals      int64
}
type AccountSnapshot struct {
	Granted  Budget
	Used     Usage
	Reserved Budget
}
type allocation struct {
	budget  Budget
	settled bool
}
type account struct {
	granted  Budget
	used     Usage
	reserved Budget
	children map[string]allocation
}
type Manager struct {
	mu       sync.Mutex
	defaults Budget
	accounts map[string]*account
}

func FromConfig(b config.BudgetConfig) Budget {
	return Budget{InputTokens: b.MaxInputTokens, OutputTokens: b.MaxOutputTokens, TotalTokens: b.MaxTotalTokens, ModelCalls: b.MaxModelCalls, ToolCalls: b.MaxToolCalls, AgentMessages: b.MaxAgentMessages, TimeSeconds: b.MaxTimeSeconds, CostUSD: b.MaxCostUSD, ParallelAgents: b.MaxParallelAgents, Rebuttals: b.MaxRebuttals}
}

// DefaultsForScope applies Doc 10's finite scope matrix, then lets explicit
// nonzero workspace config values override the fallback limits. Token totals
// are split 80/20 between input and output where the contract gives only a
// combined token ceiling.
func DefaultsForScope(scope Scope, configured config.BudgetConfig) (Budget, error) {
	var fallback Budget
	switch scope {
	case ScopeWorkspace:
		fallback = Budget{InputTokens: 4_000_000, OutputTokens: 1_000_000, TotalTokens: 5_000_000, ModelCalls: 100_000, ToolCalls: 100_000, AgentMessages: 100_000, TimeSeconds: 86_400, CostUSD: 50, ParallelAgents: 8, Rebuttals: 2}
	case ScopeRun:
		fallback = Budget{InputTokens: 400_000, OutputTokens: 100_000, TotalTokens: 500_000, ModelCalls: 10_000, ToolCalls: 10_000, AgentMessages: 10_000, TimeSeconds: 14_400, CostUSD: 5, ParallelAgents: 8, Rebuttals: 2}
	case ScopeTask:
		fallback = Budget{InputTokens: 40_000, OutputTokens: 10_000, TotalTokens: 50_000, ModelCalls: 20, ToolCalls: 30, AgentMessages: 100, TimeSeconds: 300, CostUSD: .5, ParallelAgents: 4, Rebuttals: 2}
	case ScopeAgent:
		fallback = Budget{InputTokens: 16_000, OutputTokens: 4_000, TotalTokens: 20_000, ModelCalls: 8, ToolCalls: 12, AgentMessages: 40, TimeSeconds: 120, CostUSD: .2, ParallelAgents: 1, Rebuttals: 2}
	default:
		return Budget{}, fmt.Errorf("unknown budget scope %q", scope)
	}
	configuredBudget := FromConfig(configured)
	if configuredBudget.InputTokens > 0 {
		fallback.InputTokens = configuredBudget.InputTokens
	}
	if configuredBudget.OutputTokens > 0 {
		fallback.OutputTokens = configuredBudget.OutputTokens
	}
	if configuredBudget.TotalTokens > 0 {
		fallback.TotalTokens = configuredBudget.TotalTokens
	}
	if configuredBudget.ModelCalls > 0 {
		fallback.ModelCalls = configuredBudget.ModelCalls
	}
	if configuredBudget.ToolCalls > 0 {
		fallback.ToolCalls = configuredBudget.ToolCalls
	}
	if configuredBudget.AgentMessages > 0 {
		fallback.AgentMessages = configuredBudget.AgentMessages
	}
	if configuredBudget.TimeSeconds > 0 {
		fallback.TimeSeconds = configuredBudget.TimeSeconds
	}
	if configuredBudget.CostUSD > 0 {
		fallback.CostUSD = configuredBudget.CostUSD
	}
	if configuredBudget.ParallelAgents > 0 {
		fallback.ParallelAgents = configuredBudget.ParallelAgents
	}
	if configuredBudget.Rebuttals > 0 {
		fallback.Rebuttals = configuredBudget.Rebuttals
	}
	if err := validate(fallback); err != nil {
		return Budget{}, err
	}
	return fallback, nil
}
func NewManager(defaults Budget) *Manager {
	if isZero(defaults) {
		defaults = FromConfig(config.DefaultBudget())
	}
	return &Manager{defaults: defaults, accounts: make(map[string]*account)}
}
func NewManagerFromConfig(cfg config.BudgetConfig) *Manager {
	defaults, err := DefaultsForScope(ScopeTask, cfg)
	if err != nil {
		defaults = FromConfig(config.DefaultBudget())
	}
	return NewManager(defaults)
}

func (m *Manager) OpenRoot(id string, limit Budget) error {
	if id == "" {
		return fmt.Errorf("budget account ID is required")
	}
	limit = m.normalize(limit)
	if err := validate(limit); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.accounts[id]; exists {
		return fmt.Errorf("budget account %q already exists", id)
	}
	m.accounts[id] = &account{granted: limit, children: make(map[string]allocation)}
	return nil
}

// Allocate reserves one finite child budget. The mutex serializes sibling
// allocations, preventing two concurrent requests from using the same remainder.
func (m *Manager) Allocate(parentID, childID string, requested Budget) (Budget, bool, error) {
	if parentID == "" || childID == "" || parentID == childID {
		return Budget{}, false, fmt.Errorf("distinct parent and child IDs are required")
	}
	requested = m.normalize(requested)
	if err := validate(requested); err != nil {
		return Budget{}, false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	parent := m.accounts[parentID]
	if parent == nil {
		return Budget{}, false, ErrUnknownAccount
	}
	if existing, ok := parent.children[childID]; ok {
		if existing.budget != requested {
			return Budget{}, false, ErrAllocationConflict
		}
		return existing.budget, true, nil
	}
	if _, exists := m.accounts[childID]; exists {
		return Budget{}, false, ErrAllocationConflict
	}
	remaining := remaining(parent.granted, parent.used, parent.reserved)
	if exceeds(requested, remaining) {
		return Budget{}, false, ErrBudgetExceeded
	}
	parent.children[childID] = allocation{budget: requested}
	parent.reserved = add(parent.reserved, requested)
	m.accounts[childID] = &account{granted: requested, children: make(map[string]allocation)}
	return requested, false, nil
}

// Settle returns unused reservation and charges actual usage exactly once.
func (m *Manager) Settle(parentID, childID string, actual Usage) error {
	if err := validateUsage(actual); err != nil {
		return err
	}
	if actual.TotalTokens == 0 {
		actual.TotalTokens = actual.InputTokens + actual.OutputTokens
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	parent := m.accounts[parentID]
	child := m.accounts[childID]
	if parent == nil || child == nil {
		return ErrUnknownAccount
	}
	allocation, ok := parent.children[childID]
	if !ok {
		return ErrUnknownAccount
	}
	if allocation.settled {
		return ErrAllocationConflict
	}
	if usageExceeds(actual, allocation.budget) {
		return ErrBudgetExceeded
	}
	parent.reserved = subtract(parent.reserved, allocation.budget)
	parent.used = addUsage(parent.used, actual)
	child.used = actual
	allocation.settled = true
	parent.children[childID] = allocation
	return nil
}

// Remaining returns granted limits less settled use and active child reservations.
func (m *Manager) Remaining(id string) (Budget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a := m.accounts[id]
	if a == nil {
		return Budget{}, ErrUnknownAccount
	}
	return remaining(a.granted, a.used, a.reserved), nil
}
func (m *Manager) Snapshot(id string) (AccountSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a := m.accounts[id]
	if a == nil {
		return AccountSnapshot{}, ErrUnknownAccount
	}
	return AccountSnapshot{Granted: a.granted, Used: a.used, Reserved: a.reserved}, nil
}

func (m *Manager) normalize(b Budget) Budget {
	if isZero(b) {
		return m.defaults
	}
	if b.TotalTokens == 0 && b.InputTokens <= math.MaxInt64-b.OutputTokens {
		b.TotalTokens = b.InputTokens + b.OutputTokens
	}
	return b
}
func isZero(b Budget) bool { return b == Budget{} }
func validate(b Budget) error {
	for _, n := range []int64{b.InputTokens, b.OutputTokens, b.TotalTokens, b.ModelCalls, b.ToolCalls, b.AgentMessages, b.TimeSeconds, b.ParallelAgents, b.Rebuttals} {
		if n < 0 {
			return fmt.Errorf("budget limits cannot be negative")
		}
	}
	if math.IsNaN(b.CostUSD) || math.IsInf(b.CostUSD, 0) || b.CostUSD < 0 {
		return fmt.Errorf("budget cost must be finite and non-negative")
	}
	if b.InputTokens > math.MaxInt64-b.OutputTokens {
		return fmt.Errorf("token budget overflows")
	}
	if b.TotalTokens > 0 && b.InputTokens+b.OutputTokens > b.TotalTokens {
		return fmt.Errorf("input plus output token limits exceed total token limit")
	}
	return nil
}
func validateUsage(u Usage) error {
	for _, n := range []int64{u.InputTokens, u.OutputTokens, u.TotalTokens, u.ModelCalls, u.ToolCalls, u.AgentMessages, u.TimeSeconds, u.ParallelAgents, u.Rebuttals} {
		if n < 0 {
			return fmt.Errorf("usage counters cannot be negative")
		}
	}
	if u.InputTokens > math.MaxInt64-u.OutputTokens {
		return fmt.Errorf("token usage overflows")
	}
	if u.TotalTokens > 0 && u.TotalTokens < u.InputTokens+u.OutputTokens {
		return fmt.Errorf("total token usage is below input plus output")
	}
	if math.IsNaN(u.CostUSD) || math.IsInf(u.CostUSD, 0) || u.CostUSD < 0 {
		return fmt.Errorf("usage cost must be finite and non-negative")
	}
	return nil
}
func remaining(granted Budget, used Usage, reserved Budget) Budget {
	return Budget{InputTokens: nonnegative(granted.InputTokens - used.InputTokens - reserved.InputTokens), OutputTokens: nonnegative(granted.OutputTokens - used.OutputTokens - reserved.OutputTokens), TotalTokens: nonnegative(granted.TotalTokens - used.TotalTokens - reserved.TotalTokens), ModelCalls: nonnegative(granted.ModelCalls - used.ModelCalls - reserved.ModelCalls), ToolCalls: nonnegative(granted.ToolCalls - used.ToolCalls - reserved.ToolCalls), AgentMessages: nonnegative(granted.AgentMessages - used.AgentMessages - reserved.AgentMessages), TimeSeconds: nonnegative(granted.TimeSeconds - used.TimeSeconds - reserved.TimeSeconds), CostUSD: max(0, granted.CostUSD-used.CostUSD-reserved.CostUSD), ParallelAgents: nonnegative(granted.ParallelAgents - reserved.ParallelAgents), Rebuttals: nonnegative(granted.Rebuttals - used.Rebuttals - reserved.Rebuttals)}
}
func exceeds(a, b Budget) bool {
	return a.InputTokens > b.InputTokens || a.OutputTokens > b.OutputTokens || a.TotalTokens > b.TotalTokens || a.ModelCalls > b.ModelCalls || a.ToolCalls > b.ToolCalls || a.AgentMessages > b.AgentMessages || a.TimeSeconds > b.TimeSeconds || a.CostUSD > b.CostUSD+1e-12 || a.ParallelAgents > b.ParallelAgents || a.Rebuttals > b.Rebuttals
}
func usageExceeds(u Usage, b Budget) bool {
	return u.InputTokens > b.InputTokens || u.OutputTokens > b.OutputTokens || (u.TotalTokens > b.TotalTokens) || u.ModelCalls > b.ModelCalls || u.ToolCalls > b.ToolCalls || u.AgentMessages > b.AgentMessages || u.TimeSeconds > b.TimeSeconds || u.CostUSD > b.CostUSD+1e-12 || u.ParallelAgents > b.ParallelAgents || u.Rebuttals > b.Rebuttals
}
func add(a, b Budget) Budget {
	return Budget{a.InputTokens + b.InputTokens, a.OutputTokens + b.OutputTokens, a.TotalTokens + b.TotalTokens, a.ModelCalls + b.ModelCalls, a.ToolCalls + b.ToolCalls, a.AgentMessages + b.AgentMessages, a.TimeSeconds + b.TimeSeconds, a.CostUSD + b.CostUSD, a.ParallelAgents + b.ParallelAgents, a.Rebuttals + b.Rebuttals}
}
func subtract(a, b Budget) Budget {
	return Budget{nonnegative(a.InputTokens - b.InputTokens), nonnegative(a.OutputTokens - b.OutputTokens), nonnegative(a.TotalTokens - b.TotalTokens), nonnegative(a.ModelCalls - b.ModelCalls), nonnegative(a.ToolCalls - b.ToolCalls), nonnegative(a.AgentMessages - b.AgentMessages), nonnegative(a.TimeSeconds - b.TimeSeconds), max(0, a.CostUSD-b.CostUSD), nonnegative(a.ParallelAgents - b.ParallelAgents), nonnegative(a.Rebuttals - b.Rebuttals)}
}
func addUsage(a Usage, b Usage) Usage {
	return Usage{a.InputTokens + b.InputTokens, a.OutputTokens + b.OutputTokens, a.TotalTokens + b.TotalTokens, a.ModelCalls + b.ModelCalls, a.ToolCalls + b.ToolCalls, a.AgentMessages + b.AgentMessages, a.TimeSeconds + b.TimeSeconds, a.CostUSD + b.CostUSD, a.ParallelAgents + b.ParallelAgents, a.Rebuttals + b.Rebuttals}
}
func nonnegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}
