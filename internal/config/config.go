// Package config loads and validates workspace.yaml files.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	identifierPattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)
	capabilityPattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]*(\.[a-z][a-z0-9_-]*)*$`)
	envReferencePattern = regexp.MustCompile(`^\$([A-Z_][A-Z0-9_]*)$`)
)

// Config is the versioned shape of a Societas workspace configuration.
type Config struct {
	Version          string                          `yaml:"version"`
	Workspace        WorkspaceConfig                 `yaml:"workspace"`
	Server           ServerConfig                    `yaml:"server"`
	Transport        TransportConfig                 `yaml:"transport"`
	Providers        map[string]ProviderConfig       `yaml:"providers"`
	Models           map[string]string               `yaml:"models"`
	Agents           map[string]AgentConfig          `yaml:"agents"`
	Budget           BudgetConfig                    `yaml:"budget"`
	ToolCapabilities map[string]ToolCapabilityConfig `yaml:"tool_capabilities"`
	EscalationLead   string                          `yaml:"escalation_lead"`
	Toolchain        *ToolchainConfig                `yaml:"toolchain,omitempty"`
	Risk             RiskConfig                      `yaml:"risk"`
	SessionPolicy    SessionPolicyConfig             `yaml:"session_policy"`
}

type WorkspaceConfig struct {
	Name string `yaml:"name"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type TransportConfig struct {
	EventBus string `yaml:"event_bus"`
}

// ProviderConfig keeps adapter credentials and provider-specific options under
// the provider's own namespace. Credentials contain environment references,
// never resolved secret values.
type ProviderConfig struct {
	Credentials map[string]string `yaml:"credentials"`
	Options     map[string]string `yaml:"options"`
}

type AgentConfig struct {
	Role         string                      `yaml:"role"`
	SystemPrompt string                      `yaml:"system_prompt"`
	Model        string                      `yaml:"model"`
	Provider     string                      `yaml:"provider,omitempty"`
	Autonomy     int                         `yaml:"autonomy"`
	Tools        []string                    `yaml:"tools"`
	Permissions  map[string]PermissionConfig `yaml:"permissions"`
}

// PermissionConfig describes one extensible permission domain. The policy
// engine interprets mode/scope for each registered capability at runtime.
type PermissionConfig struct {
	Mode    string   `yaml:"mode"`
	Scope   string   `yaml:"scope,omitempty"`
	Allowed []string `yaml:"allowed,omitempty"`
}

type BudgetConfig struct {
	MaxInputTokens    int64   `yaml:"max_input_tokens"`
	MaxOutputTokens   int64   `yaml:"max_output_tokens"`
	MaxTotalTokens    int64   `yaml:"max_total_tokens"`
	MaxModelCalls     int64   `yaml:"max_model_calls"`
	MaxToolCalls      int64   `yaml:"max_tool_calls"`
	MaxAgentMessages  int64   `yaml:"max_agent_messages"`
	MaxTimeSeconds    int64   `yaml:"max_time_seconds"`
	MaxCostUSD        float64 `yaml:"max_cost_usd"`
	MaxParallelAgents int64   `yaml:"max_parallel_agents"`
	MaxRebuttals      int64   `yaml:"max_rebuttals"`
}

type ToolCapabilityConfig struct {
	Idempotent           bool `yaml:"idempotent"`
	OperationKeyEnforced bool `yaml:"operation_key_enforced"`
}

type ToolchainConfig struct {
	Type           string          `yaml:"type"`
	SharedCacheDir string          `yaml:"shared_cache_dir"`
	Pipeline       []ValidatorStep `yaml:"pipeline"`
}

type ValidatorStep struct {
	Name   string `yaml:"name"`
	Cmd    string `yaml:"cmd"`
	Parser string `yaml:"parser"`
}

type RiskConfig struct {
	ApprovalDelivery      ApprovalDeliveryConfig `yaml:"approval_delivery"`
	DigestIntervalSeconds int64                  `yaml:"digest_interval_seconds"`
	NotifyOnCycleEnd      bool                   `yaml:"notify_on_cycle_end"`
	MaxItemsPerCard       int64                  `yaml:"max_items_per_card"`
	PathPatterns          map[string][]string    `yaml:"path_patterns"`
	IntentKeywords        map[string][]string    `yaml:"intent_keywords"`
}

