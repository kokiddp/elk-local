package daemon

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildServeArgsIncludesWebUIDistWhenProvided(t *testing.T) {
	t.Parallel()

	args := buildServeArgs(StartOptions{
		Host:        "127.0.0.1",
		Port:        4173,
		ProjectRoot: "/tmp/project",
		WebUIDist:   "webui/dist",
	})

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "serve --host 127.0.0.1 --port 4173 --project-root /tmp/project") {
		t.Fatalf("unexpected serve args: %v", args)
	}
	if !strings.Contains(joined, "--webui-dist webui/dist") {
		t.Fatalf("expected webui-dist flag, got %v", args)
	}
}

func TestResolveStaticDirReturnsCleanDirectory(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "webui", "dist")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("mkdir dist dir: %v", err)
	}

	resolved := resolveStaticDir(filepath.Join(directory, "."))
	if resolved != filepath.Clean(directory) {
		t.Fatalf("unexpected resolved static dir: %s", resolved)
	}
}

func TestInspectReportsRunningWhenRemoteStatusSucceeds(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true,"projectRoot":"` + projectRoot + `","staticDir":"/tmp/dist"}`))
	}))
	defer server.Close()

	state := mustStateFromURL(t, server.URL)
	state.ProjectRoot = projectRoot
	state.LogFile = DefaultLogPath(projectRoot)
	state.StartedAt = time.Now().UTC().Format(time.RFC3339)

	if err := SaveState(projectRoot, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	status, err := Inspect(projectRoot)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}

	if !status.Running || !status.Remote.OK {
		t.Fatalf("expected running daemon status, got %+v", status)
	}
	if status.Remote.StaticDir != "/tmp/dist" {
		t.Fatalf("unexpected static dir: %+v", status.Remote)
	}
}

func TestInspectRemovesStateForMissingProcess(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	state := State{
		PID:         999999999,
		Host:        "127.0.0.1",
		Port:        1,
		ProjectRoot: projectRoot,
		LogFile:     DefaultLogPath(projectRoot),
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := SaveState(projectRoot, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	status, err := Inspect(projectRoot)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}

	if !status.Stale || status.Running {
		t.Fatalf("expected stale status, got %+v", status)
	}

	if _, err := os.Stat(StatePath(projectRoot)); !os.IsNotExist(err) {
		t.Fatalf("expected stale state file removal, stat error: %v", err)
	}
}

func TestWaitForProcessExitReturnsImmediatelyForMissingProcess(t *testing.T) {
	t.Parallel()

	if err := waitForProcessExit(999999999, 500*time.Millisecond); err != nil {
		t.Fatalf("expected missing process to be treated as exited, got %v", err)
	}
}
