// Package contextmgr assembles bounded, provenance-tracked model context.
package contextmgr

import (
	"fmt"
	"html"
	"math"
	"sort"
	"strings"
)

type Kind string

const (
	SystemPrompt Kind = "system_prompt"
	Role         Kind = "role"
	Task         Kind = "task"
	Memory       Kind = "memory"
	Artifact     Kind = "artifact"
	Message      Kind = "message"
	ToolResult   Kind = "tool_result"
)

type Candidate struct {
	Kind               Kind
	Ref                string
	Content            string
	Relevance          float64
	Required           bool
	Trusted            bool
	Stale              bool
	Superseded         bool
	CognitiveGuardrail bool
}
type Request struct {
	AgentID          string
	TokenBudget      int
	MinimumRelevance float64
	Items            []Candidate
	Counter          TokenCounter
}
type Included struct {
	Kind               Kind   `json:"kind"`
	Ref                string `json:"ref"`
	Tokens             int    `json:"tokens"`
	Reason             string `json:"reason"`
	Content            string `json:"-"`
	CognitiveGuardrail bool   `json:"-"`
}
type Dropped struct {
	Kind   Kind   `json:"kind"`
	Ref    string `json:"ref"`
	Reason string `json:"reason"`
}
type Result struct {
	AgentID     string     `json:"agent_id"`
	TokenBudget int        `json:"token_budget"`
	TokensUsed  int        `json:"tokens_used"`
	Items       []Included `json:"items"`
	Dropped     []Dropped  `json:"dropped,omitempty"`
	Text        string     `json:"-"`
}
type TokenCounter interface{ Estimate(string) (int, error) }

type ConservativeCounter struct{}

func (ConservativeCounter) Estimate(text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	return int(math.Ceil(float64(len([]byte(text))) / 3)), nil
}

