package environment

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	backupFormatVersion = "elk.dev/backup.v1alpha1"
	backupMetadataName  = "metadata.json"
	backupManifestName  = "manifest.yaml"
	backupDatabaseName  = "database.sql"
	backupProjectPrefix = "project/"
	backupTimeFormat    = "20060102-150405Z"
)

type ComposeInputExecutor interface {
	Run(composeFile string, args ...string) (string, error)
	RunWithInput(composeFile string, input []byte, args ...string) (string, error)
}

type BackupService struct {
	Executor ComposeInputExecutor
	Now      func() time.Time
}

type BackupCreateOptions struct {
	OutputPath          string
	IncludeProjectFiles bool
	Force               bool
}

type RestoreOptions struct {
	ArchivePath         string
	RestoreProjectFiles bool
	Force               bool
}

type BackupArtifact struct {
	Path     string
	Metadata BackupMetadata
}

type BackupInventoryItem struct {
	FileName             string
	Path                 string
	SizeBytes            int64
	CreatedAt            string
	EnvironmentName      string
	DatabaseName         string
	IncludesProjectFiles bool
	Error                string
}

type RestoreResult struct {
	Path                 string
	Metadata             BackupMetadata
	RestoredProjectFiles bool
}

type BackupMetadata struct {
	FormatVersion        string `json:"formatVersion"`
	CreatedAt            string `json:"createdAt"`
	EnvironmentName      string `json:"environmentName"`
	Preset               string `json:"preset"`
	ProjectType          string `json:"projectType"`
	ProjectRoot          string `json:"projectRoot"`
	DatabaseEngine       string `json:"databaseEngine"`
	DatabaseName         string `json:"databaseName"`
	IncludesProjectFiles bool   `json:"includesProjectFiles"`
}

type backupInspection struct {
	Metadata        BackupMetadata
	HasManifest     bool
	HasDatabaseDump bool
	HasProjectFiles bool
}

func NewBackupService(executor ComposeInputExecutor) BackupService {
	if executor == nil {
		executor = DockerComposeExecutor{}
	}

	return BackupService{
		Executor: executor,
		Now:      time.Now,
	}
}

func (service BackupService) Backup(manifest Manifest, options BackupCreateOptions) (BackupArtifact, error) {
	now := service.now().UTC()
	outputPath, err := resolveBackupOutputPath(manifest, options.OutputPath, now)
	if err != nil {
		return BackupArtifact{}, err
	}

	if err := ensureArchiveDestination(outputPath, options.Force); err != nil {
		return BackupArtifact{}, err
	}

	if err := service.ensureDatabaseReady(manifest); err != nil {
		return BackupArtifact{}, err
	}

	databaseDump, err := service.dumpDatabase(manifest)
	if err != nil {
		return BackupArtifact{}, err
	}

	metadata := buildBackupMetadata(manifest, now, options.IncludeProjectFiles)
	if err := writeBackupArchive(outputPath, manifest, metadata, databaseDump, options.IncludeProjectFiles); err != nil {
		return BackupArtifact{}, err
	}

	return BackupArtifact{Path: outputPath, Metadata: metadata}, nil
}

func (service BackupService) Import(manifest Manifest, archivePath string, force bool) (BackupArtifact, error) {
	resolvedArchivePath, err := resolveExistingBackupArchivePath(manifest, archivePath)
	if err != nil {
		return BackupArtifact{}, err
	}

	inspection, err := inspectBackupArchive(resolvedArchivePath)
	if err != nil {
		return BackupArtifact{}, err
	}

	if err := os.MkdirAll(manifest.Storage.BackupsPath, 0o755); err != nil {
		return BackupArtifact{}, fmt.Errorf("create backups directory: %w", err)
	}

	destinationPath := filepath.Join(manifest.Storage.BackupsPath, filepath.Base(resolvedArchivePath))
	if sameFilePath(resolvedArchivePath, destinationPath) {
		return BackupArtifact{Path: destinationPath, Metadata: inspection.Metadata}, nil
	}

	if err := ensureArchiveDestination(destinationPath, force); err != nil {
		return BackupArtifact{}, err
	}

	if err := copyFile(resolvedArchivePath, destinationPath); err != nil {
		return BackupArtifact{}, err
	}

	return BackupArtifact{Path: destinationPath, Metadata: inspection.Metadata}, nil
}

