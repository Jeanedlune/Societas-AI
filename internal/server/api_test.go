package server

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

func TestWorkspaceAPIListsRedactedReadOnlyRecords(t *testing.T) {
	ctx := context.Background()
	db := apiDB(t, ctx)
	defer db.Close()
	seedAPI(t, db)
	handler := NewAPIHandler(APIOptions{DB: db, Version: "test"})
	for _, path := range []string{"/api/v1/workspaces", "/api/v1/workspaces/ws_demo", "/api/v1/workspaces/ws_demo/agents", "/api/v1/workspaces/ws_demo/runs"} {
		req := httptest.NewRequest("GET", path, nil)
		req.RemoteAddr = "127.0.0.1:1234"
		req.Header.Set("X-Request-ID", "req_test-1")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != 200 {
			t.Fatalf("GET %s status=%d body=%s", path, res.Code, res.Body.String())
		}
		if res.Header().Get("X-Request-ID") != "req_test-1" || !strings.Contains(res.Body.String(), `"api_version":"v1"`) {
			t.Fatalf("missing versioned/request ID response: %s", res.Body.String())
		}
		if strings.Contains(res.Body.String(), "secret-value") {
			t.Fatalf("response leaked config secret: %s", res.Body.String())
		}
	}
	before := rowCounts(t, db)
	req := httptest.NewRequest("GET", "/api/v1/workspaces/ws_demo/agents", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if after := rowCounts(t, db); before != after {
		t.Fatalf("read endpoint created side effects: before=%v after=%v", before, after)
	}
}

func TestWorkspaceAPIUnauthorizedMissingAndUnhealthy(t *testing.T) {
	ctx := context.Background()
	db := apiDB(t, ctx)
	seedAPI(t, db)
	handler := NewAPIHandler(APIOptions{DB: db, APIToken: "local-token"})
	req := httptest.NewRequest("GET", "/api/v1/workspaces", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != 401 || !strings.Contains(res.Body.String(), `"code":"UNAUTHORIZED"`) {
		t.Fatalf("unauthorized response status=%d body=%s", res.Code, res.Body.String())
	}
	req = httptest.NewRequest("GET", "/api/v1/workspaces/missing", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Authorization", "Bearer local-token")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != 404 || !strings.Contains(res.Body.String(), `"code":"NOT_FOUND"`) {
		t.Fatalf("missing response status=%d body=%s", res.Code, res.Body.String())
	}
	ready := httptest.NewRequest("GET", "/ready", nil)
	readyRes := httptest.NewRecorder()
	handler.ServeHTTP(readyRes, ready)
	if readyRes.Code != 200 {
		t.Fatalf("ready status=%d body=%s", readyRes.Code, readyRes.Body.String())
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	readyRes = httptest.NewRecorder()
	handler.ServeHTTP(readyRes, ready)
	if readyRes.Code != 503 {
		t.Fatalf("unhealthy readiness status=%d body=%s", readyRes.Code, readyRes.Body.String())
	}
}

func TestWorkspaceAPIRejectsRemoteClientWithoutToken(t *testing.T) {
	handler := NewAPIHandler(APIOptions{})
	req := httptest.NewRequest("GET", "/api/v1/workspaces", nil)
	req.RemoteAddr = "198.51.100.4:1234"
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != 401 {
		t.Fatalf("remote status=%d body=%s", res.Code, res.Body.String())
	}
}

func apiDB(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()
	db, err := storage.Open(ctx, filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	return db
}
func seedAPI(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{`INSERT INTO workspaces(workspace_id,name) VALUES('ws_demo','Demo')`, `INSERT INTO agents(workspace_id,agent_id,display_name,role,config_json) VALUES('ws_demo','researcher','Researcher','research','{"api_key":"secret-value"}')`, `INSERT INTO runs(workspace_id,run_id,goal,entry_agent_id) VALUES('ws_demo','RUN-001','test','researcher')`} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}
func rowCounts(t *testing.T, db *sql.DB) [3]int {
	t.Helper()
	var out [3]int
	for i, q := range []string{"SELECT count(*) FROM workspaces", "SELECT count(*) FROM agents", "SELECT count(*) FROM runs"} {
		if err := db.QueryRow(q).Scan(&out[i]); err != nil {
			t.Fatal(err)
		}
	}
	return out
}