func Build(request Request) (Result, error) {
	if request.AgentID == "" || request.TokenBudget < 1 {
		return Result{}, fmt.Errorf("agent ID and positive token budget are required")
	}
	counter := request.Counter
	if counter == nil {
		counter = ConservativeCounter{}
	}
	if request.MinimumRelevance < 0 || request.MinimumRelevance > 1 {
		return Result{}, fmt.Errorf("minimum relevance must be between zero and one")
	}
	result := Result{AgentID: request.AgentID, TokenBudget: request.TokenBudget, Items: []Included{}, Dropped: []Dropped{}}
	input := append([]Candidate(nil), request.Items...)
	sort.SliceStable(input, func(i, j int) bool {
		a, b := input[i], input[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Ref != b.Ref {
			return a.Ref < b.Ref
		}
		if a.Required != b.Required {
			return a.Required
		}
		if a.Relevance != b.Relevance {
			return a.Relevance > b.Relevance
		}
		if a.Trusted != b.Trusted {
			return a.Trusted
		}
		if a.Stale != b.Stale {
			return !a.Stale
		}
		if a.Superseded != b.Superseded {
			return !a.Superseded
		}
		return a.Content < b.Content
	})
	type scored struct {
		candidate Candidate
		index     int
		tokens    int
		rank      int
	}
	eligible := make([]scored, 0, len(input))
	seen := map[string]bool{}
	for i, candidate := range input {
		if candidate.Kind == "" || candidate.Ref == "" {
			return Result{}, fmt.Errorf("context item kind and source reference are required")
		}
		key := string(candidate.Kind) + "\x00" + candidate.Ref
		if seen[key] {
			result.Dropped = append(result.Dropped, Dropped{candidate.Kind, candidate.Ref, "duplicate"})
			continue
		}
		seen[key] = true
		if !candidate.Required && candidate.Relevance < request.MinimumRelevance {
			result.Dropped = append(result.Dropped, Dropped{candidate.Kind, candidate.Ref, "low_relevance"})
			continue
		}
		if (candidate.Kind == Memory || candidate.Kind == Artifact) && (!candidate.Trusted || candidate.Stale || candidate.Superseded) {
			if candidate.Required {
				return Result{}, fmt.Errorf("required context source %q is stale or untrusted", candidate.Ref)
			}
			result.Dropped = append(result.Dropped, Dropped{candidate.Kind, candidate.Ref, "untrusted"})
			continue
		}
		tokens, err := counter.Estimate(candidate.Content)
		if err != nil {
			return Result{}, fmt.Errorf("estimate context item %s: %w", candidate.Ref, err)
		}
		eligible = append(eligible, scored{candidate: candidate, index: i, tokens: tokens, rank: rank(candidate)})
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		a, b := eligible[i], eligible[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if a.candidate.Required != b.candidate.Required {
			return a.candidate.Required
		}
		if a.candidate.Relevance != b.candidate.Relevance {
			return a.candidate.Relevance > b.candidate.Relevance
		}
		if a.candidate.Ref != b.candidate.Ref {
			return a.candidate.Ref < b.candidate.Ref
		}
		return a.index < b.index
	})
	for _, item := range eligible {
		reason := "selected_by_priority"
		if item.candidate.Required {
			reason = "required"
		}
		result.Items = append(result.Items, Included{Kind: item.candidate.Kind, Ref: item.candidate.Ref, Tokens: item.tokens, Reason: reason, Content: item.candidate.Content, CognitiveGuardrail: item.candidate.CognitiveGuardrail})
		result.Text = render(result.Items)
		tokens, err := counter.Estimate(result.Text)
		if err != nil {
			return Result{}, err
		}
		if tokens > request.TokenBudget {
			result.Items = result.Items[:len(result.Items)-1]
			result.Dropped = append(result.Dropped, Dropped{item.candidate.Kind, item.candidate.Ref, "over_budget"})
			if item.candidate.Required {
				// Required input is never silently dropped; optional entries after it may still be removed.
				return Result{}, fmt.Errorf("required context item %q exceeds configured token budget", item.candidate.Ref)
			}
			result.Text = render(result.Items)
		}
	}
	result.Text = render(result.Items)
	tokens, err := counter.Estimate(result.Text)
	if err != nil {
		return Result{}, err
	}
	result.TokensUsed = tokens
	if tokens > request.TokenBudget {
		return Result{}, fmt.Errorf("assembled context exceeds configured token budget")
	}
	sort.Slice(result.Dropped, func(i, j int) bool {
		a, b := result.Dropped[i], result.Dropped[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Ref != b.Ref {
			return a.Ref < b.Ref
		}
		return a.Reason < b.Reason
	})
	return result, nil
}

func rank(c Candidate) int {
	switch c.Kind {
	case SystemPrompt:
		return 0
	case Role:
		return 1
	case Task:
		return 2
	case Message:
		return 3
	case Artifact:
		if c.Required {
			return 4
		}
		return 6
	case Memory:
		return 5
	case ToolResult:
		return 7
	default:
		return 8
	}
}

func render(items []Included) string {
	var out strings.Builder
	var successes, guardrails []string
	for _, item := range items {
		if item.Kind == Memory {
			block := fmt.Sprintf("[%s %s]\n%s", item.Kind, item.Ref, html.EscapeString(item.Content))
			if item.CognitiveGuardrail {
				guardrails = append(guardrails, block)
			} else {
				successes = append(successes, block)
			}
			continue
		}
		fmt.Fprintf(&out, "[%s %s]\n%s\n\n", item.Kind, item.Ref, item.Content)
	}
	if len(successes) > 0 {
		out.WriteString("<historical_success>\n")
		out.WriteString(strings.Join(successes, "\n\n"))
		out.WriteString("\n</historical_success>\n\n")
	}
	if len(guardrails) > 0 {
		out.WriteString("<confirmed_blacklist>\n")
		out.WriteString(strings.Join(guardrails, "\n\n"))
		out.WriteString("\n</confirmed_blacklist>\n")
	}
	return strings.TrimSpace(out.String())
}
