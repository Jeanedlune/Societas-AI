// Package cli implements the local workspace lifecycle commands.
package cli

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Jeanedlune/Societas-AI/internal/agent"
	"github.com/Jeanedlune/Societas-AI/internal/config"
	"github.com/Jeanedlune/Societas-AI/internal/server"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

type lifecycleFlags struct {
	configPath   string
	databasePath string
}
type processRecord struct {
	PID        int    `json:"pid"`
	InstanceID string `json:"instance_id"`
}

func Execute(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usage(stderr)
	}
	switch args[0] {
	case "init":
		return initWorkspace(args[1:], stdout, stderr)
	case "serve":
		return serve(ctx, args[1:], stdout, stderr)
	case "start":
		return start(args[1:], stdout, stderr)
	case "status":
		return status(args[1:], stdout, stderr)
	case "stop":
		return stop(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		return usage(stdout)
	default:
		return fmt.Errorf("unknown command %q; run societas help", args[0])
	}
}

func initWorkspace(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("config", "workspace.yaml", "workspace configuration path")
	name := flags.String("name", "general-content-workspace", "workspace display name")
	force := flags.Bool("force", false, "replace an existing configuration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("init does not accept positional arguments")
	}
	if _, err := os.Stat(*path); err == nil && !*force {
		return fmt.Errorf("config %q already exists (use --force to replace)", *path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check config path: %w", err)
	}
	data, err := config.DefaultWorkspaceYAML(*name)
	if err != nil {
		return fmt.Errorf("create default workspace config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(*path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(*path, data, 0o600); err != nil {
		return fmt.Errorf("write workspace config: %w", err)
	}
	_, err = fmt.Fprintf(stdout, "created %s\n", *path)
	return err
}

func serve(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	parsed, err := parseLifecycle("serve", args, stderr)
	if err != nil {
		return err
	}
	cfg, err := config.Load(parsed.configPath)
	if err != nil {
		return err
	}
	registry, err := configuredAgents(cfg)
	if err != nil {
		return err
	}
	db, err := storage.Open(ctx, parsed.databasePath)
	if err != nil {
		return fmt.Errorf("open operational database: %w", err)
	}
	defer db.Close()
	if err := syncWorkspace(ctx, db, cfg, registry); err != nil {
		return err
	}
	addr := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
	if override := os.Getenv("SOCIETAS_ADDR"); override != "" {
		addr = override
	}
	return server.RunWithOptions(ctx, addr, stdout, server.APIOptions{DB: db, Version: "dev", APIToken: os.Getenv("SOCIETAS_API_TOKEN"), InstanceID: os.Getenv("SOCIETAS_INSTANCE_ID")})
}

func start(args []string, stdout, stderr io.Writer) error {
	parsed, err := parseLifecycle("start", args, stderr)
	if err != nil {
		return err
	}
	cfg, err := config.Load(parsed.configPath)
	if err != nil {
		return err
	}
	addr := workspaceAddress(cfg)
	if override := os.Getenv("SOCIETAS_ADDR"); override != "" {
		addr = override
	}
	pidPath := parsed.databasePath + ".pid"
	if readyInstance(addr, "") != "" {
		return fmt.Errorf("Societas is already serving at %s", addr)
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate Societas executable: %w", err)
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Errorf("create process identity: %w", err)
	}
	instanceID := hex.EncodeToString(random[:])
	if err := os.MkdirAll(filepath.Dir(parsed.databasePath), 0o700); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	logFile, err := os.OpenFile(parsed.databasePath+".log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open service log: %w", err)
	}
	command := exec.Command(executable, "serve", "--config", parsed.configPath, "--db", parsed.databasePath)
	command.Env = replaceEnv(os.Environ(), "SOCIETAS_INSTANCE_ID", instanceID)
	command.Stdout = logFile
	command.Stderr = logFile
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("start Societas service: %w", err)
	}
	_ = logFile.Close()
	encoded, _ := json.Marshal(processRecord{PID: command.Process.Pid, InstanceID: instanceID})
	if err := os.WriteFile(pidPath, encoded, 0o600); err != nil {
		_ = command.Process.Signal(os.Interrupt)
		return fmt.Errorf("write service process record: %w", err)
	}
	if !waitReady(addr, instanceID, 5*time.Second) {
		_ = command.Process.Signal(os.Interrupt)
		_ = os.Remove(pidPath)
		return fmt.Errorf("service failed to become ready; inspect %s.log", parsed.databasePath)
	}
	_, err = fmt.Fprintf(stdout, "started pid %d at %s\n", command.Process.Pid, addr)
	return err
}

