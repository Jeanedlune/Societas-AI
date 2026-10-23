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
	"strings"
	"testing"
	"time"

	v1 "github.com/Jeanedlune/Societas-AI/internal/contracts/v1"
	"github.com/Jeanedlune/Societas-AI/internal/eventstore"
	"github.com/Jeanedlune/Societas-AI/internal/ids"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

func TestArtifactVersionsReadPermissionsAndIntegrity(t *testing.T) {
	ctx := context.Background()
	db, events := artifactDB(t)
	defer db.Close()
	root := filepath.Join(t.TempDir(), "objects")
	store, err := New(db, events, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("first version")
	request := artifactRequest(t, 1, content, "draft.md", "")
	created, duplicate, err := store.Write(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate || created.Version != 1 {
		t.Fatalf("created=%#v duplicate=%v", created, duplicate)
	}
	if strings.Contains(fmt.Sprintf("%+v", created), root) {
		t.Fatal("physical storage path leaked in artifact result")
	}
	if _, _, err := store.Read(ctx, "ws_demo", request.ID, 1, "agent:reviewer"); !errors.Is(err, ErrArtifactDenied) {
		t.Fatalf("unauthorized read error=%v", err)
	}
	read, body, err := store.Read(ctx, "ws_demo", request.ID, 1, "agent:researcher")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != string(content) || read.Checksum != created.Checksum {
		t.Fatalf("read=%#v body=%q", read, body)
	}
	if _, duplicate, err = store.Write(ctx, request); err != nil || !duplicate {
		t.Fatalf("idempotent write duplicate=%v err=%v", duplicate, err)
	}
	update := artifactRequest(t, 2, []byte("second version"), "draft.md", request.ID)
	second, duplicate, err := store.Write(ctx, update)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate || second.Version != 2 {
		t.Fatalf("updated=%#v duplicate=%v", second, duplicate)
	}
	first, _, err := store.Read(ctx, "ws_demo", request.ID, 1, "agent:researcher")
	if err != nil {
		t.Fatal(err)
	}
	if first.Staleness != "stale" || first.SupersededBy != request.ID {
		t.Fatalf("old version was not tombstoned: %#v", first)
	}
	var relative string
	if err := db.QueryRow(`SELECT relative_path FROM artifacts WHERE workspace_id=? AND artifact_id=? AND version=2`, "ws_demo", request.ID).Scan(&relative); err != nil {
		t.Fatal(err)
	}
	physical := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.WriteFile(physical, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Read(ctx, "ws_demo", request.ID, 2, "agent:researcher"); !errors.Is(err, ErrArtifactIntegrity) {
		t.Fatalf("tampered read error=%v", err)
	}
}

func TestArtifactWriteRejectsTraversalAndEventMismatch(t *testing.T) {
	ctx := context.Background()
	db, events := artifactDB(t)
	defer db.Close()
	store, err := New(db, events, filepath.Join(t.TempDir(), "objects"), nil)
	if err != nil {
		t.Fatal(err)
	}
	request := artifactRequest(t, 1, []byte("content"), "../escape.md", "")
	if _, _, err := store.Write(ctx, request); err == nil {
		t.Fatal("path traversal filename accepted")
	}
	request = artifactRequest(t, 1, []byte("content"), "draft.md", "")
	request.Content = []byte("different")
	if _, _, err := store.Write(ctx, request); err == nil || !strings.Contains(err.Error(), "payload does not match") {
		t.Fatalf("mismatched event error=%v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(store.root), "escape.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected escaped file: %v", err)
	}
}

func artifactDB(t *testing.T) (*sql.DB, *eventstore.Store) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.Open(ctx, filepath.Join(t.TempDir(), "artifacts.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{`INSERT INTO workspaces(workspace_id,name) VALUES('ws_demo','Demo')`, `INSERT INTO agents(workspace_id,agent_id,display_name,role,status) VALUES('ws_demo','researcher','Researcher','research','idle')`, `INSERT INTO agents(workspace_id,agent_id,display_name,role,status) VALUES('ws_demo','reviewer','Reviewer','review','idle')`, `INSERT INTO runs(workspace_id,run_id,goal) VALUES('ws_demo','RUN-001','goal')`, `INSERT INTO tasks(workspace_id,run_id,task_id,title,status) VALUES('ws_demo','RUN-001','TASK-001','task','running')`} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	validator, err := v1.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	events, err := eventstore.New(db, validator)
	if err != nil {
		t.Fatal(err)
	}
	return db, events
}

func artifactRequest(t *testing.T, version int, content []byte, name, artifactID string) WriteRequest {
	t.Helper()
	if artifactID == "" {
		var err error
		artifactID, err = ids.New("art_")
		if err != nil {
			t.Fatal(err)
		}
	}
	createdAt := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	hash := sha256.Sum256(content)
	checksum := "sha256:" + hex.EncodeToString(hash[:])
	type artifactRef struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Kind      string `json:"kind"`
		Checksum  string `json:"checksum"`
		SizeBytes int64  `json:"size_bytes"`
		Version   int    `json:"version"`
		TaskID    string `json:"task_id"`
		CreatedBy string `json:"created_by"`
		CreatedAt string `json:"created_at"`
	}
	ref := artifactRef{artifactID, name, "markdown", checksum, int64(len(content)), version, "TASK-001", "agent:researcher", createdAt}
	typ := v1.EventArtifactCreated
	payload := any(map[string]any{"artifact": ref})
	if version > 1 {
		typ = v1.EventArtifactUpdated
		payload = map[string]any{"artifact": ref, "previous_version": version - 1}
	}
	eventID, _ := ids.New("evt_")
	correlation, _ := ids.New("cor_")
	agentPrincipal, _ := ids.New("agt_")
	session, _ := ids.New("ses_")
	envelope := v1.Envelope{ProtocolVersion: v1.ProtocolVersion, SchemaVersion: v1.SchemaVersion, ID: v1.EventID(eventID), Type: typ, Timestamp: createdAt, WorkspaceID: "ws_demo", RunID: "RUN-001", TaskID: func() *v1.TaskID { id := v1.TaskID("TASK-001"); return &id }(), CorrelationID: v1.CorrelationID(correlation), IdempotencyKey: fmt.Sprintf("artifact-%s-%d", artifactID, version), From: "agent:researcher", To: "topic:all", Payload: mustJSON(t, payload), Principal: v1.Principal{PrincipalID: v1.PrincipalID(agentPrincipal), PrincipalType: v1.PrincipalAgent, SessionID: v1.SessionID(session), AuthDigest: "hmac_sha256:" + strings.Repeat("a", 64)}}
	return WriteRequest{ID: artifactID, WorkspaceID: "ws_demo", RunID: "RUN-001", TaskID: "TASK-001", Version: version, Name: name, Kind: "markdown", CreatedBy: "agent:researcher", Content: content, RawEvent: mustJSON(t, envelope)}
}
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
