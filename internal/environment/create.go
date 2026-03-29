package environment

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"text/template"

	"elk-local/internal/config"

	"gopkg.in/yaml.v3"
)

type CreateOptions struct {
	Name                 string
	Preset               string
	ApplicationVersion   string
	ProjectRoot          string
	OutputDir            string
	PHPVersion           string
	WebServer            string
	DatabaseEngine       string
	DatabaseVersion      string
	DatabaseName         string
	DatabaseUser         string
	DatabasePassword     string
	DatabaseRootPassword string
	AdminerEnabled       bool
	AdminerPort          int
	MailpitEnabled       bool
	MailpitHTTPPort      int
	MailpitSMTPPort      int
	XdebugEnabled        bool
	XdebugHost           string
	XdebugPort           int
	HTTPPort             int
	DatabasePort         int
	Force                bool
	Installer            ApplicationInstaller
}

type CreatedEnvironment struct {
	Manifest        Manifest
	ManifestPath    string
	ComposePath     string
	NginxConfigPath string
	AdminerDirPath  string
	XdebugDirPath   string
	AppConfigPaths  []string
}

func Create(options CreateOptions) (*CreatedEnvironment, error) {
	presetName := strings.ToLower(strings.TrimSpace(options.Preset))
	if presetName == "" {
		presetName = "wordpress"
	}

	preset, err := GetPreset(presetName)
	if err != nil {
		return nil, err
	}

	name := strings.ToLower(strings.TrimSpace(options.Name))
	if name == "" {
		return nil, fmt.Errorf("environment name is required")
	}

	projectRoot, err := resolveCreateProjectRoot(options.ProjectRoot, name)
	if err != nil {
		return nil, err
	}

	outputDir, err := resolveOutputDir(projectRoot, name, options.OutputDir)
	if err != nil {
		return nil, err
	}

	backupsDir, err := resolveBackupsDir(name, outputDir)
	if err != nil {
		return nil, err
	}

	manifest := Manifest{
		APIVersion: "elk.dev/v1alpha1",
		Kind:       "Environment",
		Name:       name,
		Preset:     preset.Name,
		Application: Application{
			Name: preset.ApplicationName,
		},
		Project: Project{
			Type: preset.ProjectType,
			Root: projectRoot,
		},
		Runtime: Runtime{
			PHPVersion: firstNonEmpty(options.PHPVersion, preset.PHPVersion),
			WebServer:  firstNonEmpty(options.WebServer, preset.WebServer),
			Database: Database{
				Engine:       firstNonEmpty(options.DatabaseEngine, preset.DatabaseEngine),
				Version:      firstNonEmpty(options.DatabaseVersion, DefaultDatabaseVersion(firstNonEmpty(options.DatabaseEngine, preset.DatabaseEngine)), preset.DatabaseVersion),
				Name:         firstNonEmpty(options.DatabaseName, DatabaseName(name)),
				User:         firstNonEmpty(options.DatabaseUser, "elk"),
				Password:     firstNonEmpty(options.DatabasePassword, "elk"),
				RootPassword: firstNonEmpty(options.DatabaseRootPassword, "elkroot"),
			},
		},
		Tooling: Tooling{
			Adminer: Adminer{
				Enabled: options.AdminerEnabled,
				Port:    choosePort(options.AdminerPort, DefaultAdminerPort(name)),
			},
			Mailpit: Mailpit{
				Enabled:  options.MailpitEnabled,
				HTTPPort: choosePort(options.MailpitHTTPPort, DefaultMailpitHTTPPort(name)),
				SMTPPort: choosePort(options.MailpitSMTPPort, DefaultMailpitSMTPPort(name)),
			},
			Xdebug: Xdebug{
				Enabled:    options.XdebugEnabled,
				ClientHost: firstNonEmpty(options.XdebugHost, DefaultXdebugClientHost()),
				ClientPort: choosePort(options.XdebugPort, DefaultXdebugClientPort()),
				Mode:       DefaultXdebugMode(),
				IDEKey:     DefaultXdebugIDEKey(),
			},
		},
		Network: Network{
			HTTPPort:     choosePort(options.HTTPPort, DefaultHTTPPort(name)),
			DatabasePort: choosePort(options.DatabasePort, DefaultDatabasePort(name)),
		},
		Storage: Storage{
			BasePath:    outputDir,
			BackupsPath: backupsDir,
		},
		Compose: Compose{
			ProjectName: ComposeProjectName(name),
			NamePrefix:  ComposeNamePrefix(name),
			File:        filepath.Join(outputDir, "compose.yaml"),
		},
	}

	manifest.Runtime.WebServer = strings.ToLower(manifest.Runtime.WebServer)
	manifest.Runtime.Database.Engine = strings.ToLower(manifest.Runtime.Database.Engine)
	normalizeManifest(&manifest)

	if err := ValidateManifest(manifest); err != nil {
		return nil, err
	}

	if err := ensureWritableOutputDir(outputDir, options.Force); err != nil {
		return nil, err
	}

	installer := options.Installer
	if installer == nil {
		installer = DefaultApplicationInstaller{}
	}

	requestedAppVersion := firstNonEmpty(options.ApplicationVersion, preset.DefaultAppVersion)
	installedApplication, err := installer.Install(ApplicationInstallRequest{
		Preset:           preset,
		ProjectRoot:      projectRoot,
		RequestedVersion: requestedAppVersion,
	})
	if err != nil {
		return nil, err
	}

	if manifest.Application.Name != "" {
		manifest.Application.Name = firstNonEmpty(installedApplication.Name, manifest.Application.Name)
		manifest.Application.Version = firstNonEmpty(installedApplication.Version, requestedAppVersion)
	}

	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create backups directory: %w", err)
	}

	return writeGeneratedArtifacts(manifest)
}