type ApprovalDeliveryConfig struct {
	Critical string `yaml:"critical"`
	HighMode string `yaml:"high_mode"`
	Default  string `yaml:"default"`
}

type SessionPolicyConfig struct {
	Mode                string `yaml:"mode"`
	MaxTurnsBeforeFlush int64  `yaml:"max_turns_before_flush"`
	EnablePromptCaching *bool  `yaml:"enable_prompt_caching"`
}

// Load reads a workspace file, applies safe finite defaults, and validates it.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read workspace config %q: %w", path, err)
	}
	return Parse(data)
}

// Parse strictly decodes one YAML document, applies defaults, and validates it.
func Parse(data []byte) (*Config, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("workspace.yaml: %w", err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return nil, formatDecodeError(err, &root)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("workspace.yaml: multiple YAML documents are not supported")
		}
		return nil, fmt.Errorf("workspace.yaml: %w", err)
	}

	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ApplyDefaults fills omitted configuration with finite limits and deny-safe
// behavior. A zero-valued limit means “use the default”; limits cannot be
// disabled by setting them to zero.
func (c *Config) ApplyDefaults() {
	if c.Version == "" {
		c.Version = "1"
	}
	if c.Workspace.Name == "" {
		c.Workspace.Name = "general-workspace"
	}
	if c.Server.Host == "" {
		c.Server.Host = "127.0.0.1"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 7443
	}
	if c.Transport.EventBus == "" {
		c.Transport.EventBus = "go-channel"
	}
	if c.Providers == nil {
		c.Providers = map[string]ProviderConfig{}
	}
	if c.Models == nil {
		c.Models = map[string]string{}
	}
	if c.Agents == nil {
		c.Agents = map[string]AgentConfig{}
	}
	if c.ToolCapabilities == nil {
		c.ToolCapabilities = map[string]ToolCapabilityConfig{}
	}
	if c.EscalationLead == "" {
		c.EscalationLead = "human"
	}
	defaults := DefaultBudget()
	if c.Budget.MaxInputTokens == 0 {
		c.Budget.MaxInputTokens = defaults.MaxInputTokens
	}
	if c.Budget.MaxOutputTokens == 0 {
		c.Budget.MaxOutputTokens = defaults.MaxOutputTokens
	}
	if c.Budget.MaxTotalTokens == 0 {
		c.Budget.MaxTotalTokens = defaults.MaxTotalTokens
	}
	if c.Budget.MaxModelCalls == 0 {
		c.Budget.MaxModelCalls = defaults.MaxModelCalls
	}
	if c.Budget.MaxToolCalls == 0 {
		c.Budget.MaxToolCalls = defaults.MaxToolCalls
	}
	if c.Budget.MaxAgentMessages == 0 {
		c.Budget.MaxAgentMessages = defaults.MaxAgentMessages
	}
	if c.Budget.MaxTimeSeconds == 0 {
		c.Budget.MaxTimeSeconds = defaults.MaxTimeSeconds
	}
	if c.Budget.MaxCostUSD == 0 {
		c.Budget.MaxCostUSD = defaults.MaxCostUSD
	}
	if c.Budget.MaxParallelAgents == 0 {
		c.Budget.MaxParallelAgents = defaults.MaxParallelAgents
	}
	if c.Budget.MaxRebuttals == 0 {
		c.Budget.MaxRebuttals = defaults.MaxRebuttals
	}
	if c.Risk.ApprovalDelivery.Critical == "" {
		c.Risk.ApprovalDelivery.Critical = "immediate"
	}
	if c.Risk.ApprovalDelivery.HighMode == "" {
		c.Risk.ApprovalDelivery.HighMode = "immediate"
	}
	if c.Risk.ApprovalDelivery.Default == "" {
		c.Risk.ApprovalDelivery.Default = "immediate"
	}
	if c.Risk.DigestIntervalSeconds == 0 {
		c.Risk.DigestIntervalSeconds = 900
	}
	if c.Risk.MaxItemsPerCard == 0 {
		c.Risk.MaxItemsPerCard = 20
	}
	if c.Risk.PathPatterns == nil {
		c.Risk.PathPatterns = map[string][]string{}
	}
	if c.Risk.IntentKeywords == nil {
		c.Risk.IntentKeywords = map[string][]string{}
	}
	if c.SessionPolicy.Mode == "" {
		c.SessionPolicy.Mode = "sticky_until_done"
	}
	if c.SessionPolicy.MaxTurnsBeforeFlush == 0 {
		c.SessionPolicy.MaxTurnsBeforeFlush = 15
	}
	if c.SessionPolicy.EnablePromptCaching == nil {
		enabled := true
		c.SessionPolicy.EnablePromptCaching = &enabled
	}
}

