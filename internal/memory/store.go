// Package memory stores provenance-bound memory and gates vector admission.
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Jeanedlune/Societas-AI/internal/ids"
)

var (
	ErrMemoryNotFound    = errors.New("memory was not found")
	ErrMemoryRejected    = errors.New("memory did not pass admission policy")
	ErrVectorUnavailable = errors.New("vector store is unavailable")
)

type Scope string

const (
	ScopeWorkspace Scope = "workspace"
	ScopeRun       Scope = "run"
	ScopeTask      Scope = "task"
	ScopeAgent     Scope = "agent"
)

type Entry struct {
	ID                 string          `json:"memory_id"`
	WorkspaceID        string          `json:"workspace_id"`
	Scope              Scope           `json:"scope"`
	OwnerID            string          `json:"owner_id"`
	Source             ArtifactRef     `json:"source"`
	Content            json.RawMessage `json:"content"`
	Confidence         float64         `json:"confidence"`
	FreshnessUntil     *time.Time      `json:"freshness_until,omitempty"`
	Sensitivity        string          `json:"sensitivity"`
	Tag                string          `json:"tag"`
	CognitiveGuardrail bool            `json:"cognitive_guardrail"`
	Status             string          `json:"status"`
	AdmissionStatus    string          `json:"admission_status"`
	SupersededBy       string          `json:"superseded_by,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}
type ArtifactRef struct {
	ID       string `json:"artifact_id"`
	Version  int    `json:"version"`
	Checksum string `json:"checksum"`
}
type CreateRequest struct {
	WorkspaceID    string
	Scope          Scope
	OwnerID        string
	Source         ArtifactRef
	Content        json.RawMessage
	Confidence     float64
	FreshnessUntil *time.Time
	Sensitivity    string
	Tag            string
}
type SearchRequest struct {
	WorkspaceID string
	Scope       Scope
	OwnerID     string
	Query       string
	Limit       int
}
type Hit struct {
	MemoryID string
	Score    float64
}
type VectorStore interface {
	Upsert(context.Context, Entry) error
	Search(context.Context, SearchRequest) ([]Hit, error)
	Delete(context.Context, string, string) error
}
type AdmissionPolicy interface {
	Allow(context.Context, Entry, string) bool
}

type Store struct {
	db      *sql.DB
	vectors VectorStore
	policy  AdmissionPolicy
}

func New(db *sql.DB, vectors VectorStore, policy AdmissionPolicy) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("memory store requires database")
	}
	return &Store{db: db, vectors: vectors, policy: policy}, nil
}

func (s *Store) CreateCandidate(ctx context.Context, r CreateRequest) (Entry, error) {
	if err := validateCreate(r); err != nil {
		return Entry{}, err
	}
	if err := s.validateSource(ctx, r.WorkspaceID, r.Source); err != nil {
		return Entry{}, err
	}
	id, err := ids.New("mem_")
	if err != nil {
		return Entry{}, err
	}
	now := time.Now().UTC()
	sensitivity := r.Sensitivity
	if sensitivity == "" {
		sensitivity = "normal"
	}
	guardrail := r.Tag == "anti_pattern"
	var freshness any
	if r.FreshnessUntil != nil {
		freshness = r.FreshnessUntil.UTC().Format(time.RFC3339Nano)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO memories(workspace_id,memory_id,source_artifact_id,source_artifact_version,status,content_json,created_at,updated_at,scope,owner_id,source_checksum,confidence,freshness_until,sensitivity,admission_status,cognitive_guardrail) VALUES(?,?,?,?,'active',?,?,?,?,?,?,?,?,?,'candidate',?)`, r.WorkspaceID, id, r.Source.ID, r.Source.Version, string(r.Content), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), r.Scope, r.OwnerID, r.Source.Checksum, r.Confidence, freshness, sensitivity, boolInt(guardrail))
	if err != nil {
		return Entry{}, fmt.Errorf("create memory candidate: %w", err)
	}
	return Entry{ID: id, WorkspaceID: r.WorkspaceID, Scope: r.Scope, OwnerID: r.OwnerID, Source: r.Source, Content: append(json.RawMessage(nil), r.Content...), Confidence: r.Confidence, FreshnessUntil: r.FreshnessUntil, Sensitivity: sensitivity, Tag: r.Tag, CognitiveGuardrail: guardrail, Status: "active", AdmissionStatus: "candidate", CreatedAt: now}, nil
}