func writeGeneratedArtifacts(manifest Manifest) (*CreatedEnvironment, error) {
	manifestPath := filepath.Join(manifest.Storage.BasePath, "environment.yaml")
	composePath := filepath.Join(manifest.Storage.BasePath, "compose.yaml")
	manifest.Compose.File = composePath
	normalizeManifest(&manifest)

	manifestContents, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}

	rendered, err := renderArtifacts(manifest)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(manifestPath, manifestContents, 0o644); err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}

	if err := os.WriteFile(composePath, []byte(rendered.Compose), 0o644); err != nil {
		return nil, fmt.Errorf("write compose file: %w", err)
	}

	created := &CreatedEnvironment{
		Manifest:     manifest,
		ManifestPath: manifestPath,
		ComposePath:  composePath,
	}

	phpDir := filepath.Join(manifest.Storage.BasePath, "php")
	nginxDir := filepath.Join(manifest.Storage.BasePath, "nginx")
	adminerDir := filepath.Join(manifest.Storage.BasePath, "adminer")
	xdebugINIPath := filepath.Join(phpDir, "xdebug.ini")

	if err := os.MkdirAll(phpDir, 0o755); err != nil {
		return nil, fmt.Errorf("create php directory: %w", err)
	}

	phpDockerfilePath := filepath.Join(phpDir, "Dockerfile")
	if err := os.WriteFile(phpDockerfilePath, []byte(rendered.PHPDockerfile), 0o644); err != nil {
		return nil, fmt.Errorf("write PHP Dockerfile: %w", err)
	}

	phpEntrypointPath := filepath.Join(phpDir, "docker-entrypoint.sh")
	if err := os.WriteFile(phpEntrypointPath, []byte(rendered.PHPEntrypoint), 0o755); err != nil {
		return nil, fmt.Errorf("write PHP entrypoint: %w", err)
	}

	if rendered.NginxConfig != "" {
		if err := os.MkdirAll(nginxDir, 0o755); err != nil {
			return nil, fmt.Errorf("create nginx directory: %w", err)
		}

		nginxPath := filepath.Join(nginxDir, "default.conf")
		if err := os.WriteFile(nginxPath, []byte(rendered.NginxConfig), 0o644); err != nil {
			return nil, fmt.Errorf("write nginx config: %w", err)
		}

		created.NginxConfigPath = nginxPath
	} else if err := os.RemoveAll(nginxDir); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove nginx directory: %w", err)
	}

	if rendered.AdminerDockerfile != "" {
		if err := os.MkdirAll(adminerDir, 0o755); err != nil {
			return nil, fmt.Errorf("create adminer directory: %w", err)
		}

		dockerfilePath := filepath.Join(adminerDir, "Dockerfile")
		if err := os.WriteFile(dockerfilePath, []byte(rendered.AdminerDockerfile), 0o644); err != nil {
			return nil, fmt.Errorf("write adminer Dockerfile: %w", err)
		}

		indexPath := filepath.Join(adminerDir, "index.php")
		if err := os.WriteFile(indexPath, []byte(rendered.AdminerIndexPHP), 0o644); err != nil {
			return nil, fmt.Errorf("write adminer index: %w", err)
		}

		created.AdminerDirPath = adminerDir
	} else if err := os.RemoveAll(adminerDir); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove adminer directory: %w", err)
	}

	if rendered.XdebugINI != "" {
		if err := os.WriteFile(xdebugINIPath, []byte(rendered.XdebugINI), 0o644); err != nil {
			return nil, fmt.Errorf("write xdebug ini: %w", err)
		}

		created.XdebugDirPath = phpDir
	} else if err := os.Remove(xdebugINIPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove xdebug ini: %w", err)
	}

	appConfigPaths, err := syncApplicationConfig(manifest)
	if err != nil {
		return nil, err
	}

	created.AppConfigPaths = appConfigPaths

	return created, nil
}

