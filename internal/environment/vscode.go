package environment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const vscodeLaunchVersion = "0.2.0"

var managedVSCodeLaunchConfigNames = map[string]struct{}{
	"Listen for Xdebug 3.0 (Local)": {},
	"Listen for Xdebug (Local)":     {},
	"Launch currently open script":  {},
}

type vscodeLaunchFile struct {
	Version        string           `json:"version"`
	Configurations []map[string]any `json:"configurations"`
}

func syncVSCodeLaunchConfig(manifest Manifest) error {
	launchPath := filepath.Join(manifest.Project.Root, ".vscode", "launch.json")
	if manifest.Tooling.Xdebug.Enabled {
		return writeVSCodeLaunchConfig(launchPath, manifest)
	}

	return pruneVSCodeLaunchConfig(launchPath)
}

func writeVSCodeLaunchConfig(launchPath string, manifest Manifest) error {
	launchFile, err := readVSCodeLaunchConfig(launchPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		launchFile = vscodeLaunchFile{}
	}

	launchFile.Version = firstNonEmpty(launchFile.Version, vscodeLaunchVersion)
	launchFile.Configurations = append(removeManagedVSCodeLaunchConfigs(launchFile.Configurations), managedVSCodeLaunchConfigs(manifest)...)

	contents, err := json.MarshalIndent(launchFile, "", "\t")
	if err != nil {
		return fmt.Errorf("marshal VS Code launch config: %w", err)
	}

	vscodeDir := filepath.Dir(launchPath)
	if err := os.MkdirAll(vscodeDir, 0o755); err != nil {
		return fmt.Errorf("create VS Code directory: %w", err)
	}

	if err := os.WriteFile(launchPath, append(contents, '\n'), 0o644); err != nil {
		return fmt.Errorf("write VS Code launch config: %w", err)
	}

	return nil
}

func pruneVSCodeLaunchConfig(launchPath string) error {
	launchFile, err := readVSCodeLaunchConfig(launchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	remaining := removeManagedVSCodeLaunchConfigs(launchFile.Configurations)
	if len(remaining) == len(launchFile.Configurations) {
		return nil
	}

	if len(remaining) == 0 {
		if err := os.Remove(launchPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove VS Code launch config: %w", err)
		}

		vscodeDir := filepath.Dir(launchPath)
		if err := os.Remove(vscodeDir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove VS Code directory: %w", err)
		}
		return nil
	}

	launchFile.Version = firstNonEmpty(launchFile.Version, vscodeLaunchVersion)
	launchFile.Configurations = remaining
	contents, err := json.MarshalIndent(launchFile, "", "\t")
	if err != nil {
		return fmt.Errorf("marshal VS Code launch config: %w", err)
	}

	if err := os.WriteFile(launchPath, append(contents, '\n'), 0o644); err != nil {
		return fmt.Errorf("write VS Code launch config: %w", err)
	}

	return nil
}

func readVSCodeLaunchConfig(launchPath string) (vscodeLaunchFile, error) {
	contents, err := os.ReadFile(launchPath)
	if err != nil {
		return vscodeLaunchFile{}, err
	}

	launchFile := vscodeLaunchFile{}
	if err := json.Unmarshal(contents, &launchFile); err != nil {
		return vscodeLaunchFile{}, fmt.Errorf("decode VS Code launch config %s: %w", launchPath, err)
	}

	return launchFile, nil
}

func removeManagedVSCodeLaunchConfigs(configurations []map[string]any) []map[string]any {
	remaining := make([]map[string]any, 0, len(configurations))
	for _, configuration := range configurations {
		name, _ := configuration["name"].(string)
		if _, managed := managedVSCodeLaunchConfigNames[strings.TrimSpace(name)]; managed {
			continue
		}

		remaining = append(remaining, configuration)
	}

	return remaining
}

func managedVSCodeLaunchConfigs(manifest Manifest) []map[string]any {
	xdebugPort := preferredPort(manifest.Tooling.Xdebug.ClientPort, DefaultXdebugClientPort())
	pathMappings := map[string]string{"/var/www/html": "${workspaceFolder}"}

	return []map[string]any{
		{
			"name":         "Listen for Xdebug 3.0 (Local)",
			"type":         "php",
			"request":      "launch",
			"hostname":     "0.0.0.0",
			"port":         xdebugPort,
			"pathMappings": pathMappings,
			"xdebugSettings": map[string]any{
				"max_children": 128,
				"max_data":     1024,
				"max_depth":    3,
				"show_hidden":  1,
			},
		},
		{
			"name":         "Listen for Xdebug (Local)",
			"type":         "php",
			"request":      "launch",
			"hostname":     "0.0.0.0",
			"port":         9000,
			"pathMappings": pathMappings,
			"xdebugSettings": map[string]any{
				"max_children": 128,
				"max_data":     1024,
				"max_depth":    3,
				"show_hidden":  1,
			},
		},
		{
			"name":         "Launch currently open script",
			"type":         "php",
			"request":      "launch",
			"program":      "${file}",
			"cwd":          "${fileDirname}",
			"hostname":     "0.0.0.0",
			"port":         xdebugPort,
			"pathMappings": pathMappings,
			"xdebugSettings": map[string]any{
				"max_children": 128,
				"max_data":     1024,
				"max_depth":    3,
				"show_hidden":  1,
			},
		},
	}
}
