package environment

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"elk-local/internal/config"
)

type DeleteResult struct {
	RemovedProjectFiles bool
	RemovedBackups      bool
}

func Delete(projectRoot string, name string) (DeleteResult, error) {
	if strings.TrimSpace(name) == "" {
		return DeleteResult{}, fmt.Errorf("environment name is required")
	}

	manifestPath, err := ResolveManifestPath(projectRoot, name)
	if err != nil {
		return DeleteResult{}, err
	}

	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return DeleteResult{}, err
	}

	storagePath := filepath.Clean(strings.TrimSpace(manifest.Storage.BasePath))
	projectPath := filepath.Clean(strings.TrimSpace(manifest.Project.Root))
	backupsPath := filepath.Clean(strings.TrimSpace(manifest.Storage.BackupsPath))

	result := DeleteResult{}
	if pathWithin(backupsPath, storagePath) {
		result.RemovedBackups = true
	}
	if samePath(projectPath, storagePath) {
		result.RemovedProjectFiles = true
	}

	if err := removeTreeIfSafe(storagePath, "environment storage"); err != nil {
		return DeleteResult{}, err
	}

	if !result.RemovedProjectFiles && shouldRemoveManagedProjectRoot(manifest) {
		if err := removeTreeIfSafe(projectPath, "environment project root"); err != nil {
			return DeleteResult{}, err
		}
		result.RemovedProjectFiles = true
	}

	if !result.RemovedBackups && shouldRemoveManagedBackups(manifest) {
		if err := removeTreeIfSafe(backupsPath, "environment backups"); err != nil {
			return DeleteResult{}, err
		}
		result.RemovedBackups = true
	}

	return result, nil
}

func shouldRemoveManagedProjectRoot(manifest Manifest) bool {
	defaultProjectRoot, configured, err := config.DefaultEnvironmentProjectRoot(manifest.Name)
	if err != nil || !configured {
		return false
	}

	return samePath(defaultProjectRoot, manifest.Project.Root)
}

func shouldRemoveManagedBackups(manifest Manifest) bool {
	defaultBackupsPath, configured, err := config.DefaultBackupStorageDir(manifest.Name)
	if err != nil || !configured {
		return false
	}

	return samePath(defaultBackupsPath, manifest.Storage.BackupsPath)
}

func removeTreeIfSafe(path string, label string) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("%s path is empty", label)
	}

	cleanPath := filepath.Clean(trimmedPath)
	if cleanPath == "." || cleanPath == string(filepath.Separator) {
		return fmt.Errorf("refusing to remove unsafe %s path %q", label, cleanPath)
	}

	if err := os.RemoveAll(cleanPath); err != nil {
		return fmt.Errorf("remove %s %s: %w", label, cleanPath, err)
	}

	return nil
}

func samePath(left string, right string) bool {
	return filepath.Clean(strings.TrimSpace(left)) == filepath.Clean(strings.TrimSpace(right))
}

func pathWithin(candidate string, parent string) bool {
	if strings.TrimSpace(candidate) == "" || strings.TrimSpace(parent) == "" {
		return false
	}

	relativePath, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(candidate))
	if err != nil {
		return false
	}

	return relativePath == "." || (!strings.HasPrefix(relativePath, "..") && relativePath != "")
}