func normalizeManifest(manifest *Manifest) {
	if manifest.Application.Name == "" {
		if preset, err := GetPreset(manifest.Preset); err == nil {
			manifest.Application.Name = preset.ApplicationName
		}
	}

	if manifest.Compose.ProjectName == "" {
		manifest.Compose.ProjectName = ComposeProjectName(manifest.Name)
	}

	if manifest.Compose.NamePrefix == "" {
		manifest.Compose.NamePrefix = ComposeNamePrefix(manifest.Name)
	}

	if manifest.Compose.File == "" && manifest.Storage.BasePath != "" {
		manifest.Compose.File = filepath.Join(manifest.Storage.BasePath, "compose.yaml")
	}

	if manifest.Storage.BackupsPath == "" && manifest.Storage.BasePath != "" {
		manifest.Storage.BackupsPath = filepath.Join(manifest.Storage.BasePath, "backups")
	}

	if manifest.Tooling.Adminer.Enabled && manifest.Tooling.Adminer.Port == 0 {
		manifest.Tooling.Adminer.Port = DefaultAdminerPort(manifest.Name)
	}

	if manifest.Runtime.Database.Name == "" {
		manifest.Runtime.Database.Name = DatabaseName(manifest.Name)
	}

	if manifest.Runtime.Database.User == "" {
		manifest.Runtime.Database.User = "elk"
	}

	if manifest.Runtime.Database.Password == "" {
		manifest.Runtime.Database.Password = "elk"
	}

	if manifest.Runtime.Database.RootPassword == "" {
		manifest.Runtime.Database.RootPassword = "elkroot"
	}

	if manifest.Tooling.Mailpit.Enabled {
		if manifest.Tooling.Mailpit.HTTPPort == 0 {
			manifest.Tooling.Mailpit.HTTPPort = DefaultMailpitHTTPPort(manifest.Name)
		}

		if manifest.Tooling.Mailpit.SMTPPort == 0 {
			manifest.Tooling.Mailpit.SMTPPort = DefaultMailpitSMTPPort(manifest.Name)
		}
	}

	if manifest.Tooling.Xdebug.Enabled {
		if manifest.Tooling.Xdebug.ClientHost == "" {
			manifest.Tooling.Xdebug.ClientHost = DefaultXdebugClientHost()
		}

		if manifest.Tooling.Xdebug.ClientPort == 0 {
			manifest.Tooling.Xdebug.ClientPort = DefaultXdebugClientPort()
		}

		if manifest.Tooling.Xdebug.Mode == "" {
			manifest.Tooling.Xdebug.Mode = DefaultXdebugMode()
		}

		if manifest.Tooling.Xdebug.IDEKey == "" {
			manifest.Tooling.Xdebug.IDEKey = DefaultXdebugIDEKey()
		}
	}
}

func resolveProjectRoot(projectRoot string) (string, error) {
	resolvedPath, err := config.ResolveRegistryRoot(projectRoot)
	if err != nil {
		return "", err
	}

	return filepath.Clean(resolvedPath), nil
}

