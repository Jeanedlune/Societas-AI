// Package artifacts stores immutable, versioned task outputs on disk with SQLite metadata.
package artifacts

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	v1 "github.com/Jeanedlune/Societas-AI/internal/contracts/v1"
	"github.com/Jeanedlune/Societas-AI/internal/eventstore"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

var (
	ErrArtifactNotFound  = errors.New("artifact was not found")
	ErrArtifactDenied    = errors.New("artifact access denied")
	ErrArtifactVersion   = errors.New("artifact version conflict")
	ErrArtifactIntegrity = errors.New("artifact checksum mismatch")
)

var safeSegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Artifact struct {
	ID           string            `json:"id"`
	WorkspaceID  string            `json:"workspace_id"`
	RunID        string            `json:"run_id"`
	Version      int               `json:"version"`
	TaskID       string            `json:"task_id"`
	Name         string            `json:"name"`
	Kind         string            `json:"kind"`
	Checksum     string            `json:"checksum"`
	SizeBytes    int64             `json:"size_bytes"`
	CreatedBy    string            `json:"created_by"`
	CreatedAt    time.Time         `json:"created_at"`
	SupersededBy string            `json:"superseded_by,omitempty"`
	Staleness    string            `json:"staleness"`
	SourceRefs   []ArtifactVersion `json:"source_refs,omitempty"`
	Metadata     map[string]any    `json:"metadata,omitempty"`
}

type ArtifactVersion struct {
	ArtifactID string `json:"artifact_id"`
	Version    int    `json:"version"`
	Checksum   string `json:"checksum"`
}

type WriteRequest struct {
	ID          string
	WorkspaceID string
	RunID       string
	TaskID      string
	Version     int
	Name        string
	Kind        string
	CreatedBy   string
	Content     []byte
	SourceRefs  []ArtifactVersion
	Metadata    map[string]any
	RawEvent    []byte
}

type AccessPolicy interface {
	CanRead(context.Context, string, Artifact) bool
}

type Store struct {
	db     *sql.DB
	events *eventstore.Store
	root   string
	policy AccessPolicy
}

// New creates an artifact store rooted at a private directory. Reads default-deny
// unless the requester created the artifact or an explicit policy grants access.
func New(db *sql.DB, events *eventstore.Store, root string, policy AccessPolicy) (*Store, error) {
	if db == nil || events == nil {
		return nil, fmt.Errorf("artifact store requires database and event store")
	}
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("artifact storage root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve artifact storage root: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return nil, fmt.Errorf("create artifact storage root: %w", err)
	}
	return &Store{db: db, events: events, root: absolute, policy: policy}, nil
}