func status(args []string, stdout, stderr io.Writer) error {
	parsed, err := parseLifecycle("status", args, stderr)
	if err != nil {
		return err
	}
	cfg, err := config.Load(parsed.configPath)
	if err != nil {
		return err
	}
	addr := workspaceAddress(cfg)
	if override := os.Getenv("SOCIETAS_ADDR"); override != "" {
		addr = override
	}
	response, err := getReady(addr)
	if err != nil {
		return fmt.Errorf("Societas is not ready at %s: %w", addr, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Societas is not ready at %s (HTTP %d)", addr, response.StatusCode)
	}
	_, err = fmt.Fprintf(stdout, "ready at %s\n", addr)
	return err
}

func stop(args []string, stdout, stderr io.Writer) error {
	parsed, err := parseLifecycle("stop", args, stderr)
	if err != nil {
		return err
	}
	cfg, err := config.Load(parsed.configPath)
	if err != nil {
		return err
	}
	addr := workspaceAddress(cfg)
	if override := os.Getenv("SOCIETAS_ADDR"); override != "" {
		addr = override
	}
	pidPath := parsed.databasePath + ".pid"
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return fmt.Errorf("read service process record: %w", err)
	}
	var record processRecord
	if err := json.Unmarshal(data, &record); err != nil || record.PID < 2 || record.InstanceID == "" {
		return fmt.Errorf("invalid service process record %q", pidPath)
	}
	if readyInstance(addr, record.InstanceID) == "" {
		_ = os.Remove(pidPath)
		return fmt.Errorf("running service identity did not match; removed stale process record")
	}
	process, err := os.FindProcess(record.PID)
	if err != nil {
		return fmt.Errorf("find Societas process: %w", err)
	}
	if err := process.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("signal Societas process: %w", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := getReady(addr); err != nil {
			_ = os.Remove(pidPath)
			_, writeErr := fmt.Fprintf(stdout, "stopped pid %d\n", record.PID)
			return writeErr
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("Societas did not stop within 5 seconds")
}

func parseLifecycle(name string, args []string, stderr io.Writer) (lifecycleFlags, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "workspace.yaml", "workspace configuration path")
	databasePath := flags.String("db", "", "operational database path")
	if err := flags.Parse(args); err != nil {
		return lifecycleFlags{}, err
	}
	if flags.NArg() != 0 {
		return lifecycleFlags{}, fmt.Errorf("%s does not accept positional arguments", name)
	}
	path := *databasePath
	if path == "" {
		absolute, err := filepath.Abs(*configPath)
		if err != nil {
			return lifecycleFlags{}, err
		}
		path = filepath.Join(filepath.Dir(absolute), "societas.db")
	}
	absoluteDB, err := filepath.Abs(path)
	if err != nil {
		return lifecycleFlags{}, fmt.Errorf("resolve database path: %w", err)
	}
	path = absoluteDB
	return lifecycleFlags{configPath: *configPath, databasePath: path}, nil
}

func configuredAgents(cfg *config.Config) ([]string, error) {
	registry, err := agent.NewRegistry(cfg)
	if err != nil {
		return nil, err
	}
	return registry.IDs(), nil
}

func syncWorkspace(ctx context.Context, db *sql.DB, cfg *config.Config, agentIDs []string) error {
	return storage.WithTx(ctx, db, nil, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workspaces(workspace_id,name) VALUES('ws_local',?) ON CONFLICT(workspace_id) DO UPDATE SET name=excluded.name,updated_at=CURRENT_TIMESTAMP`, cfg.Workspace.Name); err != nil {
			return fmt.Errorf("save workspace: %w", err)
		}
		for _, id := range agentIDs {
			definition := cfg.Agents[id]
			safeConfig := struct {
				Model       string                             `json:"model"`
				Provider    string                             `json:"provider,omitempty"`
				Autonomy    int                                `json:"autonomy"`
				Tools       []string                           `json:"tools"`
				Permissions map[string]config.PermissionConfig `json:"permissions"`
			}{definition.Model, definition.Provider, definition.Autonomy, definition.Tools, definition.Permissions}
			encoded, err := json.Marshal(safeConfig)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO agents(workspace_id,agent_id,display_name,role,status,config_json) VALUES('ws_local',?,?,?,'idle',?) ON CONFLICT(workspace_id,agent_id) DO UPDATE SET display_name=excluded.display_name,role=excluded.role,config_json=excluded.config_json,updated_at=CURRENT_TIMESTAMP`, id, id, definition.Role, string(encoded)); err != nil {
				return fmt.Errorf("save agent %q: %w", id, err)
			}
		}
		return nil
	})
}

func workspaceAddress(cfg *config.Config) string {
	return net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
}
func readyURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err == nil && (host == "0.0.0.0" || host == "::") {
		host = "127.0.0.1"
		addr = net.JoinHostPort(host, port)
	}
	return "http://" + addr + "/ready"
}
func getReady(addr string) (*http.Response, error) {
	client := http.Client{Timeout: 750 * time.Millisecond}
	return client.Get(readyURL(addr))
}
func readyInstance(addr, expected string) string {
	response, err := getReady(addr)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ""
	}
	id := response.Header.Get("X-Societas-Instance-ID")
	if expected != "" && id != expected {
		return ""
	}
	return id
}
func waitReady(addr, instance string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if readyInstance(addr, instance) == instance {
			return true
		}
		time.Sleep(30 * time.Millisecond)
	}
	return false
}
func replaceEnv(values []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(values)+1)
	for _, entry := range values {
		if !strings.HasPrefix(entry, prefix) {
			out = append(out, entry)
		}
	}
	return append(out, prefix+value)
}
func usage(w io.Writer) error {
	_, err := io.WriteString(w, "Societas local workspace\n\nUsage: societas <init|serve|start|status|stop> [options]\n")
	return err
}