func resolveCreateProjectRoot(projectRoot string, name string) (string, error) {
	targetPath := strings.TrimSpace(projectRoot)
	if targetPath == "" {
		configuredPath, configured, err := config.DefaultEnvironmentProjectRoot(name)
		if err != nil {
			return "", err
		}
		if configured {
			targetPath = configuredPath
		} else {
			currentDir, err := os.Getwd()
			if err != nil {
				return "", fmt.Errorf("resolve current working directory: %w", err)
			}

			targetPath = currentDir
		}
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}

	projectRoot = filepath.Clean(absPath)
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		return "", fmt.Errorf("create project root: %w", err)
	}

	info, err := os.Stat(projectRoot)
	if err != nil {
		return "", fmt.Errorf("stat project root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project root must be a directory")
	}

	return projectRoot, nil
}

func resolveOutputDir(projectRoot string, name string, outputDir string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("environment name is required")
	}

	if strings.TrimSpace(outputDir) == "" {
		configuredOutputDir, configured, err := config.DefaultManifestStorageDir(name)
		if err != nil {
			return "", err
		}
		if configured {
			outputDir = configuredOutputDir
		} else {
			outputDir = filepath.Join(projectRoot, ".elk-local", "environments", name)
		}
	}

	absPath, err := filepath.Abs(outputDir)
	if err != nil {
		return "", fmt.Errorf("resolve output directory: %w", err)
	}

	return filepath.Clean(absPath), nil
}

func resolveBackupsDir(name string, outputDir string) (string, error) {
	configuredBackupsDir, configured, err := config.DefaultBackupStorageDir(name)
	if err != nil {
		return "", err
	}
	if configured {
		return configuredBackupsDir, nil
	}

	return filepath.Join(outputDir, "backups"), nil
}

func ensureWritableOutputDir(outputDir string, force bool) error {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(outputDir, 0o755)
		}

		return fmt.Errorf("inspect output directory: %w", err)
	}

	if len(entries) > 0 && !force {
		return fmt.Errorf("output directory %s already exists and is not empty; use --force to overwrite generated files", outputDir)
	}

	return nil
}

type renderedArtifacts struct {
	Compose           string
	NginxConfig       string
	PHPDockerfile     string
	PHPEntrypoint     string
	AdminerDockerfile string
	AdminerIndexPHP   string
	XdebugINI         string
}

func renderArtifacts(manifest Manifest) (renderedArtifacts, error) {
	templateData := composeTemplateData{
		Manifest:            manifest,
		DatabaseImage:       databaseImage(manifest.Runtime.Database),
		DatabaseDataPath:    databaseDataPath(manifest.Runtime.Database.Engine),
		DatabaseEnvPrefix:   databaseEnvPrefix(manifest.Runtime.Database.Engine),
		DocumentRoot:        documentRoot(manifest.Project.Type),
		AdminerBuildContext: filepath.Join(manifest.Storage.BasePath, "adminer"),
		PHPBuildContext:     filepath.Join(manifest.Storage.BasePath, "php"),
		ProjectRootForYAML:  manifest.Project.Root,
		NginxConfigHostPath: filepath.Join(manifest.Storage.BasePath, "nginx", "default.conf"),
		XdebugConfig:        fmt.Sprintf("client_host=%s client_port=%d idekey=%s", manifest.Tooling.Xdebug.ClientHost, manifest.Tooling.Xdebug.ClientPort, manifest.Tooling.Xdebug.IDEKey),
		UsesApache:          manifest.Runtime.WebServer == "apache",
		UsesNginx:           manifest.Runtime.WebServer == "nginx",
		WebImage:            webImage(manifest.Runtime),
		AppImage:            appImage(manifest.Runtime),
	}
	templateData.HostUID, templateData.HostGID = currentHostIdentity()

	composeTemplateText := apacheComposeTemplate
	nginxTemplateText := ""
	if manifest.Runtime.WebServer == "nginx" {
		composeTemplateText = nginxComposeTemplate
		nginxTemplateText = defaultNginxConfig
	}

	composeContents, err := executeTemplate(composeTemplateText, templateData)
	if err != nil {
		return renderedArtifacts{}, fmt.Errorf("render compose file: %w", err)
	}

	phpDockerfileContents, err := executeTemplate(phpDockerfileTemplate, templateData)
	if err != nil {
		return renderedArtifacts{}, fmt.Errorf("render PHP Dockerfile: %w", err)
	}

	phpEntrypointContents, err := executeTemplate(phpEntrypointTemplate, templateData)
	if err != nil {
		return renderedArtifacts{}, fmt.Errorf("render PHP entrypoint: %w", err)
	}

	rendered := renderedArtifacts{Compose: composeContents, PHPDockerfile: phpDockerfileContents, PHPEntrypoint: phpEntrypointContents}

	if nginxTemplateText != "" {
		nginxContents, err := executeTemplate(nginxTemplateText, templateData)
		if err != nil {
			return renderedArtifacts{}, fmt.Errorf("render nginx config: %w", err)
		}

		rendered.NginxConfig = nginxContents
	}

	if manifest.Tooling.Xdebug.Enabled {
		xdebugINIContents, err := executeTemplate(xdebugINIConfigTemplate, templateData)
		if err != nil {
			return renderedArtifacts{}, fmt.Errorf("render xdebug ini: %w", err)
		}

		rendered.XdebugINI = xdebugINIContents
	}

	if manifest.Tooling.Adminer.Enabled {
		dockerfileContents, err := executeTemplate(adminerDockerfileTemplate, templateData)
		if err != nil {
			return renderedArtifacts{}, fmt.Errorf("render adminer Dockerfile: %w", err)
		}

		indexContents, err := executeTemplate(adminerIndexTemplate, templateData)
		if err != nil {
			return renderedArtifacts{}, fmt.Errorf("render adminer index: %w", err)
		}

		rendered.AdminerDockerfile = dockerfileContents
		rendered.AdminerIndexPHP = indexContents
	}

	return rendered, nil
}