// DefaultBudget returns the finite workspace budget described in Doc 02.
func DefaultBudget() BudgetConfig {
	return BudgetConfig{
		MaxInputTokens: 12000, MaxOutputTokens: 3000, MaxTotalTokens: 15000,
		MaxModelCalls: 10, MaxToolCalls: 20, MaxAgentMessages: 40,
		MaxTimeSeconds: 300, MaxCostUSD: 0.50, MaxParallelAgents: 4,
		MaxRebuttals: 2,
	}
}

// Validate reports configuration errors with the offending field path.
func (c Config) Validate() error {
	if c.Version != "1" {
		return fieldError("version", "must be \"1\"")
	}
	if strings.TrimSpace(c.Workspace.Name) == "" {
		return fieldError("workspace.name", "is required")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fieldError("server.port", "must be between 1 and 65535")
	}
	if c.Transport.EventBus != "go-channel" {
		return fieldError("transport.event_bus", "must be go-channel")
	}
	for _, path := range []string{"budget.max_input_tokens", "budget.max_output_tokens", "budget.max_total_tokens", "budget.max_model_calls", "budget.max_tool_calls", "budget.max_agent_messages", "budget.max_time_seconds", "budget.max_parallel_agents", "budget.max_rebuttals"} {
		if budgetValue(c.Budget, path) <= 0 {
			return fieldError(path, "must be greater than zero")
		}
	}
	if c.Budget.MaxCostUSD <= 0 {
		return fieldError("budget.max_cost_usd", "must be greater than zero")
	}
	if c.Budget.MaxInputTokens > c.Budget.MaxTotalTokens-c.Budget.MaxOutputTokens {
		return fieldError("budget.max_total_tokens", "must be at least max_input_tokens plus max_output_tokens")
	}
	if c.Risk.ApprovalDelivery.Critical != "immediate" {
		return fieldError("risk.approval_delivery.critical", "must be immediate")
	}
	if !validDelivery(c.Risk.ApprovalDelivery.HighMode) {
		return fieldError("risk.approval_delivery.high_mode", "must be immediate or digest")
	}
	if !validDelivery(c.Risk.ApprovalDelivery.Default) {
		return fieldError("risk.approval_delivery.default", "must be immediate or digest")
	}
	if (c.Risk.ApprovalDelivery.HighMode == "digest" || c.Risk.ApprovalDelivery.Default == "digest") && c.Risk.DigestIntervalSeconds <= 0 {
		return fieldError("risk.digest_interval_seconds", "must be positive when digest delivery is enabled")
	}
	if c.Risk.MaxItemsPerCard <= 0 {
		return fieldError("risk.max_items_per_card", "must be greater than zero")
	}
	for _, field := range []struct {
		name   string
		values map[string][]string
	}{{"risk.path_patterns", c.Risk.PathPatterns}, {"risk.intent_keywords", c.Risk.IntentKeywords}} {
		for _, tier := range sortedKeys(field.values) {
			if tier != "critical" && tier != "high" {
				return fieldError(field.name+"."+tier, "risk tier must be critical or high")
			}
			for i, value := range field.values[tier] {
				if strings.TrimSpace(value) == "" {
					return fieldError(fmt.Sprintf("%s.%s[%d]", field.name, tier, i), "must not be empty")
				}
			}
		}
	}
	if c.SessionPolicy.Mode != "sticky_until_done" && c.SessionPolicy.Mode != "stateless_step" {
		return fieldError("session_policy.mode", "must be sticky_until_done or stateless_step")
	}
	if c.SessionPolicy.MaxTurnsBeforeFlush <= 0 {
		return fieldError("session_policy.max_turns_before_flush", "must be greater than zero")
	}
	if c.Server.Host == "" {
		return fieldError("server.host", "is required")
	}
	if len(c.Models) == 0 {
		return fieldError("models", "must define at least one model alias")
	}
	for _, key := range sortedKeys(c.Models) {
		if !identifierPattern.MatchString(key) {
			return fieldError("models."+key, "model alias must be a lowercase slug")
		}
		if strings.TrimSpace(c.Models[key]) == "" {
			return fieldError("models."+key, "must not be empty")
		}
	}
	for _, capability := range sortedKeys(c.ToolCapabilities) {
		if !capabilityPattern.MatchString(capability) {
			return fieldError("tool_capabilities."+capability, "capability ID must be a lowercase dotted name")
		}
	}
	for _, id := range sortedKeys(c.Providers) {
		if !identifierPattern.MatchString(id) {
			return fieldError("providers."+id, "provider ID must be a lowercase slug")
		}
		for _, key := range sortedKeys(c.Providers[id].Credentials) {
			if !identifierPattern.MatchString(key) {
				return fieldError("providers."+id+".credentials."+key, "credential name must be a lowercase slug")
			}
			value := c.Providers[id].Credentials[key]
			if value != "" && !envReferencePattern.MatchString(value) {
				return fieldError("providers."+id+".credentials."+key, "must be an environment reference such as $PROVIDER_API_KEY")
			}
		}
	}
	for _, id := range sortedKeys(c.Agents) {
		agent := c.Agents[id]
		if !identifierPattern.MatchString(id) {
			return fieldError("agents."+id, "agent ID must be a lowercase slug")
		}
		if strings.TrimSpace(agent.Role) == "" {
			return fieldError("agents."+id+".role", "is required")
		}
		if strings.TrimSpace(agent.SystemPrompt) == "" {
			return fieldError("agents."+id+".system_prompt", "is required")
		}
		if strings.TrimSpace(agent.Model) == "" || !containsValue(c.Models, agent.Model) {
			return fieldError("agents."+id+".model", "must reference a model listed in models")
		}
		if agent.Autonomy < 0 || agent.Autonomy > 5 {
			return fieldError("agents."+id+".autonomy", "must be between 0 and 5")
		}
		if agent.Provider != "" {
			if _, exists := c.Providers[agent.Provider]; !exists {
				return fieldError("agents."+id+".provider", "must reference a configured provider")
			}
		}
		seenTools := make(map[string]struct{}, len(agent.Tools))
		for i, tool := range agent.Tools {
			path := fmt.Sprintf("agents.%s.tools[%d]", id, i)
			if _, exists := c.ToolCapabilities[tool]; !exists {
				return fieldError(path, "references an unknown tool capability")
			}
			if _, exists := seenTools[tool]; exists {
				return fieldError(path, "duplicates a tool capability")
			}
			seenTools[tool] = struct{}{}
		}
		for _, capability := range sortedKeys(agent.Permissions) {
			permission := agent.Permissions[capability]
			path := "agents." + id + ".permissions." + capability
			if strings.TrimSpace(capability) == "" || strings.TrimSpace(permission.Mode) == "" {
				return fieldError(path, "capability and mode are required")
			}
			if permission.Mode == "allowlist" && len(permission.Allowed) == 0 {
				return fieldError(path+".allowed", "must list at least one item for allowlist mode")
			}
			for i, allowed := range permission.Allowed {
				if strings.TrimSpace(allowed) == "" {
					return fieldError(fmt.Sprintf("%s.allowed[%d]", path, i), "must not be empty")
				}
			}
		}
	}
	if c.EscalationLead != "human" {
		if _, exists := c.Agents[c.EscalationLead]; !exists {
			return fieldError("escalation_lead", "must be human or a configured agent ID")
		}
	}
	if c.Toolchain != nil {
		if c.Toolchain.Type == "" {
			return fieldError("toolchain.type", "is required when toolchain is configured")
		}
		for i, step := range c.Toolchain.Pipeline {
			path := fmt.Sprintf("toolchain.pipeline[%d]", i)
			if step.Name == "" || step.Cmd == "" || step.Parser == "" {
				return fieldError(path, "name, cmd, and parser are required")
			}
		}
	}
	return nil
}

