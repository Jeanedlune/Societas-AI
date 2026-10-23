package server

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
)

var requestIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

type APIOptions struct {
	DB         *sql.DB
	Version    string
	APIToken   string
	InstanceID string
}

type apiResponse struct {
	APIVersion string    `json:"api_version"`
	Data       any       `json:"data"`
	Error      *apiError `json:"error"`
	RequestID  string    `json:"request_id"`
}
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type workspaceRecord struct {
	ID        string `json:"workspace_id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
type agentRecord struct {
	ID          string `json:"agent_id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}
type runRecord struct {
	ID           string  `json:"run_id"`
	Status       string  `json:"status"`
	EntryAgentID *string `json:"entry_agent_id,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

// NewAPIHandler exposes read-only versioned workspace endpoints. The health
// endpoint is liveness only; readiness also checks the operational database.
func NewAPIHandler(options APIOptions) http.Handler {
	if options.Version == "" {
		options.Version = "dev"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(struct {
			Status  string `json:"status"`
			Version string `json:"version"`
		}{"ok", options.Version})
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		withRequestID(w, r, func(id string) {
			if options.InstanceID != "" {
				w.Header().Set("X-Societas-Instance-ID", options.InstanceID)
			}
			if r.Method != http.MethodGet {
				writeAPIError(w, http.StatusMethodNotAllowed, id, "METHOD_NOT_ALLOWED", "only GET is supported")
				return
			}
			if options.DB == nil {
				writeAPIError(w, http.StatusServiceUnavailable, id, "NOT_READY", "operational database is unavailable")
				return
			}
			if err := options.DB.PingContext(r.Context()); err != nil {
				writeAPIError(w, http.StatusServiceUnavailable, id, "DEPENDENCY_UNAVAILABLE", "operational database is unavailable")
				return
			}
			writeAPI(w, http.StatusOK, id, map[string]any{"status": "ready", "version": options.Version})
		})
	})
	mux.Handle("/api/v1/workspaces", apiAuth(options.APIToken, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		withRequestID(w, r, func(id string) { handleWorkspaces(w, r, id, options.DB) })
	})))
	mux.Handle("/api/v1/workspaces/", apiAuth(options.APIToken, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		withRequestID(w, r, func(id string) { handleWorkspacePath(w, r, id, options.DB) })
	})))
	return mux
}

func handleWorkspaces(w http.ResponseWriter, r *http.Request, id string, db *sql.DB) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, id, "METHOD_NOT_ALLOWED", "only GET is supported")
		return
	}
	if db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, id, "NOT_READY", "operational database is unavailable")
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT workspace_id,name,created_at,updated_at FROM workspaces ORDER BY workspace_id`)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, id, "DEPENDENCY_UNAVAILABLE", "workspace storage is unavailable")
		return
	}
	defer rows.Close()
	items := make([]workspaceRecord, 0)
	for rows.Next() {
		var item workspaceRecord
		if err := rows.Scan(&item.ID, &item.Name, &item.CreatedAt, &item.UpdatedAt); err != nil {
			writeAPIError(w, http.StatusInternalServerError, id, "INTERNAL_ERROR", "could not read workspace")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, id, "DEPENDENCY_UNAVAILABLE", "workspace storage is unavailable")
		return
	}
	writeAPI(w, http.StatusOK, id, items)
}

func handleWorkspacePath(w http.ResponseWriter, r *http.Request, id string, db *sql.DB) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, id, "METHOD_NOT_ALLOWED", "only GET is supported")
		return
	}
	if db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, id, "NOT_READY", "operational database is unavailable")
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/workspaces/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] != "agents" && parts[1] != "runs") {
		writeAPIError(w, http.StatusNotFound, id, "NOT_FOUND", "resource was not found")
		return
	}
	workspaceID := parts[0]
	var exists int
	if err := db.QueryRowContext(r.Context(), `SELECT 1 FROM workspaces WHERE workspace_id=?`, workspaceID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, id, "NOT_FOUND", "workspace was not found")
		} else {
			writeAPIError(w, http.StatusServiceUnavailable, id, "DEPENDENCY_UNAVAILABLE", "workspace storage is unavailable")
		}
		return
	}
	if len(parts) == 1 {
		var item workspaceRecord
		err := db.QueryRowContext(r.Context(), `SELECT workspace_id,name,created_at,updated_at FROM workspaces WHERE workspace_id=?`, workspaceID).Scan(&item.ID, &item.Name, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, id, "DEPENDENCY_UNAVAILABLE", "workspace storage is unavailable")
			return
		}
		writeAPI(w, http.StatusOK, id, item)
		return
	}
	if parts[1] == "agents" {
		rows, err := db.QueryContext(r.Context(), `SELECT agent_id,display_name,role,status FROM agents WHERE workspace_id=? ORDER BY agent_id`, workspaceID)
		if err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, id, "DEPENDENCY_UNAVAILABLE", "agent storage is unavailable")
			return
		}
		defer rows.Close()
		items := make([]agentRecord, 0)
		for rows.Next() {
			var a agentRecord
			if err := rows.Scan(&a.ID, &a.DisplayName, &a.Role, &a.Status); err != nil {
				writeAPIError(w, http.StatusInternalServerError, id, "INTERNAL_ERROR", "could not read agents")
				return
			}
			items = append(items, a)
		}
		if err := rows.Err(); err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, id, "DEPENDENCY_UNAVAILABLE", "agent storage is unavailable")
			return
		}
		writeAPI(w, http.StatusOK, id, items)
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT run_id,status,entry_agent_id,created_at FROM runs WHERE workspace_id=? ORDER BY created_at,run_id`, workspaceID)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, id, "DEPENDENCY_UNAVAILABLE", "run storage is unavailable")
		return
	}
	defer rows.Close()
	items := make([]runRecord, 0)
	for rows.Next() {
		var run runRecord
		if err := rows.Scan(&run.ID, &run.Status, &run.EntryAgentID, &run.CreatedAt); err != nil {
			writeAPIError(w, http.StatusInternalServerError, id, "INTERNAL_ERROR", "could not read runs")
			return
		}
		items = append(items, run)
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, id, "DEPENDENCY_UNAVAILABLE", "run storage is unavailable")
		return
	}
	writeAPI(w, http.StatusOK, id, items)
}

func apiAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		withRequestID(w, r, func(id string) {
			if token != "" {
				supplied := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
				if subtle.ConstantTimeCompare([]byte(supplied), []byte(token)) != 1 {
					writeAPIError(w, http.StatusUnauthorized, id, "UNAUTHORIZED", "valid bearer token required")
					return
				}
			} else {
				host, _, err := net.SplitHostPort(r.RemoteAddr)
				ip := net.ParseIP(host)
				if err != nil || ip == nil || !ip.IsLoopback() {
					writeAPIError(w, http.StatusUnauthorized, id, "UNAUTHORIZED", "API is restricted to loopback clients")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	})
}

func withRequestID(w http.ResponseWriter, r *http.Request, fn func(string)) {
	id := r.Header.Get("X-Request-ID")
	if !requestIDPattern.MatchString(id) {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			id = "request-unavailable"
		} else {
			id = hex.EncodeToString(random[:])
		}
	}
	w.Header().Set("X-Request-ID", id)
	fn(id)
}
func writeAPI(w http.ResponseWriter, status int, id string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{APIVersion: "v1", Data: data, RequestID: id})
}
func writeAPIError(w http.ResponseWriter, status int, id, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{APIVersion: "v1", Error: &apiError{Code: code, Message: message}, RequestID: id})
}
