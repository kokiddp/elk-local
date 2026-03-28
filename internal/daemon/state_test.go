package daemon

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSaveLoadAndRemoveStateRoundTrip(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	state := State{
		PID:         4321,
		Host:        "127.0.0.1",
		Port:        4173,
		ProjectRoot: projectRoot,
		LogFile:     DefaultLogPath(projectRoot),
		StaticDir:   filepath.Join(projectRoot, "webui", "dist"),
		StartedAt:   "2026-03-28T03:00:00Z",
	}

	if err := SaveState(projectRoot, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	loaded, err := LoadState(projectRoot)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if loaded != state {
		t.Fatalf("unexpected loaded state: %+v", loaded)
	}

	if err := RemoveState(projectRoot); err != nil {
		t.Fatalf("remove state: %v", err)
	}

	if _, err := os.Stat(StatePath(projectRoot)); !os.IsNotExist(err) {
		t.Fatalf("expected state file to be removed, stat error: %v", err)
	}
}

func TestResolveProjectRootRejectsFiles(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := ResolveProjectRoot(filePath); err == nil {
		t.Fatal("expected file project root to be rejected")
	}
}

func TestResolveProjectRootUsesHomeWhenInstalledConfigExists(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("ELK_LOCAL_HOME", "")
	t.Setenv("ELK_LOCAL_ENVIRONMENTS_DIR", "")
	t.Setenv("ELK_LOCAL_BACKUPS_DIR", "")

	configDir := filepath.Join(homeDir, ".elk-local")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("environmentsDir: ~/elk-local/environments\n"), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	resolvedPath, err := ResolveProjectRoot("")
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}

	if resolvedPath != homeDir {
		t.Fatalf("expected home dir project root, got %s", resolvedPath)
	}
}

func TestQueryStatusAndShutdownRoundTrip(t *testing.T) {
	t.Parallel()

	var shutdownCalled atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/daemon/status":
			if request.Method != http.MethodGet {
				t.Fatalf("unexpected status method: %s", request.Method)
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"ok":true,"projectRoot":"/tmp/demo","staticDir":"/tmp/demo/webui/dist"}`))
		case "/api/daemon/shutdown":
			if request.Method != http.MethodPost {
				t.Fatalf("unexpected shutdown method: %s", request.Method)
			}
			shutdownCalled.Store(true)
			writer.WriteHeader(http.StatusAccepted)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	state := mustStateFromURL(t, server.URL)

	status, err := QueryStatus(state, 2*time.Second)
	if err != nil {
		t.Fatalf("query status: %v", err)
	}

	if !status.OK || status.ProjectRoot != "/tmp/demo" || status.StaticDir == "" {
		t.Fatalf("unexpected status payload: %+v", status)
	}

	if err := Shutdown(state, 2*time.Second); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	if !shutdownCalled.Load() {
		t.Fatal("expected shutdown endpoint to be called")
	}
}

func TestWaitUntilReachableAndStopped(t *testing.T) {
	t.Parallel()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/daemon/status":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"ok":true,"projectRoot":"/tmp/demo"}`))
		case "/api/daemon/shutdown":
			writer.WriteHeader(http.StatusAccepted)
			go server.Close()
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	state := mustStateFromURL(t, server.URL)

	if _, err := WaitUntilReachable(state, 3*time.Second); err != nil {
		t.Fatalf("wait until reachable: %v", err)
	}

	if err := Shutdown(state, 2*time.Second); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	if err := WaitUntilStopped(state, 3*time.Second); err != nil {
		t.Fatalf("wait until stopped: %v", err)
	}
}

func TestQueryStatusRejectsUnexpectedHTTPStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusTeapot)
	}))
	defer server.Close()

	state := mustStateFromURL(t, server.URL)
	if _, err := QueryStatus(state, 2*time.Second); err == nil || !strings.Contains(err.Error(), "418") {
		t.Fatalf("expected unexpected status error, got %v", err)
	}
}

func mustStateFromURL(t *testing.T, rawURL string) State {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	host := parsed.Hostname()
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	return State{Host: host, Port: port}
}
