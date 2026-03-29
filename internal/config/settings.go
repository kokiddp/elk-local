package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultWebUIPort = 4173

type Settings struct {
	EnvironmentsDir string `yaml:"environmentsDir"`
	BackupsDir      string `yaml:"backupsDir"`
	WebUIPort       int    `yaml:"webuiPort,omitempty"`
	ShellRC         string `yaml:"shellRC,omitempty"`
	DaemonAutoStart bool   `yaml:"daemonAutoStart,omitempty"`
}

func UserHomeDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}

	absolutePath, err := filepath.Abs(homeDir)
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}

	return filepath.Clean(absolutePath), nil
}

func HomeRoot() (string, error) {
	if override := strings.TrimSpace(os.Getenv("ELK_LOCAL_HOME")); override != "" {
		resolvedPath, err := resolvePath(override)
		if err != nil {
			return "", fmt.Errorf("resolve ELK_LOCAL_HOME: %w", err)
		}

		return resolvedPath, nil
	}

	homeDir, err := UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".elk-local"), nil
}

func ConfigPath() (string, error) {
	homeRoot, err := HomeRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeRoot, "config.yaml"), nil
}

func DefaultSettings() (Settings, error) {
	homeDir, err := UserHomeDir()
	if err != nil {
		return Settings{}, err
	}

	return Settings{
		EnvironmentsDir: filepath.Join(homeDir, "elk-local", "environments"),
		BackupsDir:      filepath.Join(homeDir, "elk-local", "backups"),
		WebUIPort:       DefaultWebUIPort,
	}, nil
}

func Load() (Settings, bool, error) {
	settings, err := DefaultSettings()
	if err != nil {
		return Settings{}, false, err
	}

	configPath, err := ConfigPath()
	if err != nil {
		return Settings{}, false, err
	}

	contents, readErr := os.ReadFile(configPath)
	configured := false
	if readErr == nil {
		configured = true

		var fileSettings Settings
		if err := yaml.Unmarshal(contents, &fileSettings); err != nil {
			return Settings{}, false, fmt.Errorf("decode config file: %w", err)
		}

		if strings.TrimSpace(fileSettings.EnvironmentsDir) != "" {
			settings.EnvironmentsDir = fileSettings.EnvironmentsDir
		}
		if strings.TrimSpace(fileSettings.BackupsDir) != "" {
			settings.BackupsDir = fileSettings.BackupsDir
		}
		if fileSettings.WebUIPort != 0 {
			settings.WebUIPort = fileSettings.WebUIPort
		}
		settings.ShellRC = strings.TrimSpace(fileSettings.ShellRC)
		settings.DaemonAutoStart = fileSettings.DaemonAutoStart
	} else if !os.IsNotExist(readErr) {
		return Settings{}, false, fmt.Errorf("read config file: %w", readErr)
	}

	if envOverride := strings.TrimSpace(os.Getenv("ELK_LOCAL_ENVIRONMENTS_DIR")); envOverride != "" {
		settings.EnvironmentsDir = envOverride
		configured = true
	}
	if envOverride := strings.TrimSpace(os.Getenv("ELK_LOCAL_BACKUPS_DIR")); envOverride != "" {
		settings.BackupsDir = envOverride
		configured = true
	}
	if envOverride := strings.TrimSpace(os.Getenv("ELK_LOCAL_WEBUI_PORT")); envOverride != "" {
		webUIPort, err := strconv.Atoi(envOverride)
		if err != nil {
			return Settings{}, false, fmt.Errorf("parse ELK_LOCAL_WEBUI_PORT: %w", err)
		}
		settings.WebUIPort = webUIPort
		configured = true
	}
	if strings.TrimSpace(os.Getenv("ELK_LOCAL_HOME")) != "" {
		configured = true
	}

	settings.EnvironmentsDir, err = resolvePath(settings.EnvironmentsDir)
	if err != nil {
		return Settings{}, false, fmt.Errorf("resolve environments dir: %w", err)
	}

	settings.BackupsDir, err = resolvePath(settings.BackupsDir)
	if err != nil {
		return Settings{}, false, fmt.Errorf("resolve backups dir: %w", err)
	}

	if settings.WebUIPort < 1 || settings.WebUIPort > 65535 {
		return Settings{}, false, fmt.Errorf("webui port must be between 1 and 65535")
	}

	if strings.TrimSpace(settings.ShellRC) != "" {
		settings.ShellRC, err = resolvePath(settings.ShellRC)
		if err != nil {
			return Settings{}, false, fmt.Errorf("resolve shell rc path: %w", err)
		}
	}

	return settings, configured, nil
}

func ResolveRegistryRoot(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return resolveExistingDir(override)
	}

	_, configured, err := Load()
	if err != nil {
		return "", err
	}

	if configured {
		return UserHomeDir()
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current working directory: %w", err)
	}

	return resolveExistingDir(currentDir)
}

func DefaultEnvironmentProjectRoot(name string) (string, bool, error) {
	settings, configured, err := Load()
	if err != nil || !configured {
		return "", configured, err
	}

	resolvedPath, err := resolvePath(filepath.Join(settings.EnvironmentsDir, name))
	if err != nil {
		return "", false, err
	}

	return resolvedPath, true, nil
}

func ResolveWebUIPort(port int) (int, error) {
	if port > 0 {
		return port, nil
	}

	settings, _, err := Load()
	if err != nil {
		return 0, err
	}

	return settings.WebUIPort, nil
}

func DefaultManifestStorageDir(name string) (string, bool, error) {
	_, configured, err := Load()
	if err != nil || !configured {
		return "", configured, err
	}

	homeRoot, err := HomeRoot()
	if err != nil {
		return "", false, err
	}

	resolvedPath, err := resolvePath(filepath.Join(homeRoot, "environments", name))
	if err != nil {
		return "", false, err
	}

	return resolvedPath, true, nil
}

func DefaultBackupStorageDir(name string) (string, bool, error) {
	settings, configured, err := Load()
	if err != nil || !configured {
		return "", configured, err
	}

	resolvedPath, err := resolvePath(filepath.Join(settings.BackupsDir, name))
	if err != nil {
		return "", false, err
	}

	return resolvedPath, true, nil
}

func resolvePath(rawPath string) (string, error) {
	expandedPath, err := expandPath(rawPath)
	if err != nil {
		return "", err
	}

	absolutePath, err := filepath.Abs(expandedPath)
	if err != nil {
		return "", err
	}

	return filepath.Clean(absolutePath), nil
}

func resolveExistingDir(rawPath string) (string, error) {
	resolvedPath, err := resolvePath(rawPath)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("stat project root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project root must be a directory")
	}

	return resolvedPath, nil
}

func expandPath(rawPath string) (string, error) {
	trimmedPath := strings.TrimSpace(rawPath)
	if trimmedPath == "" {
		return "", fmt.Errorf("path must not be empty")
	}

	if trimmedPath == "~" || strings.HasPrefix(trimmedPath, "~/") {
		homeDir, err := UserHomeDir()
		if err != nil {
			return "", err
		}

		if trimmedPath == "~" {
			trimmedPath = homeDir
		} else {
			trimmedPath = filepath.Join(homeDir, trimmedPath[2:])
		}
	}

	return os.ExpandEnv(trimmedPath), nil
}
