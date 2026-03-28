package environment

import (
	"fmt"
	"hash/fnv"
	"path/filepath"
	"regexp"
	"strings"
)

var validNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var supportedPHPVersions = map[string]struct{}{
	"7.4": {},
	"8.0": {},
	"8.1": {},
	"8.2": {},
	"8.3": {},
	"8.4": {},
}

var supportedWebServers = map[string]struct{}{
	"apache": {},
	"nginx":  {},
}

var supportedDatabaseEngines = map[string]struct{}{
	"mysql":   {},
	"mariadb": {},
}

type Manifest struct {
	APIVersion  string      `yaml:"apiVersion"`
	Kind        string      `yaml:"kind"`
	Name        string      `yaml:"name"`
	Preset      string      `yaml:"preset"`
	Application Application `yaml:"application,omitempty"`
	Project     Project     `yaml:"project"`
	Runtime     Runtime     `yaml:"runtime"`
	Tooling     Tooling     `yaml:"tooling"`
	Network     Network     `yaml:"network"`
	Storage     Storage     `yaml:"storage"`
	Compose     Compose     `yaml:"compose"`
}

type Application struct {
	Name    string `yaml:"name,omitempty"`
	Version string `yaml:"version,omitempty"`
}

type Project struct {
	Type string `yaml:"type"`
	Root string `yaml:"root"`
}

type Runtime struct {
	PHPVersion string   `yaml:"phpVersion"`
	WebServer  string   `yaml:"webServer"`
	Database   Database `yaml:"database"`
}

type Database struct {
	Engine       string `yaml:"engine"`
	Version      string `yaml:"version"`
	Name         string `yaml:"name"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	RootPassword string `yaml:"rootPassword"`
}

type Tooling struct {
	Adminer Adminer `yaml:"adminer"`
	Mailpit Mailpit `yaml:"mailpit"`
	Xdebug  Xdebug  `yaml:"xdebug"`
}

type Adminer struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

type Mailpit struct {
	Enabled  bool `yaml:"enabled"`
	HTTPPort int  `yaml:"httpPort"`
	SMTPPort int  `yaml:"smtpPort"`
}

type Xdebug struct {
	Enabled    bool   `yaml:"enabled"`
	ClientHost string `yaml:"clientHost"`
	ClientPort int    `yaml:"clientPort"`
	Mode       string `yaml:"mode"`
	IDEKey     string `yaml:"ideKey"`
}

type Network struct {
	HTTPPort     int `yaml:"httpPort"`
	DatabasePort int `yaml:"databasePort"`
}

type Storage struct {
	BasePath    string `yaml:"basePath"`
	BackupsPath string `yaml:"backupsPath"`
}

type Compose struct {
	ProjectName string `yaml:"projectName"`
	NamePrefix  string `yaml:"namePrefix"`
	File        string `yaml:"file"`
}

func ValidateManifest(manifest Manifest) error {
	if !validNamePattern.MatchString(manifest.Name) {
		return fmt.Errorf("environment name %q must use lowercase letters, numbers, and hyphens", manifest.Name)
	}

	if _, err := GetPreset(manifest.Preset); err != nil {
		return err
	}

	if _, ok := supportedPHPVersions[manifest.Runtime.PHPVersion]; !ok {
		return fmt.Errorf("unsupported PHP version %q", manifest.Runtime.PHPVersion)
	}

	if _, ok := supportedWebServers[manifest.Runtime.WebServer]; !ok {
		return fmt.Errorf("unsupported web server %q", manifest.Runtime.WebServer)
	}

	if _, ok := supportedDatabaseEngines[manifest.Runtime.Database.Engine]; !ok {
		return fmt.Errorf("unsupported database engine %q", manifest.Runtime.Database.Engine)
	}

	if strings.TrimSpace(manifest.Runtime.Database.Name) == "" {
		return fmt.Errorf("database name is required")
	}

	if strings.TrimSpace(manifest.Runtime.Database.User) == "" {
		return fmt.Errorf("database user is required")
	}

	if strings.TrimSpace(manifest.Runtime.Database.Password) == "" {
		return fmt.Errorf("database password is required")
	}

	if strings.TrimSpace(manifest.Runtime.Database.RootPassword) == "" {
		return fmt.Errorf("database root password is required")
	}

	if manifest.Project.Root == "" || !filepath.IsAbs(manifest.Project.Root) {
		return fmt.Errorf("project root must be an absolute path")
	}

	if manifest.Storage.BasePath == "" || !filepath.IsAbs(manifest.Storage.BasePath) {
		return fmt.Errorf("storage base path must be an absolute path")
	}

	if manifest.Storage.BackupsPath == "" || !filepath.IsAbs(manifest.Storage.BackupsPath) {
		return fmt.Errorf("storage backups path must be an absolute path")
	}

	if manifest.Network.HTTPPort < 1 || manifest.Network.HTTPPort > 65535 {
		return fmt.Errorf("http port must be between 1 and 65535")
	}

	if manifest.Network.DatabasePort < 1 || manifest.Network.DatabasePort > 65535 {
		return fmt.Errorf("database port must be between 1 and 65535")
	}

	if manifest.Tooling.Adminer.Enabled && !isValidPort(manifest.Tooling.Adminer.Port) {
		return fmt.Errorf("adminer port must be between 1 and 65535")
	}

	if manifest.Tooling.Mailpit.Enabled {
		if !isValidPort(manifest.Tooling.Mailpit.HTTPPort) {
			return fmt.Errorf("mailpit http port must be between 1 and 65535")
		}

		if !isValidPort(manifest.Tooling.Mailpit.SMTPPort) {
			return fmt.Errorf("mailpit smtp port must be between 1 and 65535")
		}
	}

	if manifest.Tooling.Xdebug.Enabled {
		if strings.TrimSpace(manifest.Tooling.Xdebug.ClientHost) == "" {
			return fmt.Errorf("xdebug client host is required when xdebug is enabled")
		}

		if !isValidPort(manifest.Tooling.Xdebug.ClientPort) {
			return fmt.Errorf("xdebug client port must be between 1 and 65535")
		}
	}

	return nil
}

func DefaultHTTPPort(name string) int {
	return 20000 + stablePortOffset(name)
}

func DefaultDatabasePort(name string) int {
	return 30000 + stablePortOffset(name)
}

func DefaultAdminerPort(name string) int {
	return 40000 + stablePortOffset(name)
}

func DefaultMailpitHTTPPort(name string) int {
	return 45000 + stablePortOffset(name)
}

func DefaultMailpitSMTPPort(name string) int {
	return 55000 + stablePortOffset(name)
}

func DefaultXdebugClientHost() string {
	return "host.docker.internal"
}

func DefaultXdebugClientPort() int {
	return 9003
}

func DefaultXdebugMode() string {
	return "debug,develop"
}

func DefaultXdebugIDEKey() string {
	return "VSCODE"
}

func ComposeProjectName(name string) string {
	return ComposeNamePrefix(name)
}

func ComposeNamePrefix(name string) string {
	return "elk-" + name
}

func DatabaseName(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

func stablePortOffset(name string) int {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(name))

	return int(hash.Sum32() % 10000)
}

func isValidPort(port int) bool {
	return port >= 1 && port <= 65535
}
