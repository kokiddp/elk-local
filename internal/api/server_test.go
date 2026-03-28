package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"elk-local/internal/daemon"
	"elk-local/internal/environment"
)

type fakeComposeExecutor struct {
	outputByArgs map[string]string
	errorByArgs  map[string]error
	args         []string
	stdin        []byte
}

type fakeApplicationInstaller struct{}

func (fakeApplicationInstaller) Install(request environment.ApplicationInstallRequest) (environment.InstalledApplication, error) {
	if !request.Preset.InstallsApplication() {
		return environment.InstalledApplication{}, nil
	}

	return environment.InstalledApplication{
		Name:    request.Preset.ApplicationName,
		Version: firstNonEmpty(request.RequestedVersion, request.Preset.DefaultAppVersion),
	}, nil
}

func (executor *fakeComposeExecutor) Run(composeFile string, args ...string) (string, error) {
	executor.args = append(executor.args, strings.Join(args, " "))
	key := strings.Join(args, " ")
	return executor.outputByArgs[key], executor.errorByArgs[key]
}

func (executor *fakeComposeExecutor) RunWithInput(composeFile string, input []byte, args ...string) (string, error) {
	executor.stdin = append([]byte{}, input...)
	return executor.Run(composeFile, args...)
}

func TestHandleEnvironmentsListsManagedStacks(t *testing.T) {
	isolateUserHome(t)

	projectRoot := t.TempDir()
	_, err := environment.Create(environment.CreateOptions{
		Name:           "ui-demo",
		Preset:         "wordpress",
		ProjectRoot:    projectRoot,
		AdminerEnabled: true,
		Installer:      fakeApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	executor := &fakeComposeExecutor{
		outputByArgs: map[string]string{
			"ps --format json": `[{"Name":"elk-ui-demo-web","Service":"web","State":"running","Health":"","Publishers":[{"PublishedPort":20858,"TargetPort":80,"Protocol":"tcp"}]}]`,
		},
		errorByArgs: map[string]error{},
	}

	server, err := NewServer(projectRoot, "", executor, fakeApplicationInstaller{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/environments", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", response.Code, response.Body.String())
	}

	var payload environmentsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(payload.Environments) != 1 {
		t.Fatalf("expected one environment, got %d", len(payload.Environments))
	}

	if payload.DefaultProjectRootBase == "" {
		t.Fatal("expected default project root base in API response")
	}

	if payload.Environments[0].Status.State != "running" {
		t.Fatalf("unexpected environment state: %s", payload.Environments[0].Status.State)
	}

	if payload.Environments[0].URLs.Adminer == "" {
		t.Fatal("expected adminer URL in API response")
	}
}

func TestHandleDaemonStatusReportsServerIdentity(t *testing.T) {
	isolateUserHome(t)

	projectRoot := t.TempDir()
	server, err := NewServer(projectRoot, "", &fakeComposeExecutor{}, fakeApplicationInstaller{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/daemon/status", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", response.Code, response.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload["ok"] != true {
		t.Fatalf("expected ok=true payload, got %+v", payload)
	}

	if payload["projectRoot"] != projectRoot {
		t.Fatalf("unexpected project root: %+v", payload)
	}
}

func TestDaemonShutdownStopsListener(t *testing.T) {
	isolateUserHome(t)

	projectRoot := t.TempDir()
	server, err := NewServer(projectRoot, "", &fakeComposeExecutor{}, fakeApplicationInstaller{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Listen(listener)
	}()

	address := listener.Addr().(*net.TCPAddr)
	state := daemon.State{Host: "127.0.0.1", Port: address.Port}
	if _, err := daemon.WaitUntilReachable(state, 5*time.Second); err != nil {
		t.Fatalf("wait until reachable: %v", err)
	}

	if err := daemon.Shutdown(state, 2*time.Second); err != nil {
		t.Fatalf("shutdown daemon endpoint: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("listener returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop after daemon shutdown request")
	}
}

func TestHandleCreateEnvironmentCreatesManagedStack(t *testing.T) {
	isolateUserHome(t)

	projectRoot := t.TempDir()
	server, err := NewServer(projectRoot, "", &fakeComposeExecutor{
		outputByArgs: map[string]string{"ps --format json": "[]"},
		errorByArgs:  map[string]error{},
	}, fakeApplicationInstaller{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	body := strings.NewReader(fmt.Sprintf(`{"name":"ui-create-demo","preset":"laravel","projectRoot":%q,"adminerEnabled":true,"mailpitEnabled":true,"databaseName":"panel_db","databaseUser":"panel","databasePassword":"secret"}`, projectRoot))
	request := httptest.NewRequest(http.MethodPost, "/api/environments", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("unexpected status code: %d body=%s", response.Code, response.Body.String())
	}

	manifestPath, err := environment.ResolveManifestPath(projectRoot, "ui-create-demo")
	if err != nil {
		t.Fatalf("resolve manifest path: %v", err)
	}

	manifest, err := environment.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	if manifest.Runtime.Database.Name != "panel_db" {
		t.Fatalf("unexpected database name: %s", manifest.Runtime.Database.Name)
	}

	if manifest.Project.Root != projectRoot {
		t.Fatalf("unexpected project root: got %s want %s", manifest.Project.Root, projectRoot)
	}

	if !manifest.Tooling.Adminer.Enabled || !manifest.Tooling.Mailpit.Enabled {
		t.Fatal("expected optional tooling to be enabled by create endpoint")
	}

	if manifest.Application.Name != "Laravel" || manifest.Application.Version != "latest" {
		t.Fatalf("unexpected installed application metadata: %+v", manifest.Application)
	}
}

func TestHandleCreateEnvironmentUsesInstalledDefaultProjectRoot(t *testing.T) {
	isolateUserHome(t)

	userHome := t.TempDir()
	environmentsDir := filepath.Join(userHome, "elk-local", "environments")
	configureInstalledDefaults(t, userHome, environmentsDir)

	server, err := NewServer(userHome, "", &fakeComposeExecutor{
		outputByArgs: map[string]string{"ps --format json": "[]"},
		errorByArgs:  map[string]error{},
	}, fakeApplicationInstaller{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	body := strings.NewReader(`{"name":"ui-default-root","preset":"wordpress"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/environments", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("unexpected status code: %d body=%s", response.Code, response.Body.String())
	}

	manifestPath, err := environment.ResolveManifestPath(userHome, "ui-default-root")
	if err != nil {
		t.Fatalf("resolve manifest path: %v", err)
	}

	manifest, err := environment.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	expectedProjectRoot := filepath.Join(environmentsDir, "ui-default-root")
	if manifest.Project.Root != expectedProjectRoot {
		t.Fatalf("unexpected project root: got %s want %s", manifest.Project.Root, expectedProjectRoot)
	}

	var payload environmentResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Environment.ProjectRoot != expectedProjectRoot {
		t.Fatalf("unexpected response project root: got %s want %s", payload.Environment.ProjectRoot, expectedProjectRoot)
	}
}

func configureInstalledDefaults(t *testing.T, userHome string, environmentsDir string) {
	t.Helper()

	homeRoot := filepath.Join(userHome, ".elk-local")
	if err := os.MkdirAll(homeRoot, 0o755); err != nil {
		t.Fatalf("create home root: %v", err)
	}

	t.Setenv("HOME", userHome)
	t.Setenv("ELK_LOCAL_HOME", "")

	configContents := fmt.Sprintf("environmentsDir: %s\nbackupsDir: %s\n", environmentsDir, filepath.Join(userHome, "elk-local", "backups"))
	if err := os.WriteFile(filepath.Join(homeRoot, "config.yaml"), []byte(configContents), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestHandleEnvironmentActionRunsLifecycle(t *testing.T) {
	isolateUserHome(t)

	projectRoot := t.TempDir()
	created, err := environment.Create(environment.CreateOptions{
		Name:        "ui-action-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		Installer:   fakeApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	statusOutput := fmt.Sprintf(`[{"Name":"%s-web","Service":"web","State":"running","Health":"","Publishers":[]}]`, created.Manifest.Compose.NamePrefix)
	executor := &fakeComposeExecutor{
		outputByArgs: map[string]string{
			"up -d":            "started",
			"ps --format json": statusOutput,
		},
		errorByArgs: map[string]error{},
	}

	server, err := NewServer(projectRoot, "", executor, fakeApplicationInstaller{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/environments/ui-action-demo/actions/start", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", response.Code, response.Body.String())
	}

	if len(executor.args) < 2 || executor.args[0] != "up -d" {
		t.Fatalf("expected lifecycle start call, got %#v", executor.args)
	}

	var payload environmentResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Environment.Status.State != "running" {
		t.Fatalf("unexpected environment state: %s", payload.Environment.Status.State)
	}
}

func TestHandleBackupsListsManagedArchives(t *testing.T) {
	isolateUserHome(t)

	projectRoot := t.TempDir()
	created, err := environment.Create(environment.CreateOptions{
		Name:        "ui-backups-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		Installer:   fakeApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	service := environment.NewBackupService(&fakeComposeExecutor{})
	service.Now = func() time.Time { return time.Date(2026, time.March, 28, 16, 0, 0, 0, time.UTC) }
	if _, err := service.Backup(created.Manifest, environment.BackupCreateOptions{IncludeProjectFiles: true}); err != nil {
		t.Fatalf("create managed backup: %v", err)
	}

	server, err := NewServer(projectRoot, "", &fakeComposeExecutor{}, fakeApplicationInstaller{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/environments/ui-backups-demo/backups", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d body=%s", response.Code, response.Body.String())
	}

	var payload backupInventoryResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(payload.Backups) != 1 {
		t.Fatalf("expected one backup, got %d", len(payload.Backups))
	}

	if !payload.Backups[0].IncludesProjectFiles {
		t.Fatal("expected listed backup to report included project files")
	}
}

func TestHandleBackupCreateCreatesManagedArchive(t *testing.T) {
	isolateUserHome(t)

	projectRoot := t.TempDir()
	_, err := environment.Create(environment.CreateOptions{
		Name:        "ui-backup-create-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		Installer:   fakeApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	executor := &fakeComposeExecutor{}
	server, err := NewServer(projectRoot, "", executor, fakeApplicationInstaller{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/environments/ui-backup-create-demo/backups/create", strings.NewReader(`{"includeProjectFiles":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("unexpected status code: %d body=%s", response.Code, response.Body.String())
	}

	var payload backupActionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Backup.FileName == "" || len(payload.Backups) != 1 {
		t.Fatalf("unexpected backup payload: %+v", payload)
	}

	if len(executor.args) < 3 {
		t.Fatalf("expected backup executor calls, got %#v", executor.args)
	}
	if executor.args[0] != "up -d db" {
		t.Fatalf("expected database startup before backup, got %#v", executor.args)
	}
	if !strings.Contains(strings.Join(executor.args, "\n"), "mysqldump") {
		t.Fatalf("expected mysqldump invocation, got %#v", executor.args)
	}
	if !payload.Backup.IncludesProjectFiles {
		t.Fatal("expected backup response to report included project files")
	}
	if payload.Message == "" {
		t.Fatal("expected backup response message")
	}
	if payload.EnvironmentName != "ui-backup-create-demo" {
		t.Fatalf("unexpected environment name: %s", payload.EnvironmentName)
	}
	if payload.Backup.Path == "" {
		t.Fatal("expected backup path in response")
	}
	if _, err := os.Stat(payload.Backup.Path); err != nil {
		t.Fatalf("stat created backup archive: %v", err)
	}
	if payload.Backup.DatabaseName != "ui_backup_create_demo" {
		t.Fatalf("unexpected database name in backup response: %s", payload.Backup.DatabaseName)
	}
}

func isolateUserHome(t *testing.T) string {
	t.Helper()

	userHome := t.TempDir()
	t.Setenv("HOME", userHome)
	t.Setenv("ELK_LOCAL_HOME", "")

	return userHome
}
