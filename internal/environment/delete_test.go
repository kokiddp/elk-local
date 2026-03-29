package environment

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteRemovesManagedEnvironmentPaths(t *testing.T) {
	userHome := t.TempDir()
	t.Setenv("HOME", userHome)
	t.Setenv("ELK_LOCAL_HOME", "")
	configureManagedEnvironmentDefaults(t, userHome)

	created, err := Create(CreateOptions{
		Name:      "delete-managed-demo",
		Preset:    "wordpress",
		Installer: stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	backupFile := filepath.Join(created.Manifest.Storage.BackupsPath, "keep.tar.gz")
	if err := os.WriteFile(backupFile, []byte("backup"), 0o644); err != nil {
		t.Fatalf("write backup file: %v", err)
	}

	result, err := Delete(userHome, created.Manifest.Name)
	if err != nil {
		t.Fatalf("delete environment: %v", err)
	}

	if !result.RemovedProjectFiles {
		t.Fatal("expected managed project files to be removed")
	}

	if !result.RemovedBackups {
		t.Fatal("expected managed backups to be removed")
	}

	assertPathMissing(t, created.Manifest.Storage.BasePath)
	assertPathMissing(t, created.Manifest.Project.Root)
	assertPathMissing(t, created.Manifest.Storage.BackupsPath)
	assertPathMissing(t, filepath.Join(userHome, ".elk-local", "environments", created.Manifest.Name))
}

func TestDeleteKeepsExternalProjectRoot(t *testing.T) {
	projectRoot := t.TempDir()
	userHome := t.TempDir()
	t.Setenv("HOME", userHome)
	t.Setenv("ELK_LOCAL_HOME", "")
	configureManagedEnvironmentDefaults(t, userHome)

	created, err := Create(CreateOptions{
		Name:        "delete-external-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	markerPath := filepath.Join(projectRoot, "marker.txt")
	if err := os.WriteFile(markerPath, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	result, err := Delete(userHome, created.Manifest.Name)
	if err != nil {
		t.Fatalf("delete environment: %v", err)
	}

	if result.RemovedProjectFiles {
		t.Fatal("expected external project root to be preserved")
	}

	assertPathMissing(t, created.Manifest.Storage.BasePath)
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("expected external project root contents to remain, stat error: %v", err)
	}
	assertPathMissing(t, created.Manifest.Storage.BackupsPath)
	if !result.RemovedBackups {
		t.Fatal("expected managed backups to be removed")
	}
}

func configureManagedEnvironmentDefaults(t *testing.T, userHome string) {
	t.Helper()

	homeRoot := filepath.Join(userHome, ".elk-local")
	if err := os.MkdirAll(homeRoot, 0o755); err != nil {
		t.Fatalf("create home root: %v", err)
	}

	configContents := []byte("environmentsDir: ~/elk-local/environments\nbackupsDir: ~/elk-local/backups\n")
	if err := os.WriteFile(filepath.Join(homeRoot, "config.yaml"), configContents, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, stat err=%v", path, err)
	}
}
