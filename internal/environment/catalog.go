package environment

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func ListManifests(projectRoot string) ([]Manifest, error) {
	root, err := resolveProjectRoot(projectRoot)
	if err != nil {
		return nil, err
	}

	environmentsDir := filepath.Join(root, ".elk-local", "environments")
	entries, err := os.ReadDir(environmentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read environments directory: %w", err)
	}

	sort.Slice(entries, func(left int, right int) bool {
		return entries[left].Name() < entries[right].Name()
	})

	manifests := make([]Manifest, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		manifestPath := filepath.Join(environmentsDir, entry.Name(), "environment.yaml")
		if _, err := os.Stat(manifestPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, fmt.Errorf("stat manifest %s: %w", manifestPath, err)
		}

		manifest, err := LoadManifest(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("load manifest %s: %w", manifestPath, err)
		}

		manifests = append(manifests, manifest)
	}

	return manifests, nil
}