// Permission returns the configured permission for a capability. Missing
// entries always deny access.
func (a AgentConfig) Permission(capability string) PermissionConfig {
	if permission, ok := a.Permissions[capability]; ok && permission.Mode != "" {
		return permission
	}
	return PermissionConfig{Mode: "deny"}
}

// Credential resolves one provider credential reference without storing the
// resolved secret back into Config.
func (c Config) Credential(provider, key string, lookupEnv func(string) (string, bool)) (string, error) {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	providerConfig, ok := c.Providers[provider]
	if !ok {
		return "", fieldError("providers."+provider, "is not configured")
	}
	reference := providerConfig.Credentials[key]
	match := envReferencePattern.FindStringSubmatch(reference)
	path := "providers." + provider + ".credentials." + key
	if len(match) != 2 {
		return "", fieldError(path, "does not contain an environment reference")
	}
	value, ok := lookupEnv(match[1])
	if !ok || value == "" {
		return "", fieldError(path, "environment variable "+match[1]+" is not set")
	}
	return value, nil
}

func formatDecodeError(err error, root *yaml.Node) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return fmt.Errorf("workspace.yaml: %w", err)
	}
	parts := make([]string, 0, len(typeErr.Errors))
	for _, detail := range typeErr.Errors {
		path := "workspace.yaml"
		var line int
		if _, scanErr := fmt.Sscanf(detail, "line %d:", &line); scanErr == nil {
			if fieldPath := yamlPathAtLine(root, line); fieldPath != "" {
				path += ": " + fieldPath
			}
		}
		parts = append(parts, path+": "+detail)
	}
	return errors.New(strings.Join(parts, "; "))
}

