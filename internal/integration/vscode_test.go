package integration

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVSCodeOpenerPrefersWSLCodeCommand(t *testing.T) {
	t.Parallel()

	opener := VSCodeOpener{
		LookPath: func(file string) (string, error) {
			if file == "code" {
				return "/usr/bin/code", nil
			}
			return "", assertLookPathError(file)
		},
		IsWSL: func() bool { return true },
	}

	command, args, err := opener.commandFor("/tmp/project")
	if err != nil {
		t.Fatalf("commandFor: %v", err)
	}
	if command != "/usr/bin/code" {
		t.Fatalf("unexpected command: %s", command)
	}
	if len(args) != 1 || args[0] != "/tmp/project" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestVSCodeOpenerFallsBackToCodeCmdWithWSLFolderURI(t *testing.T) {
	t.Parallel()

	opener := VSCodeOpener{
		LookPath: func(file string) (string, error) {
			if file == "code.cmd" {
				return `C:\\Program Files\\Microsoft VS Code\\bin\\code.cmd`, nil
			}
			return "", assertLookPathError(file)
		},
		IsWSL:     func() bool { return true },
		WSLDistro: "Ubuntu",
	}

	projectPath := filepath.Join(string(filepath.Separator), "tmp", "My Project")
	command, args, err := opener.commandFor(projectPath)
	if err != nil {
		t.Fatalf("commandFor: %v", err)
	}
	if !strings.HasSuffix(command, "code.cmd") {
		t.Fatalf("unexpected command: %s", command)
	}
	if len(args) != 2 || args[0] != "--folder-uri" {
		t.Fatalf("unexpected args: %#v", args)
	}
	if !strings.Contains(args[1], "vscode-remote://wsl+Ubuntu/tmp/My%20Project") {
		t.Fatalf("unexpected folder uri: %s", args[1])
	}
}

func TestVSCodeOpenerReturnsHelpfulErrorWhenUnavailable(t *testing.T) {
	t.Parallel()

	opener := VSCodeOpener{
		LookPath: func(file string) (string, error) { return "", assertLookPathError(file) },
	}

	if _, _, err := opener.commandFor("/tmp/project"); err == nil || !strings.Contains(err.Error(), "could not find VS Code on PATH") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertLookPathError(file string) error {
	return &fakeLookPathError{file: file}
}

type fakeLookPathError struct {
	file string
}

func (err *fakeLookPathError) Error() string {
	return err.file + " not found"
}
