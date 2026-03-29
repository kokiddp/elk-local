package integration

import "testing"

func TestSystemFolderOpenerUsesXDGOpenOnLinux(t *testing.T) {
	t.Parallel()

	opener := SystemFolderOpener{
		LookPath: func(file string) (string, error) {
			if file == "xdg-open" {
				return "/usr/bin/xdg-open", nil
			}
			return "", assertLookPathError(file)
		},
		GOOS: "linux",
	}

	command, args, err := opener.commandFor("/tmp/project")
	if err != nil {
		t.Fatalf("commandFor: %v", err)
	}
	if command != "/usr/bin/xdg-open" {
		t.Fatalf("unexpected command: %s", command)
	}
	if len(args) != 1 || args[0] != "/tmp/project" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestSystemFolderOpenerUsesOpenOnDarwin(t *testing.T) {
	t.Parallel()

	opener := SystemFolderOpener{
		LookPath: func(file string) (string, error) {
			if file == "open" {
				return "/usr/bin/open", nil
			}
			return "", assertLookPathError(file)
		},
		GOOS: "darwin",
	}

	command, args, err := opener.commandFor("/tmp/project")
	if err != nil {
		t.Fatalf("commandFor: %v", err)
	}
	if command != "/usr/bin/open" {
		t.Fatalf("unexpected command: %s", command)
	}
	if len(args) != 1 || args[0] != "/tmp/project" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestSystemFolderOpenerPrefersWSLCommands(t *testing.T) {
	t.Parallel()

	opener := SystemFolderOpener{
		LookPath: func(file string) (string, error) {
			if file == "wslview" {
				return "/usr/bin/wslview", nil
			}
			return "", assertLookPathError(file)
		},
		GOOS:  "linux",
		IsWSL: func() bool { return true },
	}

	command, args, err := opener.commandFor("/tmp/project")
	if err != nil {
		t.Fatalf("commandFor: %v", err)
	}
	if command != "/usr/bin/wslview" {
		t.Fatalf("unexpected command: %s", command)
	}
	if len(args) != 1 || args[0] != "/tmp/project" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestSystemFolderOpenerReturnsHelpfulErrorWhenUnavailable(t *testing.T) {
	t.Parallel()

	opener := SystemFolderOpener{
		LookPath: func(file string) (string, error) { return "", assertLookPathError(file) },
		GOOS:     "linux",
	}

	if _, _, err := opener.commandFor("/tmp/project"); err == nil || err.Error() != "could not find a system file explorer command on PATH" {
		t.Fatalf("unexpected error: %v", err)
	}
}