func (service BackupService) Restore(manifest Manifest, options RestoreOptions) (RestoreResult, error) {
	if !options.Force {
		return RestoreResult{}, fmt.Errorf("restore replaces the target database; rerun with --force to continue")
	}

	resolvedArchivePath, err := resolveExistingBackupArchivePath(manifest, options.ArchivePath)
	if err != nil {
		return RestoreResult{}, err
	}

	inspection, err := inspectBackupArchive(resolvedArchivePath)
	if err != nil {
		return RestoreResult{}, err
	}

	if options.RestoreProjectFiles && !inspection.HasProjectFiles {
		return RestoreResult{}, fmt.Errorf("backup archive %s does not include project files", resolvedArchivePath)
	}

	databaseDump, err := readBackupDatabaseDump(resolvedArchivePath)
	if err != nil {
		return RestoreResult{}, err
	}

	if err := service.ensureDatabaseReady(manifest); err != nil {
		return RestoreResult{}, err
	}

	if err := service.resetDatabase(manifest); err != nil {
		return RestoreResult{}, err
	}

	if err := service.restoreDatabase(manifest, databaseDump); err != nil {
		return RestoreResult{}, err
	}

	if options.RestoreProjectFiles {
		if err := restoreProjectFilesFromArchive(resolvedArchivePath, manifest.Project.Root); err != nil {
			return RestoreResult{}, err
		}
	}

	return RestoreResult{
		Path:                 resolvedArchivePath,
		Metadata:             inspection.Metadata,
		RestoredProjectFiles: options.RestoreProjectFiles,
	}, nil
}

func (service BackupService) List(manifest Manifest) ([]BackupInventoryItem, error) {
	entries, err := os.ReadDir(manifest.Storage.BackupsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read backups directory: %w", err)
	}

	items := make([]BackupInventoryItem, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		itemPath := filepath.Join(manifest.Storage.BackupsPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat backup archive %s: %w", itemPath, err)
		}

		item := BackupInventoryItem{
			FileName:  entry.Name(),
			Path:      itemPath,
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
		}

		inspection, inspectErr := inspectBackupArchive(itemPath)
		if inspectErr != nil {
			item.Error = inspectErr.Error()
		} else {
			item.CreatedAt = firstNonEmpty(inspection.Metadata.CreatedAt, item.CreatedAt)
			item.EnvironmentName = inspection.Metadata.EnvironmentName
			item.DatabaseName = inspection.Metadata.DatabaseName
			item.IncludesProjectFiles = inspection.Metadata.IncludesProjectFiles
		}

		items = append(items, item)
	}

	sort.Slice(items, func(left int, right int) bool {
		if items[left].CreatedAt == items[right].CreatedAt {
			return items[left].FileName > items[right].FileName
		}

		return items[left].CreatedAt > items[right].CreatedAt
	})

	return items, nil
}

func (service BackupService) ResolveManagedArchive(manifest Manifest, archiveName string) (string, error) {
	return resolveManagedBackupArchivePath(manifest, archiveName)
}

func (service BackupService) DeleteManagedArchive(manifest Manifest, archiveName string) error {
	resolvedArchivePath, err := resolveManagedBackupArchivePath(manifest, archiveName)
	if err != nil {
		return err
	}

	if err := os.Remove(resolvedArchivePath); err != nil {
		return fmt.Errorf("delete managed backup archive: %w", err)
	}

	return nil
}