func (s *Store) Admit(ctx context.Context, workspaceID, memoryID string) (Entry, error) {
	if s.vectors == nil {
		return Entry{}, ErrVectorUnavailable
	}
	entry, err := s.get(ctx, workspaceID, memoryID)
	if err != nil {
		return Entry{}, err
	}
	if entry.Status != "active" || entry.AdmissionStatus == "rejected" || entry.Sensitivity != "normal" || entry.FreshnessUntil != nil && time.Now().After(*entry.FreshnessUntil) {
		return Entry{}, ErrMemoryRejected
	}
	if err := s.validateSource(ctx, workspaceID, entry.Source); err != nil {
		return Entry{}, ErrMemoryRejected
	}
	var taskStatus, resultStatus string
	if err := s.db.QueryRowContext(ctx, `SELECT t.status,COALESCE(json_extract(t.data_json,'$.result_status'),'') FROM artifacts a JOIN tasks t ON t.workspace_id=a.workspace_id AND t.run_id=a.run_id AND t.task_id=a.task_id WHERE a.workspace_id=? AND a.artifact_id=? AND a.version=?`, workspaceID, entry.Source.ID, entry.Source.Version).Scan(&taskStatus, &resultStatus); err != nil {
		return Entry{}, ErrMemoryRejected
	}
	if taskStatus != "completed" || resultStatus != "success" {
		return Entry{}, ErrMemoryRejected
	}
	if s.policy != nil && !s.policy.Allow(ctx, entry, taskStatus+":"+resultStatus) {
		return Entry{}, ErrMemoryRejected
	}
	_, err = s.db.ExecContext(ctx, `UPDATE memories SET admission_status='admitted',updated_at=? WHERE workspace_id=? AND memory_id=? AND status='active' AND admission_status IN ('candidate','admitted')`, time.Now().UTC().Format(time.RFC3339Nano), workspaceID, memoryID)
	if err != nil {
		return Entry{}, err
	}
	entry.AdmissionStatus = "admitted"
	if err := s.vectors.Upsert(ctx, entry); err != nil {
		return entry, fmt.Errorf("upsert admitted memory: %w", err)
	}
	return entry, nil
}

// Search treats vector results as hints only: every hit is checked against
// SQLite for exact workspace/scope, admission, source checksum and freshness.
func (s *Store) Search(ctx context.Context, r SearchRequest) ([]Entry, error) {
	if s.vectors == nil {
		return nil, ErrVectorUnavailable
	}
	if r.WorkspaceID == "" || r.OwnerID == "" || strings.TrimSpace(r.Query) == "" || r.Limit < 1 || r.Limit > 100 || !validScope(r.Scope) {
		return nil, fmt.Errorf("invalid memory search request")
	}
	hits, err := s.vectors.Search(ctx, r)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(hits))
	for _, hit := range hits {
		entry, err := s.get(ctx, r.WorkspaceID, hit.MemoryID)
		if errors.Is(err, ErrMemoryNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if entry.Scope != r.Scope || entry.OwnerID != r.OwnerID || entry.AdmissionStatus != "admitted" || entry.Status != "active" || entry.Sensitivity != "normal" || entry.FreshnessUntil != nil && time.Now().After(*entry.FreshnessUntil) {
			continue
		}
		if err := s.validateSource(ctx, r.WorkspaceID, entry.Source); err != nil {
			continue
		}
		out = append(out, entry)
		if len(out) >= r.Limit {
			break
		}
	}
	return out, nil
}