// Write adds a new immutable version and appends its canonical artifact event
// within the same SQLite transaction. Version is an expected CAS value.
func (s *Store) Write(ctx context.Context, request WriteRequest) (Artifact, bool, error) {
	if err := validateWrite(request); err != nil {
		return Artifact{}, false, err
	}
	sum := sha256.Sum256(request.Content)
	checksum := "sha256:" + hex.EncodeToString(sum[:])
	var eventEnvelope v1.Envelope
	if err := json.Unmarshal(request.RawEvent, &eventEnvelope); err != nil {
		return Artifact{}, false, fmt.Errorf("decode artifact event: %w", err)
	}
	if eventEnvelope.WorkspaceID != v1.WorkspaceID(request.WorkspaceID) || eventEnvelope.RunID != v1.RunID(request.RunID) || eventEnvelope.TaskID == nil || string(*eventEnvelope.TaskID) != request.TaskID || eventEnvelope.From != request.CreatedBy || eventEnvelope.To != "topic:all" {
		return Artifact{}, false, fmt.Errorf("artifact event identity does not match write request")
	}
	expectedType := v1.EventArtifactCreated
	if request.Version > 1 {
		expectedType = v1.EventArtifactUpdated
	}
	if eventEnvelope.Type != expectedType {
		return Artifact{}, false, fmt.Errorf("artifact version %d requires event %q", request.Version, expectedType)
	}
	eventArtifact, previousVersion, err := decodeArtifactPayload(eventEnvelope)
	if err != nil {
		return Artifact{}, false, err
	}
	createdAt, err := time.Parse(time.RFC3339Nano, eventEnvelope.Timestamp)
	if err != nil {
		return Artifact{}, false, fmt.Errorf("artifact event timestamp is invalid")
	}
	artifact := Artifact{ID: request.ID, WorkspaceID: request.WorkspaceID, RunID: request.RunID, Version: request.Version, TaskID: request.TaskID, Name: request.Name, Kind: request.Kind, Checksum: checksum, SizeBytes: int64(len(request.Content)), CreatedBy: request.CreatedBy, CreatedAt: createdAt.UTC(), Staleness: "fresh", SourceRefs: append([]ArtifactVersion(nil), request.SourceRefs...), Metadata: request.Metadata}
	if !eventMatches(eventArtifact, artifact) || ((request.Version > 1) && previousVersion != request.Version-1) {
		return Artifact{}, false, fmt.Errorf("artifact event payload does not match content metadata")
	}
	metadata, err := json.Marshal(struct {
		SourceRefs []ArtifactVersion `json:"source_refs,omitempty"`
		Metadata   map[string]any    `json:"metadata,omitempty"`
	}{artifact.SourceRefs, artifact.Metadata})
	if err != nil {
		return Artifact{}, false, err
	}
	relative := filepath.ToSlash(filepath.Join("artifacts", request.WorkspaceID, request.ID, fmt.Sprintf("v%d", request.Version), request.Name))
	full, err := s.resolve(relative)
	if err != nil {
		return Artifact{}, false, err
	}
	var duplicate bool
	err = storage.WithTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		appended, err := s.events.AppendInTx(ctx, tx, request.RawEvent)
		if err != nil {
			return err
		}
		if appended.Duplicate {
			var existing Artifact
			var existingMetadata string
			err := scanArtifact(tx.QueryRowContext(ctx, `SELECT workspace_id,run_id,artifact_id,version,task_id,name,kind,checksum,size_bytes,created_by,created_at,COALESCE(superseded_by,''),staleness,metadata_json FROM artifacts WHERE workspace_id=? AND artifact_id=? AND version=?`, request.WorkspaceID, request.ID, request.Version), &existing, &existingMetadata)
			if err != nil {
				return fmt.Errorf("duplicate artifact event has no metadata row: %w", err)
			}
			if existing.Checksum != artifact.Checksum || !sameArtifactIdentity(existing, artifact) {
				return ErrArtifactVersion
			}
			if err := decodeMetadata(existingMetadata, &existing); err != nil {
				return err
			}
			artifact = existing
			duplicate = true
			return nil
		}
		var latest sql.NullInt64
		err = tx.QueryRowContext(ctx, `SELECT MAX(version) FROM artifacts WHERE workspace_id=? AND artifact_id=?`, request.WorkspaceID, request.ID).Scan(&latest)
		if err != nil {
			return err
		}
		want := 1
		if latest.Valid {
			want = int(latest.Int64) + 1
		}
		if request.Version != want {
			return fmt.Errorf("%w: expected version %d, got %d", ErrArtifactVersion, want, request.Version)
		}
		for _, source := range artifact.SourceRefs {
			var sourceChecksum, staleness string
			var superseded sql.NullString
			err := tx.QueryRowContext(ctx, `SELECT checksum,staleness,superseded_by FROM artifacts WHERE workspace_id=? AND artifact_id=? AND version=?`, artifact.WorkspaceID, source.ArtifactID, source.Version).Scan(&sourceChecksum, &staleness, &superseded)
			if err != nil || sourceChecksum != source.Checksum || staleness != "fresh" || superseded.Valid {
				return fmt.Errorf("artifact source reference is missing, stale, or has a mismatched checksum")
			}
		}
		if err := writeAtomic(full, request.Content); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO artifacts(workspace_id,artifact_id,run_id,version,task_id,name,kind,relative_path,checksum,size_bytes,created_by,created_at,metadata_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, artifact.WorkspaceID, artifact.ID, artifact.RunID, artifact.Version, artifact.TaskID, artifact.Name, artifact.Kind, relative, artifact.Checksum, artifact.SizeBytes, artifact.CreatedBy, artifact.CreatedAt.Format(time.RFC3339Nano), string(metadata))
		if err != nil {
			_ = os.Remove(full)
			return fmt.Errorf("insert artifact metadata: %w", err)
		}
		if latest.Valid {
			_, err = tx.ExecContext(ctx, `UPDATE artifacts SET superseded_by=?,staleness='stale' WHERE workspace_id=? AND artifact_id=? AND version=?`, artifact.ID, artifact.WorkspaceID, artifact.ID, latest.Int64)
			if err != nil {
				_ = os.Remove(full)
				return err
			}
		}
		return nil
	})
	if err != nil {
		if !duplicate {
			_ = os.Remove(full)
		}
		return Artifact{}, false, fmt.Errorf("write artifact: %w", err)
	}
	return artifact, duplicate, nil
}

