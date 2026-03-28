package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReturnsDefaultsWhenConfigIsMissing(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("ELK_LOCAL_HOME", "")
	t.Setenv("ELK_LOCAL_ENVIRONMENTS_DIR", "")
	t.Setenv("ELK_LOCAL_BACKUPS_DIR", "")
	t.Setenv("ELK_LOCAL_WEBUI_PORT", "")

	settings, configured, err := Load()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	if configured {
		t.Fatal("expected missing config file to report not configured")
	}

	if settings.EnvironmentsDir != filepath.Join(homeDir, "elk-local", "environments") {
		t.Fatalf("unexpected environments dir: %s", settings.EnvironmentsDir)
	}
	if settings.BackupsDir != filepath.Join(homeDir, "elk-local", "backups") {
		t.Fatalf("unexpected backups dir: %s", settings.BackupsDir)
	}
	if settings.WebUIPort != DefaultWebUIPort {
		t.Fatalf("unexpected web UI port: %d", settings.WebUIPort)
	}
}

func TestLoadMergesConfigFileWithDefaults(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("ELK_LOCAL_HOME", "")
	t.Setenv("ELK_LOCAL_ENVIRONMENTS_DIR", "")
	t.Setenv("ELK_LOCAL_BACKUPS_DIR", "")
	t.Setenv("ELK_LOCAL_WEBUI_PORT", "")

	installRoot := filepath.Join(homeDir, ".elk-local")
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatalf("create install root: %v", err)
	}

	configContents := []byte("environmentsDir: ~/custom-envs\nwebuiPort: 4317\nshellRC: ~/.bashrc\ndaemonAutoStart: true\n")
	if err := os.WriteFile(filepath.Join(installRoot, "config.yaml"), configContents, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	settings, configured, err := Load()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	if !configured {
		t.Fatal("expected config file to report configured")
	}

	if settings.EnvironmentsDir != filepath.Join(homeDir, "custom-envs") {
		t.Fatalf("unexpected environments dir: %s", settings.EnvironmentsDir)
	}
	if settings.BackupsDir != filepath.Join(homeDir, "elk-local", "backups") {
		t.Fatalf("expected default backups dir, got %s", settings.BackupsDir)
	}
	if settings.ShellRC != filepath.Join(homeDir, ".bashrc") {
		t.Fatalf("unexpected shell rc path: %s", settings.ShellRC)
	}
	if settings.WebUIPort != 4317 {
		t.Fatalf("unexpected web UI port: %d", settings.WebUIPort)
	}
	if !settings.DaemonAutoStart {
		t.Fatal("expected daemonAutoStart from config file")
	}
}