func (service BackupService) ensureDatabaseReady(manifest Manifest) error {
	if _, err := service.Executor.Run(manifest.Compose.File, "up", "-d", "db"); err != nil {
		return err
	}

	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	var lastOutput string
	for time.Now().Before(deadline) {
		output, err := service.Executor.Run(
			manifest.Compose.File,
			"exec", "-T", "db",
			"sh", "-lc", databaseReadinessScript(manifest.Runtime.Database.RootPassword),
		)
		if err == nil {
			return nil
		}

		lastErr = err
		lastOutput = output
		time.Sleep(1 * time.Second)
	}

	if strings.TrimSpace(lastOutput) != "" {
		return fmt.Errorf("wait for database readiness: %w\n%s", lastErr, lastOutput)
	}

	return fmt.Errorf("wait for database readiness: %w", lastErr)
}

func (service BackupService) dumpDatabase(manifest Manifest) ([]byte, error) {
	output, err := service.Executor.Run(
		manifest.Compose.File,
		"exec", "-T", "db",
		"sh", "-lc", databaseDumpScript(manifest.Runtime.Database.RootPassword, manifest.Runtime.Database.Name),
	)
	if err != nil {
		return nil, err
	}

	return []byte(output), nil
}

func (service BackupService) resetDatabase(manifest Manifest) error {
	statement := fmt.Sprintf(
		"DROP DATABASE IF EXISTS %s; CREATE DATABASE %s;",
		quoteMySQLIdentifier(manifest.Runtime.Database.Name),
		quoteMySQLIdentifier(manifest.Runtime.Database.Name),
	)

	_, err := service.Executor.Run(
		manifest.Compose.File,
		"exec", "-T", "db",
		"sh", "-lc", databaseResetScript(manifest.Runtime.Database.RootPassword, statement),
	)
	return err
}

func (service BackupService) restoreDatabase(manifest Manifest, databaseDump []byte) error {
	_, err := service.Executor.RunWithInput(
		manifest.Compose.File,
		databaseDump,
		"exec", "-T", "db",
		"sh", "-lc", databaseRestoreScript(manifest.Runtime.Database.RootPassword, manifest.Runtime.Database.Name),
	)
	return err
}

func (service BackupService) now() time.Time {
	if service.Now != nil {
		return service.Now()
	}

	return time.Now()
}

func resolveBackupOutputPath(manifest Manifest, outputPath string, now time.Time) (string, error) {
	trimmedOutput := strings.TrimSpace(outputPath)
	if trimmedOutput == "" {
		trimmedOutput = filepath.Join(
			manifest.Storage.BackupsPath,
			fmt.Sprintf("%s-%s.tar.gz", manifest.Name, now.Format(backupTimeFormat)),
		)
	}

	resolvedOutputPath, err := filepath.Abs(trimmedOutput)
	if err != nil {
		return "", fmt.Errorf("resolve backup output path: %w", err)
	}

	return filepath.Clean(resolvedOutputPath), nil
}

func ensureArchiveDestination(outputPath string, force bool) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create backup output directory: %w", err)
	}

	_, err := os.Stat(outputPath)
	if err == nil && !force {
		return fmt.Errorf("backup archive %s already exists; use --force to overwrite it", outputPath)
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect backup output path: %w", err)
	}

	return nil
}

func buildBackupMetadata(manifest Manifest, createdAt time.Time, includesProjectFiles bool) BackupMetadata {
	return BackupMetadata{
		FormatVersion:        backupFormatVersion,
		CreatedAt:            createdAt.Format(time.RFC3339),
		EnvironmentName:      manifest.Name,
		Preset:               manifest.Preset,
		ProjectType:          manifest.Project.Type,
		ProjectRoot:          manifest.Project.Root,
		DatabaseEngine:       manifest.Runtime.Database.Engine,
		DatabaseName:         manifest.Runtime.Database.Name,
		IncludesProjectFiles: includesProjectFiles,
	}
}