func (s *Store) Read(ctx context.Context, workspaceID, artifactID string, version int, requester string) (Artifact, []byte, error) {
	var artifact Artifact
	var relative, metadata string
	if err := scanArtifact(s.db.QueryRowContext(ctx, `SELECT workspace_id,run_id,artifact_id,version,task_id,name,kind,checksum,size_bytes,created_by,created_at,COALESCE(superseded_by,''),staleness,metadata_json,relative_path FROM artifacts WHERE workspace_id=? AND artifact_id=? AND version=?`, workspaceID, artifactID, version), &artifact, &metadata, &relative); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Artifact{}, nil, ErrArtifactNotFound
		}
		return Artifact{}, nil, err
	}
	if requester == "" {
		return Artifact{}, nil, ErrArtifactDenied
	}
	if requester != artifact.CreatedBy && (s.policy == nil || !s.policy.CanRead(ctx, requester, artifact)) {
		return Artifact{}, nil, ErrArtifactDenied
	}
	if err := decodeMetadata(metadata, &artifact); err != nil {
		return Artifact{}, nil, err
	}
	full, err := s.resolve(relative)
	if err != nil {
		return Artifact{}, nil, err
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return Artifact{}, nil, fmt.Errorf("read artifact content: %w", err)
	}
	sum := sha256.Sum256(content)
	if artifact.Checksum != "sha256:"+hex.EncodeToString(sum[:]) || artifact.SizeBytes != int64(len(content)) {
		return Artifact{}, nil, ErrArtifactIntegrity
	}
	return artifact, content, nil
}

func (s *Store) MarkStale(ctx context.Context, workspaceID, artifactID string, version int) error {
	result, err := s.db.ExecContext(ctx, `UPDATE artifacts SET staleness='stale' WHERE workspace_id=? AND artifact_id=? AND version=?`, workspaceID, artifactID, version)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrArtifactNotFound
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanArtifact(row rowScanner, a *Artifact, extra ...any) error {
	var created string
	args := []any{&a.WorkspaceID, &a.RunID, &a.ID, &a.Version, &a.TaskID, &a.Name, &a.Kind, &a.Checksum, &a.SizeBytes, &a.CreatedBy, &created, &a.SupersededBy, &a.Staleness}
	args = append(args, extra...)
	if err := row.Scan(args...); err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return fmt.Errorf("stored artifact timestamp is invalid")
	}
	a.CreatedAt = parsed.UTC()
	return nil
}

