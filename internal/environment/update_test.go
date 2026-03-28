package environment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateSwitchesRuntimeAndRegeneratesFiles(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	_, err := Create(CreateOptions{
		Name:        "switch-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	updated, err := Update(UpdateOptions{
		Name:           "switch-demo",
		ProjectRoot:    projectRoot,
		PHPVersion:     "7.4",
		WebServer:      "nginx",
		DatabaseEngine: "mysql",
	})
	if err != nil {
		t.Fatalf("update environment: %v", err)
	}

	if updated.Manifest.Runtime.PHPVersion != "7.4" {
		t.Fatalf("unexpected PHP version: %s", updated.Manifest.Runtime.PHPVersion)
	}

	if updated.Manifest.Runtime.WebServer != "nginx" {
		t.Fatalf("unexpected web server: %s", updated.Manifest.Runtime.WebServer)
	}

	if updated.Manifest.Runtime.Database.Engine != "mysql" {
		t.Fatalf("unexpected database engine: %s", updated.Manifest.Runtime.Database.Engine)
	}

	if updated.Manifest.Runtime.Database.Version != "8.4" {
		t.Fatalf("unexpected database version: %s", updated.Manifest.Runtime.Database.Version)
	}

	composeContents, err := os.ReadFile(updated.ComposePath)
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}

	composeText := string(composeContents)
	phpBuildContext := filepath.Join(updated.Manifest.Storage.BasePath, "php")
	if !strings.Contains(composeText, "context: \""+phpBuildContext+"\"") {
		t.Fatalf("compose file does not use the switched PHP build context: %s", composeText)
	}

	if !strings.Contains(composeText, "image: mysql:8.4") {
		t.Fatalf("compose file does not use mysql after switching: %s", composeText)
	}

	if updated.NginxConfigPath == "" {
		t.Fatal("expected nginx config path after switching to nginx")
	}
}

func TestUpdateRemovesNginxFilesWhenSwitchingBackToApache(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	created, err := Create(CreateOptions{
		Name:        "apache-switch-demo",
		Preset:      "custom",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	if created.NginxConfigPath == "" {
		t.Fatal("expected nginx config for custom preset")
	}

	updated, err := Update(UpdateOptions{
		Name:        "apache-switch-demo",
		ProjectRoot: projectRoot,
		WebServer:   "apache",
	})
	if err != nil {
		t.Fatalf("update environment: %v", err)
	}

	if updated.NginxConfigPath != "" {
		t.Fatalf("expected nginx config path to be empty after switching to apache")
	}

	if _, err := os.Stat(created.NginxConfigPath); !os.IsNotExist(err) {
		t.Fatalf("expected nginx config to be removed, stat error: %v", err)
	}
}

func TestUpdateCanToggleOptionalTooling(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	created, err := Create(CreateOptions{
		Name:        "tooling-switch-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	updated, err := Update(UpdateOptions{
		Name:          created.Manifest.Name,
		ProjectRoot:   projectRoot,
		EnableAdminer: true,
		EnableMailpit: true,
		EnableXdebug:  true,
	})
	if err != nil {
		t.Fatalf("update environment: %v", err)
	}

	if !updated.Manifest.Tooling.Adminer.Enabled || !updated.Manifest.Tooling.Mailpit.Enabled || !updated.Manifest.Tooling.Xdebug.Enabled {
		t.Fatal("expected all optional tooling to be enabled")
	}

	updated, err = Update(UpdateOptions{
		Name:           created.Manifest.Name,
		ProjectRoot:    projectRoot,
		DisableAdminer: true,
		DisableMailpit: true,
		DisableXdebug:  true,
	})
	if err != nil {
		t.Fatalf("disable tooling: %v", err)
	}

	if updated.Manifest.Tooling.Adminer.Enabled || updated.Manifest.Tooling.Mailpit.Enabled || updated.Manifest.Tooling.Xdebug.Enabled {
		t.Fatal("expected all optional tooling to be disabled")
	}

	if updated.XdebugDirPath != "" {
		t.Fatalf("expected xdebug output to be removed after disabling")
	}

	if _, err := os.Stat(filepath.Join(updated.Manifest.Storage.BasePath, "php", "xdebug.ini")); !os.IsNotExist(err) {
		t.Fatalf("expected xdebug.ini to be removed after disabling, stat error: %v", err)
	}
}

func TestUpdateSyncsDatabaseCredentialsIntoApplicationConfig(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	created, err := Create(CreateOptions{
		Name:        "laravel-sync-demo",
		Preset:      "laravel",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	updated, err := Update(UpdateOptions{
		Name:                 created.Manifest.Name,
		ProjectRoot:          projectRoot,
		DatabaseName:         "team_app",
		DatabaseUser:         "team_user",
		DatabasePassword:     "team-pass",
		DatabaseRootPassword: "root-pass",
	})
	if err != nil {
		t.Fatalf("update environment: %v", err)
	}

	if updated.Manifest.Runtime.Database.Name != "team_app" {
		t.Fatalf("unexpected database name: %s", updated.Manifest.Runtime.Database.Name)
	}

	envContents, err := os.ReadFile(filepath.Join(projectRoot, ".env"))
	if err != nil {
		t.Fatalf("read synced env: %v", err)
	}

	envText := string(envContents)
	if !strings.Contains(envText, "DB_DATABASE=\"team_app\"") {
		t.Fatalf("env file does not include updated db name: %s", envText)
	}

	if !strings.Contains(envText, "DB_USERNAME=\"team_user\"") {
		t.Fatalf("env file does not include updated db user: %s", envText)
	}

	if !strings.Contains(envText, "DB_PASSWORD=\"team-pass\"") {
		t.Fatalf("env file does not include updated db password: %s", envText)
	}

	composeContents, err := os.ReadFile(updated.ComposePath)
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}

	if !strings.Contains(string(composeContents), "MARIADB_DATABASE: \"team_app\"") {
		t.Fatalf("compose file does not include updated db name: %s", string(composeContents))
	}
}
