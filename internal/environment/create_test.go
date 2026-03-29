package environment

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

type stubApplicationInstaller struct{}

func (stubApplicationInstaller) Install(request ApplicationInstallRequest) (InstalledApplication, error) {
	if !request.Preset.InstallsApplication() {
		return InstalledApplication{}, nil
	}

	return InstalledApplication{
		Name:    request.Preset.ApplicationName,
		Version: firstNonEmpty(request.RequestedVersion, request.Preset.DefaultAppVersion),
	}, nil
}

func TestCreateWritesApacheEnvironmentArtifacts(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()

	created, err := Create(CreateOptions{
		Name:        "wp-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	if created.Manifest.Runtime.WebServer != "apache" {
		t.Fatalf("unexpected web server: %s", created.Manifest.Runtime.WebServer)
	}

	manifestContents, err := os.ReadFile(created.ManifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	if !strings.Contains(string(manifestContents), "preset: wordpress") {
		t.Fatalf("manifest does not include expected preset: %s", string(manifestContents))
	}

	if !strings.Contains(string(manifestContents), "application:") || !strings.Contains(string(manifestContents), "name: WordPress") {
		t.Fatalf("manifest does not include application metadata: %s", string(manifestContents))
	}

	composeContents, err := os.ReadFile(created.ComposePath)
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}

	composeText := string(composeContents)
	phpBuildContext := filepath.Join(created.Manifest.Storage.BasePath, "php")
	if !strings.Contains(composeText, "context: \""+phpBuildContext+"\"") {
		t.Fatalf("compose does not use the generated PHP build context: %s", composeText)
	}

	phpDockerfileContents, err := os.ReadFile(filepath.Join(phpBuildContext, "Dockerfile"))
	if err != nil {
		t.Fatalf("read PHP Dockerfile: %v", err)
	}

	phpDockerfileText := string(phpDockerfileContents)
	if !strings.Contains(phpDockerfileText, "install-php-extensions") || !strings.Contains(phpDockerfileText, "mysqli") || !strings.Contains(phpDockerfileText, "imagick") {
		t.Fatalf("PHP Dockerfile does not include required WordPress extensions: %s", phpDockerfileText)
	}

	if !strings.Contains(phpDockerfileText, "CMD [\"apache2-foreground\"]") {
		t.Fatalf("PHP Dockerfile does not preserve the apache runtime command: %s", phpDockerfileText)
	}

	if strings.Contains(composeText, "\t") {
		t.Fatalf("compose file should not contain tabs: %q", composeText)
	}

	if !strings.Contains(composeText, "container_name: elk-wp-demo-web") {
		t.Fatalf("compose does not use derived container names: %s", composeText)
	}

	portText := strconv.Itoa(created.Manifest.Network.HTTPPort)
	if !strings.Contains(composeText, "\""+portText+":"+portText+"\"") {
		t.Fatalf("compose does not mirror the apache port inside the container: %s", composeText)
	}

	if !strings.Contains(composeText, "ELK_HTTP_PORT: \""+portText+"\"") {
		t.Fatalf("compose does not expose the apache listen port to the container: %s", composeText)
	}

	hostUID, hostGID := currentHostIdentity()
	if runtime.GOOS != "windows" && hostUID != "" && !strings.Contains(composeText, "ELK_HOST_UID: \""+hostUID+"\"") {
		t.Fatalf("compose does not pass the host uid to the php container: %s", composeText)
	}
	if runtime.GOOS != "windows" && hostGID != "" && !strings.Contains(composeText, "ELK_HOST_GID: \""+hostGID+"\"") {
		t.Fatalf("compose does not pass the host gid to the php container: %s", composeText)
	}

	if created.NginxConfigPath != "" {
		t.Fatalf("apache preset should not generate nginx config")
	}

	configContents, err := os.ReadFile(filepath.Join(projectRoot, "wp-config.php"))
	if err != nil {
		t.Fatalf("read wp-config: %v", err)
	}

	if !strings.Contains(string(configContents), "define('DB_NAME', 'wp_demo');") {
		t.Fatalf("wp-config does not include database name: %s", string(configContents))
	}

	if !strings.Contains(string(configContents), "define('WP_HOME', 'http://127.0.0.1:"+portText+"');") {
		t.Fatalf("wp-config does not include WP_HOME for the local stack url: %s", string(configContents))
	}

	if !strings.Contains(string(configContents), "define('WP_SITEURL', 'http://127.0.0.1:"+portText+"');") {
		t.Fatalf("wp-config does not include WP_SITEURL for the local stack url: %s", string(configContents))
	}

	if !strings.Contains(string(configContents), "define('FS_METHOD', 'direct');") {
		t.Fatalf("wp-config does not force direct local filesystem writes: %s", string(configContents))
	}

	entrypointContents, err := os.ReadFile(filepath.Join(phpBuildContext, "docker-entrypoint.sh"))
	if err != nil {
		t.Fatalf("read PHP entrypoint: %v", err)
	}

	if !strings.Contains(string(entrypointContents), "configure_apache_port") {
		t.Fatalf("PHP entrypoint does not configure the apache listen port: %s", string(entrypointContents))
	}

	if !strings.Contains(string(entrypointContents), "exec docker-php-entrypoint \"$@\"") {
		t.Fatalf("PHP entrypoint does not delegate to docker-php-entrypoint: %s", string(entrypointContents))
	}
}

func TestCreateWritesNginxEnvironmentArtifacts(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	outputDir := filepath.Join(projectRoot, "infra")

	created, err := Create(CreateOptions{
		Name:        "api-demo",
		Preset:      "custom",
		ProjectRoot: projectRoot,
		OutputDir:   outputDir,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	if created.NginxConfigPath == "" {
		t.Fatalf("nginx preset should generate nginx config")
	}

	if _, err := os.Stat(created.NginxConfigPath); err != nil {
		t.Fatalf("stat nginx config: %v", err)
	}

	composeContents, err := os.ReadFile(created.ComposePath)
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}

	composeText := string(composeContents)
	phpBuildContext := filepath.Join(created.Manifest.Storage.BasePath, "php")
	if !strings.Contains(composeText, "context: \""+phpBuildContext+"\"") {
		t.Fatalf("compose does not use the PHP build context: %s", composeText)
	}

	phpDockerfileContents, err := os.ReadFile(filepath.Join(phpBuildContext, "Dockerfile"))
	if err != nil {
		t.Fatalf("read PHP Dockerfile: %v", err)
	}

	if !strings.Contains(string(phpDockerfileContents), "CMD [\"php-fpm\"]") {
		t.Fatalf("nginx preset should preserve the php-fpm runtime command: %s", string(phpDockerfileContents))
	}

	if !strings.Contains(composeText, "nginx:1.27-alpine") {
		t.Fatalf("compose does not include nginx: %s", composeText)
	}

	nginxContents, err := os.ReadFile(created.NginxConfigPath)
	if err != nil {
		t.Fatalf("read nginx config: %v", err)
	}

	if !strings.Contains(string(nginxContents), "root /var/www/html;") {
		t.Fatalf("custom preset should default to /var/www/html document root: %s", string(nginxContents))
	}

	if strings.Contains(composeText, "\t") || strings.Contains(string(nginxContents), "\t") {
		t.Fatalf("generated files should not contain tabs")
	}
}

func TestCreateUsesPublicDocumentRootForLaravel(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()

	created, err := Create(CreateOptions{
		Name:        "laravel-demo",
		Preset:      "laravel",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	nginxContents, err := os.ReadFile(created.NginxConfigPath)
	if err != nil {
		t.Fatalf("read nginx config: %v", err)
	}

	if !strings.Contains(string(nginxContents), "root /var/www/html/public;") {
		t.Fatalf("laravel preset should use /public document root: %s", string(nginxContents))
	}

	envContents, err := os.ReadFile(filepath.Join(projectRoot, ".env"))
	if err != nil {
		t.Fatalf("read laravel env: %v", err)
	}

	envText := string(envContents)
	if !strings.Contains(envText, "DB_HOST=\"db\"") || !strings.Contains(envText, "DB_DATABASE=\"laravel_demo\"") {
		t.Fatalf("laravel env was not synced: %s", envText)
	}
}

func TestCreateRejectsNonEmptyOutputWithoutForce(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	outputDir := filepath.Join(projectRoot, "existing")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir output dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(outputDir, "manual.txt"), []byte("keep me"), 0o644); err != nil {
		t.Fatalf("seed output dir: %v", err)
	}

	_, err := Create(CreateOptions{
		Name:        "blocked-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		OutputDir:   outputDir,
		Installer:   stubApplicationInstaller{},
	})
	if err == nil {
		t.Fatal("expected create to fail when output directory is not empty")
	}
}

func TestCreateAcceptsPHP74(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()

	created, err := Create(CreateOptions{
		Name:        "legacy-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		PHPVersion:  "7.4",
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	if created.Manifest.Runtime.PHPVersion != "7.4" {
		t.Fatalf("expected PHP 7.4, got %s", created.Manifest.Runtime.PHPVersion)
	}
}

func TestCreateCanEnableOptionalTooling(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()

	created, err := Create(CreateOptions{
		Name:           "tooling-demo",
		Preset:         "custom",
		ProjectRoot:    projectRoot,
		AdminerEnabled: true,
		MailpitEnabled: true,
		XdebugEnabled:  true,
		Installer:      stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	if created.XdebugDirPath == "" {
		t.Fatal("expected xdebug files to be generated")
	}

	if created.XdebugDirPath != filepath.Join(created.Manifest.Storage.BasePath, "php") {
		t.Fatalf("expected xdebug files to live in the generated PHP build context, got %s", created.XdebugDirPath)
	}

	if created.AdminerDirPath == "" {
		t.Fatal("expected adminer files to be generated")
	}

	composeContents, err := os.ReadFile(created.ComposePath)
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}

	composeText := string(composeContents)
	if !strings.Contains(composeText, "container_name: elk-tooling-demo-adminer") {
		t.Fatalf("compose does not include adminer: %s", composeText)
	}

	if !strings.Contains(composeText, "ELK_ADMINER_DB_NAME: \"tooling_demo\"") {
		t.Fatalf("compose does not include adminer database settings: %s", composeText)
	}

	if !strings.Contains(composeText, "container_name: elk-tooling-demo-mailpit") {
		t.Fatalf("compose does not include mailpit: %s", composeText)
	}

	if !strings.Contains(composeText, "host.docker.internal:host-gateway") {
		t.Fatalf("compose does not include xdebug host gateway: %s", composeText)
	}

	phpDockerfileContents, err := os.ReadFile(filepath.Join(created.Manifest.Storage.BasePath, "php", "Dockerfile"))
	if err != nil {
		t.Fatalf("read PHP Dockerfile: %v", err)
	}

	phpDockerfileText := string(phpDockerfileContents)
	if !strings.Contains(phpDockerfileText, "install-php-extensions") || !strings.Contains(phpDockerfileText, "imagick") || !strings.Contains(phpDockerfileText, "xdebug") {
		t.Fatalf("expected PHP Dockerfile to install imagick and xdebug: %s", phpDockerfileText)
	}

	if !strings.Contains(phpDockerfileText, "ENTRYPOINT [\"elk-local-php-entrypoint\"]") {
		t.Fatalf("expected PHP Dockerfile to use the generated entrypoint: %s", phpDockerfileText)
	}

	if !strings.Contains(phpDockerfileText, "CMD [\"php-fpm\"]") {
		t.Fatalf("expected PHP Dockerfile to preserve the php-fpm runtime command: %s", phpDockerfileText)
	}

	xdebugINIContents, err := os.ReadFile(filepath.Join(created.XdebugDirPath, "xdebug.ini"))
	if err != nil {
		t.Fatalf("read xdebug ini: %v", err)
	}

	if !strings.Contains(string(xdebugINIContents), "xdebug.client_port=9003") {
		t.Fatalf("unexpected xdebug ini: %s", string(xdebugINIContents))
	}

	launchContents, err := os.ReadFile(filepath.Join(projectRoot, ".vscode", "launch.json"))
	if err != nil {
		t.Fatalf("read VS Code launch config: %v", err)
	}

	launchText := string(launchContents)
	if !strings.Contains(launchText, "Listen for Xdebug 3.0 (Local)") || !strings.Contains(launchText, "\"port\": 9003") {
		t.Fatalf("expected VS Code launch config for Xdebug 3: %s", launchText)
	}

	if !strings.Contains(launchText, "Launch currently open script") {
		t.Fatalf("expected VS Code launch config to include launch script entry: %s", launchText)
	}

	adminerIndexContents, err := os.ReadFile(filepath.Join(created.AdminerDirPath, "index.php"))
	if err != nil {
		t.Fatalf("read adminer index: %v", err)
	}

	if !strings.Contains(string(adminerIndexContents), "Connecting to the ELK-Local database") {
		t.Fatalf("expected adminer auto login markers: %s", string(adminerIndexContents))
	}

	adminerIndexText := string(adminerIndexContents)
	if strings.Contains(adminerIndexText, "<form") {
		t.Fatalf("adminer index should not emit a nested form: %s", adminerIndexText)
	}

	if !strings.Contains(adminerIndexText, "document.currentScript.closest('form').submit();") {
		t.Fatalf("expected adminer index to submit the surrounding login form: %s", adminerIndexText)
	}
}

func TestCreateUsesInstalledDefaultsWhenConfigExists(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("ELK_LOCAL_HOME", "")
	t.Setenv("ELK_LOCAL_ENVIRONMENTS_DIR", "")
	t.Setenv("ELK_LOCAL_BACKUPS_DIR", "")

	configDir := filepath.Join(homeDir, ".elk-local")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}

	configContents := []byte("environmentsDir: ~/managed-envs\nbackupsDir: ~/managed-backups\n")
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), configContents, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	created, err := Create(CreateOptions{
		Name:      "installed-defaults-demo",
		Preset:    "custom",
		Installer: stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	expectedProjectRoot := filepath.Join(homeDir, "managed-envs", "installed-defaults-demo")
	if created.Manifest.Project.Root != expectedProjectRoot {
		t.Fatalf("unexpected project root: %s", created.Manifest.Project.Root)
	}

	expectedStoragePath := filepath.Join(homeDir, ".elk-local", "environments", "installed-defaults-demo")
	if created.Manifest.Storage.BasePath != expectedStoragePath {
		t.Fatalf("unexpected storage path: %s", created.Manifest.Storage.BasePath)
	}

	expectedBackupsPath := filepath.Join(homeDir, "managed-backups", "installed-defaults-demo")
	if created.Manifest.Storage.BackupsPath != expectedBackupsPath {
		t.Fatalf("unexpected backups path: %s", created.Manifest.Storage.BackupsPath)
	}

	if _, err := os.Stat(expectedProjectRoot); err != nil {
		t.Fatalf("stat project root: %v", err)
	}
	if _, err := os.Stat(expectedBackupsPath); err != nil {
		t.Fatalf("stat backups path: %v", err)
	}
	if _, err := os.Stat(created.ManifestPath); err != nil {
		t.Fatalf("stat manifest path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(expectedStoragePath, "compose.yaml")); err != nil {
		t.Fatalf("stat compose path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(expectedProjectRoot, ".env.local")); err != nil {
		t.Fatalf("stat synced app config: %v", err)
	}
}
