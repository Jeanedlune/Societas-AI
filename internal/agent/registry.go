// Package agent contains runtime agent metadata and its immutable workspace
// registry.
package agent

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Jeanedlune/Societas-AI/internal/config"
)

// Agent is one validated agent definition captured for a run.
type Agent struct {
	ID           string
	Role         string
	SystemPrompt string
	Model        string
	Provider     string
	Autonomy     int
	Tools        []string
	Permissions  map[string]config.PermissionConfig
}

// RoutingAgent contains only fields needed to present a safe routing choice.
// Prompts, permissions, tools, and provider details are intentionally omitted.
type RoutingAgent struct {
	ID   string
	Role string
}

// Registry is an immutable snapshot of active agents from a validated config.
// Build one registry at run start and keep it for the lifetime of that run.
type Registry struct {
	agents         map[string]Agent
	ids            []string
	escalationLead string
}

// NewRegistry validates cfg and snapshots its active agents. It rejects an
// empty squad so the runtime cannot accidentally start with no routable agents.
func NewRegistry(cfg *config.Config) (*Registry, error) {
	if cfg == nil {
		return nil, errors.New("agent registry: workspace config is nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("agent registry: invalid workspace config: %w", err)
	}
	if len(cfg.Agents) == 0 {
		return nil, errors.New("agent registry: agents must contain at least one active agent")
	}

	ids := make([]string, 0, len(cfg.Agents))
	for id := range cfg.Agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	agents := make(map[string]Agent, len(ids))
	for _, id := range ids {
		definition := cfg.Agents[id]
		agents[id] = Agent{
			ID:           id,
			Role:         definition.Role,
			SystemPrompt: definition.SystemPrompt,
			Model:        definition.Model,
			Provider:     definition.Provider,
			Autonomy:     definition.Autonomy,
			Tools:        append([]string(nil), definition.Tools...),
			Permissions:  clonePermissionMap(definition.Permissions),
		}
	}
	return &Registry{
		agents:         agents,
		ids:            ids,
		escalationLead: cfg.EscalationLead,
	}, nil
}

// Get returns a detached copy of an agent by its exact, case-sensitive ID.
func (r *Registry) Get(id string) (Agent, bool) {
	if r == nil {
		return Agent{}, false
	}
	agent, ok := r.agents[id]
	if !ok {
		return Agent{}, false
	}
	agent.Tools = append([]string(nil), agent.Tools...)
	agent.Permissions = clonePermissionMap(agent.Permissions)
	return agent, true
}

// IDs returns active agent IDs in stable lexical order.
func (r *Registry) IDs() []string {
	if r == nil {
		return nil
	}
	return append([]string(nil), r.ids...)
}

// RoutingAgents returns a stable, minimal list for routing prompts and dynamic
// enum construction. It contains no prompt text or permission details.
func (r *Registry) RoutingAgents() []RoutingAgent {
	if r == nil {
		return nil
	}
	result := make([]RoutingAgent, 0, len(r.ids))
	for _, id := range r.ids {
		result = append(result, RoutingAgent{ID: id, Role: r.agents[id].Role})
	}
	return result
}

// Has reports whether id is registered in this run's snapshot.
func (r *Registry) Has(id string) bool {
	if r == nil {
		return false
	}
	_, ok := r.agents[id]
	return ok
}

// EscalationLead returns the validated escalation target captured from config.
func (r *Registry) EscalationLead() string {
	if r == nil {
		return ""
	}
	return r.escalationLead
}

func clonePermissionMap(source map[string]config.PermissionConfig) map[string]config.PermissionConfig {
	if source == nil {
		return nil
	}
	copy := make(map[string]config.PermissionConfig, len(source))
	for key, value := range source {
		value.Allowed = append([]string(nil), value.Allowed...)
		copy[key] = value
	}
	return copy
}
