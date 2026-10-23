package summarizer

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jeanedlune/Societas-AI/internal/artifacts"
	"github.com/Jeanedlune/Societas-AI/internal/provider"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

type fixedArtifact struct {
	artifact artifacts.Artifact
	content  []byte
	err      error
}

func (f fixedArtifact) Read(context.Context, string, string, int, string) (artifacts.Artifact, []byte, error) {
	return f.artifact, append([]byte(nil), f.content...), f.err
}

func TestSummaryBindsExactArtifactVersionAndPersists(t *testing.T) {
	ctx := context.Background()
	db, artifact := summaryDB(t)
	defer db.Close()
	reader := fixedArtifact{artifact: artifact, content: []byte("A sourced report about the company.")}
	llm := &provider.Mock{Name: "mock", Response: provider.Response{Content: `{"text":"The report says revenue grew.","findings":["Revenue grew"],"risks":["Period is not specified"],"recommendation":"Verify the period.","sources":["art_01J00000000000000000000000:v2"]}`, Usage: provider.Usage{OutputTokens: 25}}}
	manager, err := New(db, reader, llm, nil)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := manager.Create(ctx, "ws_demo", artifact.ID, artifact.Version, "agent:reviewer", "system:orchestrator", "mock/model", 200)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ArtifactID != artifact.ID || summary.ArtifactVersion != 2 || summary.TokenEstimate <= 0 || summary.Model != "mock/model" {
		t.Fatalf("summary=%#v", summary)
	}
	var gotVersion int
	var gotBody string
	if err := db.QueryRow(`SELECT artifact_version,body_json FROM summaries WHERE workspace_id=? AND summary_id=?`, summary.WorkspaceID, summary.ID).Scan(&gotVersion, &gotBody); err != nil {
		t.Fatal(err)
	}
	if gotVersion != 2 || gotBody == "" {
		t.Fatalf("persisted source version=%d body=%q", gotVersion, gotBody)
	}
}

func TestSummaryProviderFailureOrInvalidOutputDoesNotPersist(t *testing.T) {
	ctx := context.Background()
	db, artifact := summaryDB(t)
	defer db.Close()
	reader := fixedArtifact{artifact: artifact, content: []byte("source")}
	for _, test := range []struct {
		name string
		mock *provider.Mock
	}{
		{name: "provider failure", mock: &provider.Mock{Name: "mock", Err: errors.New("provider-secret-error")}},
		{name: "malformed body", mock: &provider.Mock{Name: "mock", Response: provider.Response{Content: `{"text":"looks okay","vendor_secret":"redacted"}`}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager, err := New(db, reader, test.mock, nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = manager.Create(ctx, "ws_demo", artifact.ID, artifact.Version, "agent:reviewer", "system:orchestrator", "mock/model", 100)
			if err == nil {
				t.Fatal("expected summary error")
			}
			var count int
			if err := db.QueryRow(`SELECT count(*) FROM summaries`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("persisted %d invalid summaries", count)
			}
		})
	}
}

func summaryDB(t *testing.T) (*sql.DB, artifacts.Artifact) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.Open(ctx, filepath.Join(t.TempDir(), "summary.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`INSERT INTO workspaces(workspace_id,name) VALUES('ws_demo','Demo')`, `INSERT INTO agents(workspace_id,agent_id,display_name,role,status) VALUES('ws_demo','researcher','Researcher','research','idle')`, `INSERT INTO agents(workspace_id,agent_id,display_name,role,status) VALUES('ws_demo','reviewer','Reviewer','review','idle')`, `INSERT INTO runs(workspace_id,run_id,goal) VALUES('ws_demo','RUN-001','goal')`, `INSERT INTO tasks(workspace_id,run_id,task_id,title,status) VALUES('ws_demo','RUN-001','TASK-001','task','completed')`} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	created := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO artifacts(workspace_id,artifact_id,run_id,version,task_id,name,kind,relative_path,checksum,size_bytes,created_by,created_at) VALUES('ws_demo','art_01J00000000000000000000000','RUN-001',2,'TASK-001','report.md','markdown','artifacts/report-v2.md','sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',35,'agent:researcher',?)`, created); err != nil {
		t.Fatal(err)
	}
	return db, artifacts.Artifact{ID: "art_01J00000000000000000000000", WorkspaceID: "ws_demo", RunID: "RUN-001", Version: 2, TaskID: "TASK-001", Name: "report.md", Kind: "markdown", Checksum: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SizeBytes: 35, CreatedBy: "agent:researcher", CreatedAt: time.Now().UTC(), Staleness: "fresh"}
}
