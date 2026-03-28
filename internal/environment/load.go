package environment

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func LoadManifest(manifestPath string) (Manifest, error) {
	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(contents, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}

	normalizeManifest(&manifest)

	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, err
	}

	return manifest, nil
}

func ResolveManifestPath(projectRoot string, environmentName string) (string, error) {
	root, err := resolveProjectRoot(projectRoot)
	if err != nil {
		return "", err
	}

	name := filepath.Clean(environmentName)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "", fmt.Errorf("environment name is required")
	}

	return filepath.Join(root, ".elk-local", "environments", name, "environment.yaml"), nil
}
