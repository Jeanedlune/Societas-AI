package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

type fakeVector struct {
	entries map[string]Entry
	hits    []Hit
	writes  int
}

func (f *fakeVector) Upsert(_ context.Context, e Entry) error {
	if f.entries == nil {
		f.entries = map[string]Entry{}
	}
	f.entries[e.ID] = e
	f.writes++
	return nil
}
func (f *fakeVector) Search(_ context.Context, _ SearchRequest) ([]Hit, error) {
	return append([]Hit(nil), f.hits...), nil
}
func (f *fakeVector) Delete(_ context.Context, workspace, id string) error {
	delete(f.entries, id)
	return nil
}

type allowAll struct{}

func (allowAll) Allow(context.Context, Entry, string) bool { return true }

func TestMemoryAdmissionScopeIsolationAndStaleness(t *testing.T) {
	ctx := context.Background()
	db := memoryDB(t)
	defer db.Close()
	vector := &fakeVector{}
	store, err := New(db, vector, allowAll{})
	if err != nil {
		t.Fatal(err)
	}
	request := CreateRequest{WorkspaceID: "ws_demo", Scope: ScopeWorkspace, OwnerID: "ws_demo", Source: ArtifactRef{ID: "art_01J00000000000000000000000", Version: 1, Checksum: "sha256:" + repeatA(64)}, Content: json.RawMessage(`{"text":"approved brand voice"}`), Confidence: .9, Tag: "best_practice"}
	candidate, err := store.CreateCandidate(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.AdmissionStatus != "candidate" || vector.writes != 0 {
		t.Fatalf("candidate=%#v vector writes=%d", candidate, vector.writes)
	}
	admitted, err := store.Admit(ctx, "ws_demo", candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if admitted.AdmissionStatus != "admitted" || vector.writes != 1 {
		t.Fatalf("admitted=%#v vector writes=%d", admitted, vector.writes)
	}
	vector.hits = []Hit{{MemoryID: candidate.ID, Score: .99}}
	if got, err := store.Search(ctx, SearchRequest{WorkspaceID: "ws_demo", Scope: ScopeAgent, OwnerID: "researcher", Query: "brand voice", Limit: 10}); err != nil || len(got) != 0 {
		t.Fatalf("foreign scope hit=%#v err=%v", got, err)
	}
	got, err := store.Search(ctx, SearchRequest{WorkspaceID: "ws_demo", Scope: ScopeWorkspace, OwnerID: "ws_demo", Query: "brand voice", Limit: 10})
	if err != nil || len(got) != 1 || got[0].ID != candidate.ID {
		t.Fatalf("valid search=%#v err=%v", got, err)
	}
	if err := store.MarkSourceStale(ctx, "ws_demo", candidate.Source.ID, candidate.Source.Version); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Search(ctx, SearchRequest{WorkspaceID: "ws_demo", Scope: ScopeWorkspace, OwnerID: "ws_demo", Query: "brand voice", Limit: 10}); err != nil || len(got) != 0 {
		t.Fatalf("stale memory search=%#v err=%v", got, err)
	}
}

func TestMemoryAdmissionRequiresCompletedSuccessfulSource(t *testing.T) {
	ctx := context.Background()
	db := memoryDB(t)
	defer db.Close()
	vector := &fakeVector{}
	store, err := New(db, vector, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE tasks SET status='running',data_json='{}' WHERE workspace_id='ws_demo' AND run_id='RUN-001' AND task_id='TASK-001'`); err != nil {
		t.Fatal(err)
	}
	candidate, err := store.CreateCandidate(ctx, CreateRequest{WorkspaceID: "ws_demo", Scope: ScopeTask, OwnerID: "TASK-001", Source: ArtifactRef{ID: "art_01J00000000000000000000000", Version: 1, Checksum: "sha256:" + repeatA(64)}, Content: json.RawMessage(`{"text":"candidate"}`), Confidence: .5, Tag: "research_finding"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Admit(ctx, "ws_demo", candidate.ID); err != ErrMemoryRejected {
		t.Fatalf("admit incomplete task error=%v", err)
	}
	if vector.writes != 0 {
		t.Fatal("unvalidated memory reached vector store")
	}
	if _, err := store.CreateCandidate(ctx, CreateRequest{WorkspaceID: "ws_demo", Scope: ScopeAgent, OwnerID: "researcher", Source: candidate.Source, Content: json.RawMessage(`{"text":"sensitive"}`), Confidence: .8, Sensitivity: "sensitive", Tag: "best_practice"}); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT memory_id FROM memories WHERE sensitivity='sensitive'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Admit(ctx, "ws_demo", id); err != ErrMemoryRejected {
			t.Fatalf("sensitive admit error=%v", err)
		}
	}
}

func memoryDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	db, err := storage.Open(ctx, filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`INSERT INTO workspaces(workspace_id,name) VALUES('ws_demo','Demo')`, `INSERT INTO agents(workspace_id,agent_id,display_name,role,status) VALUES('ws_demo','researcher','Researcher','research','idle')`, `INSERT INTO runs(workspace_id,run_id,goal) VALUES('ws_demo','RUN-001','goal')`, `INSERT INTO tasks(workspace_id,run_id,task_id,title,status,data_json) VALUES('ws_demo','RUN-001','TASK-001','task','completed','{"result_status":"success"}')`, `INSERT INTO artifacts(workspace_id,artifact_id,run_id,version,task_id,name,kind,relative_path,checksum,size_bytes,created_by,created_at) VALUES('ws_demo','art_01J00000000000000000000000','RUN-001',1,'TASK-001','source.md','markdown','artifacts/source.md','sha256:` + repeatA(64) + `',20,'agent:researcher',?)`} {
		if strings.Contains(q, `VALUES('ws_demo','art_01J00000000000000000000000'`) {
			if _, err := db.Exec(q, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
				t.Fatal(err)
			}
		} else if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return db
}
func repeatA(n int) string { return strings.Repeat("a", n) }