func writeBackupArchive(outputPath string, manifest Manifest, metadata BackupMetadata, databaseDump []byte, includeProjectFiles bool) error {
	tempFile, err := os.CreateTemp(filepath.Dir(outputPath), filepath.Base(outputPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create backup archive: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	gzipWriter := gzip.NewWriter(tempFile)
	tarWriter := tar.NewWriter(gzipWriter)

	manifestContents, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal backup manifest: %w", err)
	}

	metadataContents, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal backup metadata: %w", err)
	}

	archiveTime, err := time.Parse(time.RFC3339, metadata.CreatedAt)
	if err != nil {
		archiveTime = time.Now().UTC()
	}

	if err := writeTarEntry(tarWriter, backupMetadataName, metadataContents, 0o644, archiveTime); err != nil {
		return err
	}
	if err := writeTarEntry(tarWriter, backupManifestName, manifestContents, 0o644, archiveTime); err != nil {
		return err
	}
	if err := writeTarEntry(tarWriter, backupDatabaseName, databaseDump, 0o600, archiveTime); err != nil {
		return err
	}

	if includeProjectFiles {
		excludedPaths := map[string]struct{}{
			manifest.Storage.BasePath: {},
			outputPath:                {},
			tempPath:                  {},
		}
		if err := addProjectTreeToArchive(tarWriter, manifest.Project.Root, excludedPaths); err != nil {
			return err
		}
	}

	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("finalize backup archive: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("compress backup archive: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close backup archive: %w", err)
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		return fmt.Errorf("move backup archive into place: %w", err)
	}

	return nil
}

func writeTarEntry(writer *tar.Writer, name string, contents []byte, mode int64, modTime time.Time) error {
	header := &tar.Header{
		Name:    name,
		Mode:    mode,
		Size:    int64(len(contents)),
		ModTime: modTime,
	}
	if err := writer.WriteHeader(header); err != nil {
		return fmt.Errorf("write backup entry %s: %w", name, err)
	}
	if _, err := writer.Write(contents); err != nil {
		return fmt.Errorf("write backup entry %s: %w", name, err)
	}

	return nil
}

func addProjectTreeToArchive(writer *tar.Writer, projectRoot string, excludedPaths map[string]struct{}) error {
	return filepath.WalkDir(projectRoot, func(currentPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		cleanPath := filepath.Clean(currentPath)
		if _, excluded := excludedPaths[cleanPath]; excluded {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(projectRoot, cleanPath)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		archiveName := filepath.ToSlash(relPath)
		if firstPathComponent(archiveName) == ".elk-local" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if !info.Mode().IsRegular() && !info.IsDir() {
			return fmt.Errorf("backup project files: unsupported file type at %s", cleanPath)
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("backup project files: create tar header for %s: %w", cleanPath, err)
		}
		header.Name = backupProjectPrefix + archiveName
		if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}

		if err := writer.WriteHeader(header); err != nil {
			return fmt.Errorf("backup project files: write header for %s: %w", cleanPath, err)
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(cleanPath)
		if err != nil {
			return fmt.Errorf("backup project files: open %s: %w", cleanPath, err)
		}

		if _, err := io.Copy(writer, file); err != nil {
			_ = file.Close()
			return fmt.Errorf("backup project files: copy %s: %w", cleanPath, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("backup project files: close %s: %w", cleanPath, err)
		}

		return nil
	})
}

func inspectBackupArchive(archivePath string) (backupInspection, error) {
	archive, err := os.Open(archivePath)
	if err != nil {
		return backupInspection{}, fmt.Errorf("open backup archive: %w", err)
	}
	defer archive.Close()

	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return backupInspection{}, fmt.Errorf("open backup gzip stream: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	inspection := backupInspection{}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return backupInspection{}, fmt.Errorf("read backup archive: %w", err)
		}

		cleanName := path.Clean(header.Name)
		switch cleanName {
		case backupMetadataName:
			contents, err := io.ReadAll(tarReader)
			if err != nil {
				return backupInspection{}, fmt.Errorf("read backup metadata: %w", err)
			}
			if err := json.Unmarshal(contents, &inspection.Metadata); err != nil {
				return backupInspection{}, fmt.Errorf("decode backup metadata: %w", err)
			}
		case backupManifestName:
			inspection.HasManifest = true
		case backupDatabaseName:
			inspection.HasDatabaseDump = true
		default:
			if strings.HasPrefix(cleanName, backupProjectPrefix) && cleanName != backupProjectPrefix {
				inspection.HasProjectFiles = true
			}
		}
	}

	if inspection.Metadata.FormatVersion != backupFormatVersion {
		return backupInspection{}, fmt.Errorf("backup archive %s does not use the supported ELK-Local backup format", archivePath)
	}
	if !inspection.HasManifest {
		return backupInspection{}, fmt.Errorf("backup archive %s is missing %s", archivePath, backupManifestName)
	}
	if !inspection.HasDatabaseDump {
		return backupInspection{}, fmt.Errorf("backup archive %s is missing %s", archivePath, backupDatabaseName)
	}

	return inspection, nil
}

func readBackupDatabaseDump(archivePath string) ([]byte, error) {
	archive, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open backup archive: %w", err)
	}
	defer archive.Close()

	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return nil, fmt.Errorf("open backup gzip stream: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read backup archive: %w", err)
		}

		if path.Clean(header.Name) == backupDatabaseName {
			contents, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("read backup database dump: %w", err)
			}
			return contents, nil
		}
	}

	return nil, fmt.Errorf("backup archive %s is missing %s", archivePath, backupDatabaseName)
}