func validateWrite(r WriteRequest) error {
	if !safeSegment.MatchString(r.WorkspaceID) || !safeSegment.MatchString(r.ID) || !safeSegment.MatchString(r.RunID) || !safeSegment.MatchString(r.TaskID) {
		return fmt.Errorf("artifact identifiers must be safe path segments")
	}
	if r.Version < 1 || strings.TrimSpace(r.Name) == "" || filepath.Base(r.Name) != r.Name || strings.ContainsAny(r.Name, "/\\\x00") || len(r.Name) > 255 {
		return fmt.Errorf("artifact version or name is invalid")
	}
	if r.Kind == "" || len(r.Kind) > 64 || r.CreatedBy == "" || len(r.Content) == 0 || len(r.RawEvent) == 0 {
		return fmt.Errorf("artifact kind, creator, content, and event are required")
	}
	if len(r.SourceRefs) > 64 {
		return fmt.Errorf("too many artifact source references")
	}
	return nil
}
func decodeArtifactPayload(e v1.Envelope) (payloadArtifact, int, error) {
	var payload artifactPayload
	if err := json.Unmarshal(e.Payload, &payload); err != nil {
		return payloadArtifact{}, 0, err
	}
	if payload.Artifact.ID == "" {
		return payloadArtifact{}, 0, fmt.Errorf("artifact event has no artifact reference")
	}
	return payload.Artifact, payload.PreviousVersion, nil
}

type artifactPayload struct {
	Artifact        payloadArtifact `json:"artifact"`
	PreviousVersion int             `json:"previous_version"`
}
type payloadArtifact struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Kind         string            `json:"kind"`
	Checksum     string            `json:"checksum"`
	SizeBytes    int64             `json:"size_bytes"`
	Version      int               `json:"version"`
	TaskID       string            `json:"task_id"`
	CreatedBy    string            `json:"created_by"`
	CreatedAt    string            `json:"created_at"`
	SupersededBy string            `json:"superseded_by,omitempty"`
	Staleness    string            `json:"staleness,omitempty"`
	SourceRefs   []ArtifactVersion `json:"source_refs,omitempty"`
	Metadata     map[string]any    `json:"metadata,omitempty"`
}

func eventMatches(e payloadArtifact, a Artifact) bool {
	return e.ID == a.ID && e.Name == a.Name && e.Kind == a.Kind && e.Checksum == a.Checksum && e.SizeBytes == a.SizeBytes && e.Version == a.Version && e.TaskID == a.TaskID && e.CreatedBy == a.CreatedBy && e.CreatedAt == a.CreatedAt.Format(time.RFC3339Nano) && (e.Staleness == "" || e.Staleness == "fresh") && e.SupersededBy == "" && reflect.DeepEqual(e.SourceRefs, a.SourceRefs) && reflect.DeepEqual(e.Metadata, a.Metadata)
}
func sameArtifactIdentity(a, b Artifact) bool {
	return a.WorkspaceID == b.WorkspaceID && a.RunID == b.RunID && a.TaskID == b.TaskID && a.Name == b.Name && a.Kind == b.Kind && a.CreatedBy == b.CreatedBy && a.SizeBytes == b.SizeBytes
}
func decodeMetadata(raw string, a *Artifact) error {
	var metadata struct {
		SourceRefs []ArtifactVersion `json:"source_refs"`
		Metadata   map[string]any    `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return err
	}
	a.SourceRefs = metadata.SourceRefs
	a.Metadata = metadata.Metadata
	return nil
}
func (s *Store) resolve(relative string) (string, error) {
	if filepath.IsAbs(relative) || !filepath.IsLocal(filepath.FromSlash(relative)) {
		return "", fmt.Errorf("artifact path is not relative")
	}
	full := filepath.Join(s.root, filepath.FromSlash(relative))
	rel, err := filepath.Rel(s.root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("artifact path escapes storage root")
	}
	return full, nil
}
func writeAtomic(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".artifact-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	cleanup := func() { _ = file.Close(); _ = os.Remove(temp) }
	if _, err = file.Write(content); err != nil {
		cleanup()
		return err
	}
	if err = file.Sync(); err != nil {
		cleanup()
		return err
	}
	if err = file.Close(); err != nil {
		_ = os.Remove(temp)
		return err
	}
	if err = os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return nil
}