type composeTemplateData struct {
	Manifest            Manifest
	DatabaseImage       string
	DatabaseDataPath    string
	DatabaseEnvPrefix   string
	DocumentRoot        string
	AdminerBuildContext string
	PHPBuildContext     string
	ProjectRootForYAML  string
	NginxConfigHostPath string
	XdebugConfig        string
	UsesApache          bool
	UsesNginx           bool
	WebImage            string
	AppImage            string
	HostUID             string
	HostGID             string
}

func executeTemplate(templateText string, data composeTemplateData) (string, error) {
	tmpl, err := template.New("compose").Parse(templateText)
	if err != nil {
		return "", err
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return "", err
	}

	return strings.ReplaceAll(output.String(), "\t", "    "), nil
}

func databaseImage(database Database) string {
	return fmt.Sprintf("%s:%s", database.Engine, database.Version)
}

func databaseDataPath(engine string) string {
	if engine == "mariadb" {
		return "/var/lib/mysql"
	}

	return "/var/lib/mysql"
}

func databaseEnvPrefix(engine string) string {
	if engine == "mariadb" {
		return "MARIADB"
	}

	return "MYSQL"
}

func webImage(runtime Runtime) string {
	if runtime.WebServer == "apache" {
		return fmt.Sprintf("php:%s-apache", runtime.PHPVersion)
	}

	return "nginx:1.27-alpine"
}

func appImage(runtime Runtime) string {
	if runtime.WebServer == "nginx" {
		return fmt.Sprintf("php:%s-fpm", runtime.PHPVersion)
	}

	return ""
}