func restoreProjectFilesFromArchive(archivePath string, projectRoot string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open backup archive: %w", err)
	}
	defer archive.Close()

	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("open backup gzip stream: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read backup archive: %w", err)
		}

		cleanName := path.Clean(header.Name)
		if !strings.HasPrefix(cleanName, backupProjectPrefix) || cleanName == backupProjectPrefix {
			continue
		}

		targetPath, err := safeProjectRestorePath(projectRoot, cleanName)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("restore project directory %s: %w", targetPath, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("restore project directory %s: %w", filepath.Dir(targetPath), err)
			}

			contents, err := io.ReadAll(tarReader)
			if err != nil {
				return fmt.Errorf("read project file %s from archive: %w", cleanName, err)
			}

			if err := os.WriteFile(targetPath, contents, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("restore project file %s: %w", targetPath, err)
			}
		default:
			return fmt.Errorf("restore project files: unsupported archive entry type for %s", cleanName)
		}
	}

	return nil
}

func safeProjectRestorePath(projectRoot string, archiveName string) (string, error) {
	relPath := strings.TrimPrefix(path.Clean(archiveName), backupProjectPrefix)
	if relPath == "." || relPath == "" || strings.HasPrefix(relPath, "../") {
		return "", fmt.Errorf("restore project files: invalid archive path %s", archiveName)
	}

	targetPath := filepath.Join(projectRoot, filepath.FromSlash(relPath))
	cleanRoot := filepath.Clean(projectRoot)
	cleanTarget := filepath.Clean(targetPath)
	if cleanTarget != cleanRoot && !strings.HasPrefix(cleanTarget, cleanRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("restore project files: archive path escapes project root: %s", archiveName)
	}

	return cleanTarget, nil
}

func resolveExistingBackupArchivePath(manifest Manifest, archivePath string) (string, error) {
	trimmedPath := strings.TrimSpace(archivePath)
	if trimmedPath == "" {
		return "", fmt.Errorf("backup archive path is required")
	}

	if filepath.IsAbs(trimmedPath) {
		if _, err := os.Stat(trimmedPath); err != nil {
			return "", fmt.Errorf("inspect backup archive: %w", err)
		}
		return filepath.Clean(trimmedPath), nil
	}

	absInputPath, err := filepath.Abs(trimmedPath)
	if err != nil {
		return "", fmt.Errorf("resolve backup archive path: %w", err)
	}
	if _, err := os.Stat(absInputPath); err == nil {
		return filepath.Clean(absInputPath), nil
	}

	managedPath := filepath.Join(manifest.Storage.BackupsPath, trimmedPath)
	if _, err := os.Stat(managedPath); err != nil {
		return "", fmt.Errorf("inspect backup archive: %w", err)
	}

	return filepath.Clean(managedPath), nil
}