func yamlPathAtLine(root *yaml.Node, line int) string {
	if root == nil || len(root.Content) == 0 {
		return ""
	}
	return findYAMLPath(root.Content[0], line, "")
}

func findYAMLPath(node *yaml.Node, line int, path string) string {
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			childPath := key.Value
			if path != "" {
				childPath = path + "." + childPath
			}
			if line >= key.Line && line <= nodeEndLine(value) {
				if deeper := findYAMLPath(value, line, childPath); deeper != "" {
					return deeper
				}
				return childPath
			}
		}
	case yaml.SequenceNode:
		for i, value := range node.Content {
			if line >= value.Line && line <= nodeEndLine(value) {
				childPath := fmt.Sprintf("%s[%d]", path, i)
				if deeper := findYAMLPath(value, line, childPath); deeper != "" {
					return deeper
				}
				return childPath
			}
		}
	}
	return ""
}

func nodeEndLine(node *yaml.Node) int {
	end := node.Line
	for _, child := range node.Content {
		if childEnd := nodeEndLine(child); childEnd > end {
			end = childEnd
		}
	}
	return end
}

func fieldError(path, message string) error {
	return fmt.Errorf("%s: %s", path, message)
}

func validDelivery(value string) bool { return value == "immediate" || value == "digest" }

func containsValue(values map[string]string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func budgetValue(b BudgetConfig, path string) int64 {
	switch path {
	case "budget.max_input_tokens":
		return b.MaxInputTokens
	case "budget.max_output_tokens":
		return b.MaxOutputTokens
	case "budget.max_total_tokens":
		return b.MaxTotalTokens
	case "budget.max_model_calls":
		return b.MaxModelCalls
	case "budget.max_tool_calls":
		return b.MaxToolCalls
	case "budget.max_agent_messages":
		return b.MaxAgentMessages
	case "budget.max_time_seconds":
		return b.MaxTimeSeconds
	case "budget.max_parallel_agents":
		return b.MaxParallelAgents
	case "budget.max_rebuttals":
		return b.MaxRebuttals
	default:
		return 0
	}
}