func currentHostIdentity() (string, string) {
	currentUser, err := user.Current()
	if err != nil {
		return "", ""
	}

	if !numericID(currentUser.Uid) || !numericID(currentUser.Gid) {
		return "", ""
	}

	return currentUser.Uid, currentUser.Gid
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func choosePort(value int, fallback int) int {
	if value > 0 {
		return value
	}

	return fallback
}

func documentRoot(projectType string) string {
	switch strings.ToLower(projectType) {
	case "laravel", "symfony":
		return "/var/www/html/public"
	default:
		return "/var/www/html"
	}
}

const apacheComposeTemplate = `name: {{ .Manifest.Compose.ProjectName }}
services:
	web:
		build:
			context: {{ printf "%q" .PHPBuildContext }}
			dockerfile: Dockerfile
			args:
				BASE_IMAGE: {{ .WebImage }}
		environment:
			ELK_HTTP_PORT: {{ printf "%q" (printf "%d" .Manifest.Network.HTTPPort) }}
			{{- if .HostUID }}
			ELK_HOST_UID: {{ printf "%q" .HostUID }}
			{{- end }}
			{{- if .HostGID }}
			ELK_HOST_GID: {{ printf "%q" .HostGID }}
			{{- end }}
			{{- if .Manifest.Tooling.Xdebug.Enabled }}
			XDEBUG_MODE: {{ .Manifest.Tooling.Xdebug.Mode }}
			XDEBUG_CONFIG: {{ printf "%q" .XdebugConfig }}
			{{- end }}
		{{- if .Manifest.Tooling.Xdebug.Enabled }}
		extra_hosts:
			- "host.docker.internal:host-gateway"
		{{- end }}
		container_name: {{ .Manifest.Compose.NamePrefix }}-web
		depends_on:
			- db
		ports:
			- "{{ .Manifest.Network.HTTPPort }}:{{ .Manifest.Network.HTTPPort }}"
		volumes:
			- {{ printf "%q" (printf "%s:/var/www/html" .ProjectRootForYAML) }}
	db:
		image: {{ .DatabaseImage }}
		container_name: {{ .Manifest.Compose.NamePrefix }}-db
		ports:
			- "{{ .Manifest.Network.DatabasePort }}:3306"
		environment:
			{{ .DatabaseEnvPrefix }}_DATABASE: {{ printf "%q" .Manifest.Runtime.Database.Name }}
			{{ .DatabaseEnvPrefix }}_USER: {{ printf "%q" .Manifest.Runtime.Database.User }}
			{{ .DatabaseEnvPrefix }}_PASSWORD: {{ printf "%q" .Manifest.Runtime.Database.Password }}
			{{ .DatabaseEnvPrefix }}_ROOT_PASSWORD: {{ printf "%q" .Manifest.Runtime.Database.RootPassword }}
		volumes:
			- db_data:{{ .DatabaseDataPath }}
	{{- if .Manifest.Tooling.Adminer.Enabled }}
	adminer:
		build:
			context: {{ printf "%q" .AdminerBuildContext }}
			dockerfile: Dockerfile
		container_name: {{ .Manifest.Compose.NamePrefix }}-adminer
		depends_on:
			- db
		ports:
			- "{{ .Manifest.Tooling.Adminer.Port }}:8080"
		environment:
			ELK_ADMINER_DB_HOST: db
			ELK_ADMINER_DB_NAME: {{ printf "%q" .Manifest.Runtime.Database.Name }}
			ELK_ADMINER_DB_USER: {{ printf "%q" .Manifest.Runtime.Database.User }}
			ELK_ADMINER_DB_PASSWORD: {{ printf "%q" .Manifest.Runtime.Database.Password }}
	{{- end }}
	{{- if .Manifest.Tooling.Mailpit.Enabled }}
	mailpit:
		image: axllent/mailpit:v1.27
		container_name: {{ .Manifest.Compose.NamePrefix }}-mailpit
		ports:
			- "{{ .Manifest.Tooling.Mailpit.HTTPPort }}:8025"
			- "{{ .Manifest.Tooling.Mailpit.SMTPPort }}:1025"
	{{- end }}

volumes:
	db_data:
		name: {{ .Manifest.Compose.NamePrefix }}-db-data
`

const nginxComposeTemplate = `name: {{ .Manifest.Compose.ProjectName }}
services:
	app:
		build:
			context: {{ printf "%q" .PHPBuildContext }}
			dockerfile: Dockerfile
			args:
				BASE_IMAGE: {{ .AppImage }}
		{{- if or .HostUID .HostGID .Manifest.Tooling.Xdebug.Enabled }}
		environment:
			{{- if .HostUID }}
			ELK_HOST_UID: {{ printf "%q" .HostUID }}
			{{- end }}
			{{- if .HostGID }}
			ELK_HOST_GID: {{ printf "%q" .HostGID }}
			{{- end }}
			{{- if .Manifest.Tooling.Xdebug.Enabled }}
			XDEBUG_MODE: {{ .Manifest.Tooling.Xdebug.Mode }}
			XDEBUG_CONFIG: {{ printf "%q" .XdebugConfig }}
			{{- end }}
		{{- end }}
		{{- if .Manifest.Tooling.Xdebug.Enabled }}
		extra_hosts:
			- "host.docker.internal:host-gateway"
		{{- end }}
		container_name: {{ .Manifest.Compose.NamePrefix }}-app
		working_dir: /var/www/html
		volumes:
			- {{ printf "%q" (printf "%s:/var/www/html" .ProjectRootForYAML) }}
	web:
		image: {{ .WebImage }}
		container_name: {{ .Manifest.Compose.NamePrefix }}-web
		depends_on:
			- app
		ports:
			- "{{ .Manifest.Network.HTTPPort }}:80"
		volumes:
			- {{ printf "%q" (printf "%s:/var/www/html:ro" .ProjectRootForYAML) }}
			- {{ printf "%q" (printf "%s:/etc/nginx/conf.d/default.conf:ro" .NginxConfigHostPath) }}
	db:
		image: {{ .DatabaseImage }}
		container_name: {{ .Manifest.Compose.NamePrefix }}-db
		ports:
			- "{{ .Manifest.Network.DatabasePort }}:3306"
		environment:
			{{ .DatabaseEnvPrefix }}_DATABASE: {{ printf "%q" .Manifest.Runtime.Database.Name }}
			{{ .DatabaseEnvPrefix }}_USER: {{ printf "%q" .Manifest.Runtime.Database.User }}
			{{ .DatabaseEnvPrefix }}_PASSWORD: {{ printf "%q" .Manifest.Runtime.Database.Password }}
			{{ .DatabaseEnvPrefix }}_ROOT_PASSWORD: {{ printf "%q" .Manifest.Runtime.Database.RootPassword }}
		volumes:
			- db_data:{{ .DatabaseDataPath }}
	{{- if .Manifest.Tooling.Adminer.Enabled }}
	adminer:
		build:
			context: {{ printf "%q" .AdminerBuildContext }}
			dockerfile: Dockerfile
		container_name: {{ .Manifest.Compose.NamePrefix }}-adminer
		depends_on:
			- db
		ports:
			- "{{ .Manifest.Tooling.Adminer.Port }}:8080"
		environment:
			ELK_ADMINER_DB_HOST: db
			ELK_ADMINER_DB_NAME: {{ printf "%q" .Manifest.Runtime.Database.Name }}
			ELK_ADMINER_DB_USER: {{ printf "%q" .Manifest.Runtime.Database.User }}
			ELK_ADMINER_DB_PASSWORD: {{ printf "%q" .Manifest.Runtime.Database.Password }}
	{{- end }}
	{{- if .Manifest.Tooling.Mailpit.Enabled }}
	mailpit:
		image: axllent/mailpit:v1.27
		container_name: {{ .Manifest.Compose.NamePrefix }}-mailpit
		ports:
			- "{{ .Manifest.Tooling.Mailpit.HTTPPort }}:8025"
			- "{{ .Manifest.Tooling.Mailpit.SMTPPort }}:1025"
	{{- end }}

volumes:
	db_data:
		name: {{ .Manifest.Compose.NamePrefix }}-db-data
`

const defaultNginxConfig = `server {
		listen 80;
		server_name _;
		root {{ .DocumentRoot }};
		index index.php index.html;

		location / {
				try_files $uri $uri/ /index.php?$query_string;
		}

		location ~ \.php$ {
				include fastcgi_params;
				fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
				fastcgi_pass app:9000;
		}
}
`

const phpDockerfileTemplate = `ARG BASE_IMAGE={{ if .UsesApache }}{{ .WebImage }}{{ else }}{{ .AppImage }}{{ end }}
FROM mlocati/php-extension-installer:2 AS php_extension_installer
FROM ${BASE_IMAGE}

COPY --from=php_extension_installer /usr/bin/install-php-extensions /usr/local/bin/
COPY docker-entrypoint.sh /usr/local/bin/elk-local-php-entrypoint

RUN set -eux; \
	apt-get update; \
	apt-get install -y --no-install-recommends \
		ghostscript \
		unzip \
		zip; \
	rm -rf /var/lib/apt/lists/*; \
	chmod +x /usr/local/bin/elk-local-php-entrypoint; \
	install-php-extensions \
		curl \
		dom \
		exif \
		gd \
		imagick \
		intl \
		mbstring \
		mysqli \
		opcache \
		pdo_mysql \
		simplexml \
		xml \
		xmlreader \
		xmlwriter \
		zip{{ if .Manifest.Tooling.Xdebug.Enabled }} \
		xdebug{{ end }}

ENTRYPOINT ["elk-local-php-entrypoint"]
CMD [{{ if .UsesApache }}"apache2-foreground"{{ else }}"php-fpm"{{ end }}]

{{- if .Manifest.Tooling.Xdebug.Enabled }}
COPY xdebug.ini /usr/local/etc/php/conf.d/zz-elk-xdebug.ini
{{- end }}
`

const phpEntrypointTemplate = `#!/usr/bin/env bash

set -euo pipefail

remap_www_data() {
	if [[ "$(id -u)" -ne 0 ]]; then
		return
	fi

	local host_uid="${ELK_HOST_UID:-}"
	local host_gid="${ELK_HOST_GID:-}"
	if [[ ! "$host_uid" =~ ^[0-9]+$ ]] || [[ ! "$host_gid" =~ ^[0-9]+$ ]]; then
		return
	fi

	if [[ "$(id -g www-data)" != "$host_gid" ]]; then
		groupmod -o -g "$host_gid" www-data
	fi

	if [[ "$(id -u www-data)" != "$host_uid" ]]; then
		usermod -o -u "$host_uid" -g "$host_gid" www-data
	fi

	chown -R www-data:www-data /var/run/apache2 /var/lock/apache2 /var/log/apache2 /var/run/php 2>/dev/null || true
}

configure_apache_port() {
	if [[ "$#" -eq 0 ]] || [[ "$1" != "apache2-foreground" ]]; then
		return
	fi

	local http_port="${ELK_HTTP_PORT:-}"
	if [[ ! "$http_port" =~ ^[0-9]+$ ]]; then
		return
	fi

	sed -ri "s/Listen 80/Listen ${http_port}/g" /etc/apache2/ports.conf
	sed -ri "s/<VirtualHost \*:80>/<VirtualHost *:${http_port}>/g" /etc/apache2/sites-available/000-default.conf
}

main() {
	remap_www_data
	configure_apache_port "$@"
	exec docker-php-entrypoint "$@"
}

main "$@"
`

const adminerDockerfileTemplate = `FROM adminer:5.4.2-standalone

COPY index.php /var/www/html/index.php
`

const adminerIndexTemplate = `<?php
function adminer_object() {
	class ElkLocalAdminer extends Adminer\Adminer {
		private function value($name, $fallback = '') {
			$value = getenv($name);
			if ($value === false || trim($value) === '') {
				return $fallback;
			}

			return $value;
		}

		function name() {
			return 'ELK-Local Adminer';
		}

		function credentials() {
			return array(
				$this->value('ELK_ADMINER_DB_HOST', 'db'),
				$this->value('ELK_ADMINER_DB_USER'),
				$this->value('ELK_ADMINER_DB_PASSWORD'),
			);
		}

		function database() {
			$database = $this->value('ELK_ADMINER_DB_NAME');
			if ($database === '') {
				return null;
			}

			return $database;
		}

		function login($login, $password) {
			return $login === $this->value('ELK_ADMINER_DB_USER') && $password === $this->value('ELK_ADMINER_DB_PASSWORD');
		}

		function loginForm() {
			$database = $this->database();

			echo Adminer\input_token();
			echo Adminer\input_hidden('auth[driver]', 'server');
			echo Adminer\input_hidden('auth[server]', $this->value('ELK_ADMINER_DB_HOST', 'db'));
			echo Adminer\input_hidden('auth[username]', $this->value('ELK_ADMINER_DB_USER'));
			echo Adminer\input_hidden('auth[password]', $this->value('ELK_ADMINER_DB_PASSWORD'));
			if ($database !== null) {
				echo Adminer\input_hidden('auth[db]', $database);
			}

			echo '<p class="jsonly">Connecting to the ELK-Local database...</p>';
			echo '<noscript><p><button type="submit">Connect to the ELK-Local database</button></p></noscript>';
			echo Adminer\script("document.currentScript.closest('form').submit();");
		}
	}

	return new ElkLocalAdminer();
}

include './adminer.php';
`

const xdebugINIConfigTemplate = `zend_extension=xdebug
xdebug.mode={{ .Manifest.Tooling.Xdebug.Mode }}
xdebug.start_with_request=yes
xdebug.client_host={{ .Manifest.Tooling.Xdebug.ClientHost }}
xdebug.client_port={{ .Manifest.Tooling.Xdebug.ClientPort }}
xdebug.idekey={{ .Manifest.Tooling.Xdebug.IDEKey }}
`