func resolveManagedBackupArchivePath(manifest Manifest, archiveName string) (string, error) {
	trimmedName := strings.TrimSpace(archiveName)
	if trimmedName == "" {
		return "", fmt.Errorf("backup file name is required")
	}
	if filepath.IsAbs(trimmedName) {
		return "", fmt.Errorf("managed backup file names must not be absolute paths")
	}

	cleanName := filepath.Clean(trimmedName)
	if cleanName == "." || cleanName == ".." || cleanName != filepath.Base(cleanName) {
		return "", fmt.Errorf("managed backup file names must not include directory segments")
	}

	managedPath := filepath.Join(manifest.Storage.BackupsPath, cleanName)
	info, err := os.Stat(managedPath)
	if err != nil {
		return "", fmt.Errorf("inspect managed backup archive: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("managed backup archive %s is a directory", cleanName)
	}

	return filepath.Clean(managedPath), nil
}

func copyFile(sourcePath string, destinationPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source backup archive: %w", err)
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(destinationPath)
	if err != nil {
		return fmt.Errorf("create imported backup archive: %w", err)
	}
	defer destinationFile.Close()

	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		return fmt.Errorf("copy backup archive: %w", err)
	}

	return nil
}

func sameFilePath(left string, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func firstPathComponent(pathValue string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(pathValue), "/")
	if trimmed == "" {
		return ""
	}

	parts := strings.Split(trimmed, "/")
	return parts[0]
}

func quoteMySQLIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func databaseReadinessScript(rootPassword string) string {
	return fmt.Sprintf(
		"export MYSQL_PWD=%s; if command -v mariadb-admin >/dev/null 2>&1; then exec mariadb-admin ping -h 127.0.0.1 -u root --silent; elif command -v mysqladmin >/dev/null 2>&1; then exec mysqladmin ping -h 127.0.0.1 -u root --silent; elif command -v mariadb >/dev/null 2>&1; then mariadb -h 127.0.0.1 -u root -e \"SELECT 1\" >/dev/null; else mysql -h 127.0.0.1 -u root -e \"SELECT 1\" >/dev/null; fi",
		shellSingleQuote(rootPassword),
	)
}

func databaseDumpScript(rootPassword string, databaseName string) string {
	return fmt.Sprintf(
		"export MYSQL_PWD=%s; if command -v mariadb-dump >/dev/null 2>&1; then exec mariadb-dump -u root --single-transaction --routines --triggers --events %s; else exec mysqldump -u root --single-transaction --routines --triggers --events %s; fi",
		shellSingleQuote(rootPassword),
		shellSingleQuote(databaseName),
		shellSingleQuote(databaseName),
	)
}

func databaseResetScript(rootPassword string, statement string) string {
	return fmt.Sprintf(
		"export MYSQL_PWD=%s; if command -v mariadb >/dev/null 2>&1; then exec mariadb -u root -e %s; else exec mysql -u root -e %s; fi",
		shellSingleQuote(rootPassword),
		shellSingleQuote(statement),
		shellSingleQuote(statement),
	)
}

func databaseRestoreScript(rootPassword string, databaseName string) string {
	return fmt.Sprintf(
		"export MYSQL_PWD=%s; if command -v mariadb >/dev/null 2>&1; then exec mariadb -u root %s; else exec mysql -u root %s; fi",
		shellSingleQuote(rootPassword),
		shellSingleQuote(databaseName),
		shellSingleQuote(databaseName),
	)
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
