package cli

import (
	"bytes"
	"strings"
	"testing"

	"elk-local/internal/daemon"
)

func TestDaemonStatusCommandPrintsNotRunning(t *testing.T) {
	t.Parallel()

	command := newDaemonStatusCommand()
	stdout := &bytes.Buffer{}
	command.SetOut(stdout)
	command.SetErr(stdout)
	command.SetArgs([]string{"--project-root", t.TempDir()})

	if err := command.Execute(); err != nil {
		t.Fatalf("execute status command: %v", err)
	}

	if !strings.Contains(stdout.String(), "ELK-Local daemon is not running.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestDaemonStopCommandPrintsNotRunning(t *testing.T) {
	t.Parallel()

	command := newDaemonStopCommand()
	stdout := &bytes.Buffer{}
	command.SetOut(stdout)
	command.SetErr(stdout)
	command.SetArgs([]string{"--project-root", t.TempDir()})

	if err := command.Execute(); err != nil {
		t.Fatalf("execute stop command: %v", err)
	}

	if !strings.Contains(stdout.String(), "ELK-Local daemon is not running.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestDaemonCommandIncludesLifecycleSubcommands(t *testing.T) {
	t.Parallel()

	command := newDaemonCommand()
	expectedUses := []string{"start", "stop", "status", "restart"}

	for _, expectedUse := range expectedUses {
		if _, _, err := command.Find([]string{expectedUse}); err != nil {
			t.Fatalf("expected daemon subcommand %q: %v", expectedUse, err)
		}
	}
}

func TestPrintDaemonRunningIncludesStaticDirWhenPresent(t *testing.T) {
	t.Parallel()

	command := newDaemonStatusCommand()
	stdout := &bytes.Buffer{}
	command.SetOut(stdout)

	printDaemonRunning(command, "ELK-Local daemon is running.", daemon.Status{
		State: daemon.State{
			PID:         123,
			Host:        "127.0.0.1",
			Port:        4173,
			ProjectRoot: "/tmp/project",
			LogFile:     "/tmp/project/.elk-local/daemon/daemon.log",
		},
		Remote: daemon.RemoteStatus{StaticDir: "/tmp/project/webui/dist"},
	})

	output := stdout.String()
	for _, expected := range []string{
		"ELK-Local daemon is running.",
		"URL: http://127.0.0.1:4173",
		"PID: 123",
		"Project root: /tmp/project",
		"Web UI assets: /tmp/project/webui/dist",
		"Log file: /tmp/project/.elk-local/daemon/daemon.log",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got %s", expected, output)
		}
	}
}
