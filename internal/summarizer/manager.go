// Package summarizer creates schema-bound, provenance-linked summaries.
package summarizer

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Jeanedlune/Societas-AI/internal/artifacts"
	"github.com/Jeanedlune/Societas-AI/internal/contextmgr"
	"github.com/Jeanedlune/Societas-AI/internal/ids"
	"github.com/Jeanedlune/Societas-AI/internal/provider"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

var ErrInvalidSummary = errors.New("provider returned an invalid summary")

type Body struct {
	Text           string   `json:"text"`
	Findings       []string `json:"findings,omitempty"`
	Risks          []string `json:"risks,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
	Sources        []string `json:"sources,omitempty"`
}
type Summary struct {
	ID              string    `json:"id"`
	WorkspaceID     string    `json:"workspace_id"`
	ArtifactID      string    `json:"artifact_id"`
	ArtifactVersion int       `json:"artifact_version"`
	TokenEstimate   int       `json:"token_estimate"`
	GeneratedBy     string    `json:"generated_by"`
	Model           string    `json:"model"`
	Body            Body      `json:"body"`
	CreatedAt       time.Time `json:"created_at"`
}
type ArtifactReader interface {
	Read(context.Context, string, string, int, string) (artifacts.Artifact, []byte, error)
}
type Manager struct {
	db        *sql.DB
	artifacts ArtifactReader
	provider  provider.Provider
	counter   contextmgr.TokenCounter
}

func New(db *sql.DB, artifactReader ArtifactReader, modelProvider provider.Provider, counter contextmgr.TokenCounter) (*Manager, error) {
	if db == nil || artifactReader == nil || modelProvider == nil {
		return nil, fmt.Errorf("summary manager requires database, artifact reader, and provider")
	}
	if counter == nil {
		counter = contextmgr.ConservativeCounter{}
	}
	return &Manager{db: db, artifacts: artifactReader, provider: modelProvider, counter: counter}, nil
}

// Create summarizes an exact artifact version. The provider must return the
// canonical summary body as JSON; malformed output never enters persistence.
func (m *Manager) Create(ctx context.Context, workspaceID, artifactID string, version int, requester, generatedBy, model string, maxOutputTokens int64) (Summary, error) {
	if workspaceID == "" || artifactID == "" || version < 1 || requester == "" || generatedBy == "" || strings.TrimSpace(model) == "" || maxOutputTokens < 1 {
		return Summary{}, fmt.Errorf("summary request identity and output budget are required")
	}
	artifact, content, err := m.artifacts.Read(ctx, workspaceID, artifactID, version, requester)
	if err != nil {
		return Summary{}, err
	}
	request := provider.Request{Model: model, MaxOutputTokens: maxOutputTokens, Temperature: 0, JSONMode: true, Messages: []provider.Message{{Role: "system", Content: "Summarize the supplied source. Return only a JSON object with fields text, findings, risks, recommendation, and sources. Preserve uncertainty and do not claim approval or verification."}, {Role: "user", Content: string(content)}}}
	response, err := m.provider.Generate(ctx, request)
	if err != nil {
		return Summary{}, provider.MapError(err)
	}
	var body Body
	decoder := json.NewDecoder(bytes.NewBufferString(response.Content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return Summary{}, ErrInvalidSummary
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Summary{}, ErrInvalidSummary
	}
	if strings.TrimSpace(body.Text) == "" || len([]rune(body.Text)) > 8000 {
		return Summary{}, ErrInvalidSummary
	}
	encodedBody, err := json.Marshal(body)
	if err != nil {
		return Summary{}, err
	}
	tokenEstimate, err := m.counter.Estimate(string(encodedBody))
	if err != nil {
		return Summary{}, err
	}
	if int64(tokenEstimate) > maxOutputTokens {
		return Summary{}, ErrInvalidSummary
	}
	if response.Usage.OutputTokens > maxOutputTokens {
		return Summary{}, ErrInvalidSummary
	}
	id, err := ids.New("sum_")
	if err != nil {
		return Summary{}, err
	}
	created := time.Now().UTC()
	summary := Summary{ID: id, WorkspaceID: workspaceID, ArtifactID: artifact.ID, ArtifactVersion: artifact.Version, TokenEstimate: tokenEstimate, GeneratedBy: generatedBy, Model: model, Body: body, CreatedAt: created}
	if err := storage.WithTx(ctx, m.db, nil, func(tx *sql.Tx) error {
		var checksum, staleness string
		var superseded sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT checksum,staleness,superseded_by FROM artifacts WHERE workspace_id=? AND artifact_id=? AND version=?`, workspaceID, artifactID, version).Scan(&checksum, &staleness, &superseded)
		if err != nil {
			return err
		}
		if checksum != artifact.Checksum || staleness != "fresh" || superseded.Valid {
			return fmt.Errorf("summary source artifact changed during generation")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO summaries(workspace_id,summary_id,artifact_id,artifact_version,token_estimate,generated_by,model,body_json,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, workspaceID, id, artifactID, version, tokenEstimate, generatedBy, model, string(encodedBody), created.Format(time.RFC3339Nano))
		return err
	}); err != nil {
		return Summary{}, fmt.Errorf("persist summary: %w", err)
	}
	return summary, nil
}