func (s *Store) Supersede(ctx context.Context, workspaceID, memoryID, replacementID string) error {
	if replacementID == "" || replacementID == memoryID {
		return fmt.Errorf("valid replacement memory ID is required")
	}
	old, err := s.get(ctx, workspaceID, memoryID)
	if err != nil {
		return err
	}
	replacement, err := s.get(ctx, workspaceID, replacementID)
	if err != nil {
		return fmt.Errorf("replacement memory is unavailable: %w", err)
	}
	if replacement.Status != "active" || replacement.AdmissionStatus != "admitted" || replacement.Scope != old.Scope || replacement.OwnerID != old.OwnerID {
		return fmt.Errorf("replacement memory must be admitted, active, and share scope and owner")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE memories SET status='superseded',superseded_by=?,updated_at=? WHERE workspace_id=? AND memory_id=? AND status='active'`, replacementID, time.Now().UTC().Format(time.RFC3339Nano), workspaceID, memoryID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrMemoryNotFound
	}
	if s.vectors != nil {
		return s.vectors.Delete(ctx, workspaceID, memoryID)
	}
	return nil
}

func (s *Store) MarkSourceStale(ctx context.Context, workspaceID, artifactID string, version int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE memories SET status='stale',updated_at=? WHERE workspace_id=? AND source_artifact_id=? AND source_artifact_version=? AND status='active'`, time.Now().UTC().Format(time.RFC3339Nano), workspaceID, artifactID, version)
	return err
}

func (s *Store) get(ctx context.Context, workspaceID, memoryID string) (Entry, error) {
	var e Entry
	var content, created string
	var freshness sql.NullString
	var guardrail int
	err := s.db.QueryRowContext(ctx, `SELECT memory_id,workspace_id,scope,owner_id,source_artifact_id,source_artifact_version,source_checksum,content_json,confidence,freshness_until,sensitivity,cognitive_guardrail,status,admission_status,COALESCE(superseded_by,''),created_at FROM memories WHERE workspace_id=? AND memory_id=?`, workspaceID, memoryID).Scan(&e.ID, &e.WorkspaceID, &e.Scope, &e.OwnerID, &e.Source.ID, &e.Source.Version, &e.Source.Checksum, &content, &e.Confidence, &freshness, &e.Sensitivity, &guardrail, &e.Status, &e.AdmissionStatus, &e.SupersededBy, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Entry{}, ErrMemoryNotFound
	}
	if err != nil {
		return Entry{}, err
	}
	e.Content = json.RawMessage(content)
	e.CognitiveGuardrail = guardrail == 1
	e.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	if freshness.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, freshness.String)
		if err != nil {
			return Entry{}, err
		}
		e.FreshnessUntil = &parsed
	}
	if e.CognitiveGuardrail {
		e.Tag = "anti_pattern"
	}
	return e, nil
}

func (s *Store) validateSource(ctx context.Context, workspaceID string, source ArtifactRef) error {
	var checksum, staleness string
	var superseded sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT checksum,staleness,superseded_by FROM artifacts WHERE workspace_id=? AND artifact_id=? AND version=?`, workspaceID, source.ID, source.Version).Scan(&checksum, &staleness, &superseded)
	if err != nil {
		return fmt.Errorf("memory source artifact is unavailable")
	}
	if checksum != source.Checksum || staleness != "fresh" || superseded.Valid {
		return fmt.Errorf("memory source artifact is stale or checksum mismatched")
	}
	return nil
}
func validateCreate(r CreateRequest) error {
	if r.WorkspaceID == "" || r.OwnerID == "" || !validScope(r.Scope) || r.Source.ID == "" || r.Source.Version < 1 || !strings.HasPrefix(r.Source.Checksum, "sha256:") || !json.Valid(r.Content) || len(r.Content) > 16000 {
		return fmt.Errorf("memory candidate metadata or content is invalid")
	}
	if math.IsNaN(r.Confidence) || math.IsInf(r.Confidence, 0) || r.Confidence < 0 || r.Confidence > 1 {
		return fmt.Errorf("memory confidence must be between 0 and 1")
	}
	if r.Sensitivity != "" && r.Sensitivity != "normal" && r.Sensitivity != "sensitive" {
		return fmt.Errorf("memory sensitivity must be normal or sensitive")
	}
	switch r.Tag {
	case "best_practice", "anti_pattern", "research_finding":
	default:
		return fmt.Errorf("memory tag is not supported")
	}
	return nil
}
func validScope(s Scope) bool {
	return s == ScopeWorkspace || s == ScopeRun || s == ScopeTask || s == ScopeAgent
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
