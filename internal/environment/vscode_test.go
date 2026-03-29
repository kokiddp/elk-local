package environment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncVSCodeLaunchConfigPreservesCustomConfigurations(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	vscodeDir := filepath.Join(projectRoot, ".vscode")
	if err := os.MkdirAll(vscodeDir, 0o755); err != nil {
		t.Fatalf("create .vscode dir: %v", err)
	}

	existing := `{
	"version": "0.2.0",
	"configurations": [
		{
			"name": "Custom PHP CLI",
			"type": "php",
			"request": "launch",
			"port": 9010
		},
		{
			"name": "Listen for Xdebug (Local)",
			"type": "php",
			"request": "launch",
			"port": 9999
		}
	]
}`

	launchPath := filepath.Join(vscodeDir, "launch.json")
	if err := os.WriteFile(launchPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing launch.json: %v", err)
	}

	manifest := Manifest{Project: Project{Root: projectRoot}, Tooling: Tooling{Xdebug: Xdebug{Enabled: true}}}
	if err := syncVSCodeLaunchConfig(manifest); err != nil {
		t.Fatalf("sync launch config: %v", err)
	}

	contents, err := os.ReadFile(launchPath)
	if err != nil {
		t.Fatalf("read merged launch.json: %v", err)
	}

	text := string(contents)
	if !strings.Contains(text, "Custom PHP CLI") {
		t.Fatalf("expected custom launch config to be preserved: %s", text)
	}
	if strings.Contains(text, "\"port\": 9999") {
		t.Fatalf("expected managed launch config to be replaced with ELK default ports: %s", text)
	}
	if !strings.Contains(text, "Listen for Xdebug 3.0 (Local)") || !strings.Contains(text, "\"port\": 9003") {
		t.Fatalf("expected managed Xdebug launch config to be present: %s", text)
	}

	manifest.Tooling.Xdebug.Enabled = false
	if err := syncVSCodeLaunchConfig(manifest); err != nil {
		t.Fatalf("prune launch config: %v", err)
	}

	contents, err = os.ReadFile(launchPath)
	if err != nil {
		t.Fatalf("read pruned launch.json: %v", err)
	}

	text = string(contents)
	if !strings.Contains(text, "Custom PHP CLI") {
		t.Fatalf("expected custom launch config to remain after pruning managed entries: %s", text)
	}
	if strings.Contains(text, "Listen for Xdebug 3.0 (Local)") || strings.Contains(text, "Launch currently open script") {
		t.Fatalf("expected managed Xdebug launch configs to be removed after pruning: %s", text)
	}
}