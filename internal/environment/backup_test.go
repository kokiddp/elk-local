package environment

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeBackupExecutor struct {
	calls      []fakeBackupCall
	dumpOutput string
	stdin      []byte
}

type fakeBackupCall struct {
	composeFile string
	args        []string
	withInput   bool
}

func (executor *fakeBackupExecutor) Run(composeFile string, args ...string) (string, error) {
	executor.calls = append(executor.calls, fakeBackupCall{composeFile: composeFile, args: append([]string{}, args...)})
	joinedArgs := strings.Join(args, " ")
	if strings.Contains(joinedArgs, "mysqldump") {
		return executor.dumpOutput, nil
	}
	return "", nil
}

func (executor *fakeBackupExecutor) RunWithInput(composeFile string, input []byte, args ...string) (string, error) {
	executor.stdin = append([]byte{}, input...)
	executor.calls = append(executor.calls, fakeBackupCall{composeFile: composeFile, args: append([]string{}, args...), withInput: true})
	return "", nil
}

func TestBackupCreatesManagedArchive(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "index.php"), []byte("<?php echo 'hello';\n"), 0o644); err != nil {
		t.Fatalf("write project file: %v", err)
	}

	created, err := Create(CreateOptions{
		Name:        "backup-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	executor := &fakeBackupExecutor{dumpOutput: "CREATE TABLE posts (id INT);\n"}
	service := NewBackupService(executor)
	service.Now = func() time.Time { return time.Date(2026, time.March, 28, 12, 34, 56, 0, time.UTC) }

	artifact, err := service.Backup(created.Manifest, BackupCreateOptions{IncludeProjectFiles: true})
	if err != nil {
		t.Fatalf("backup environment: %v", err)
	}

	expectedPath := filepath.Join(created.Manifest.Storage.BackupsPath, "backup-demo-20260328-123456Z.tar.gz")
	if artifact.Path != expectedPath {
		t.Fatalf("unexpected backup path: %s", artifact.Path)
	}

	entries, metadata := readBackupArchiveForTest(t, artifact.Path)
	if metadata.EnvironmentName != created.Manifest.Name {
		t.Fatalf("unexpected metadata environment name: %+v", metadata)
	}
	if !metadata.IncludesProjectFiles {
		t.Fatalf("expected project files to be included: %+v", metadata)
	}
	if _, ok := entries[backupMetadataName]; !ok {
		t.Fatalf("backup archive missing %s", backupMetadataName)
	}
	if _, ok := entries[backupManifestName]; !ok {
		t.Fatalf("backup archive missing %s", backupManifestName)
	}
	if string(entries[backupDatabaseName]) != executor.dumpOutput {
		t.Fatalf("unexpected database dump contents: %s", string(entries[backupDatabaseName]))
	}
	if string(entries[path.Join(backupProjectPrefix, "index.php")]) != "<?php echo 'hello';\n" {
		t.Fatalf("backup archive missing project file: %v", entries)
	}
	if _, ok := entries[path.Join(backupProjectPrefix, ".elk-local/environments")]; ok {
		t.Fatalf("backup archive should not include managed elk-local artifacts")
	}

	joinedCalls := make([]string, 0, len(executor.calls))
	for _, call := range executor.calls {
		joinedCalls = append(joinedCalls, strings.Join(call.args, " "))
	}
	if len(joinedCalls) < 3 {
		t.Fatalf("expected startup, readiness, and dump calls, got %v", joinedCalls)
	}
}

func TestImportCopiesArchiveIntoManagedBackupsDirectory(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	created, err := Create(CreateOptions{
		Name:        "import-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	sourceArchive := filepath.Join(t.TempDir(), "portable.tar.gz")
	metadata := buildBackupMetadata(created.Manifest, time.Date(2026, time.March, 28, 10, 0, 0, 0, time.UTC), false)
	if err := writeBackupArchive(sourceArchive, created.Manifest, metadata, []byte("CREATE TABLE demo (id INT);\n"), false); err != nil {
		t.Fatalf("write source backup archive: %v", err)
	}

	service := NewBackupService(&fakeBackupExecutor{})
	artifact, err := service.Import(created.Manifest, sourceArchive, false)
	if err != nil {
		t.Fatalf("import backup archive: %v", err)
	}

	if filepath.Dir(artifact.Path) != created.Manifest.Storage.BackupsPath {
		t.Fatalf("expected imported archive in managed backups dir, got %s", artifact.Path)
	}
	if _, err := os.Stat(artifact.Path); err != nil {
		t.Fatalf("stat imported archive: %v", err)
	}
	if artifact.Metadata.EnvironmentName != created.Manifest.Name {
		t.Fatalf("unexpected imported metadata: %+v", artifact.Metadata)
	}
	if sameFilePath(artifact.Path, sourceArchive) {
		t.Fatal("expected import to create a managed copy")
	}
	importedContents, err := os.ReadFile(artifact.Path)
	if err != nil {
		t.Fatalf("read imported archive: %v", err)
	}
	sourceContents, err := os.ReadFile(sourceArchive)
	if err != nil {
		t.Fatalf("read source archive: %v", err)
	}
	if string(importedContents) != string(sourceContents) {
		t.Fatal("imported archive contents do not match the source archive")
	}
}

func TestRestoreResetsDatabaseAndProjectFiles(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "index.php"), []byte("<?php echo 'before';\n"), 0o644); err != nil {
		t.Fatalf("write seed project file: %v", err)
	}

	created, err := Create(CreateOptions{
		Name:        "restore-demo",
		Preset:      "wordpress",
		ProjectRoot: projectRoot,
		Installer:   stubApplicationInstaller{},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	archivePath := filepath.Join(t.TempDir(), "restore-demo.tar.gz")
	metadata := buildBackupMetadata(created.Manifest, time.Date(2026, time.March, 28, 14, 0, 0, 0, time.UTC), true)
	if err := writeBackupArchive(archivePath, created.Manifest, metadata, []byte("CREATE TABLE restored (id INT);\n"), true); err != nil {
		t.Fatalf("write backup archive: %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectRoot, "index.php"), []byte("<?php echo 'changed';\n"), 0o644); err != nil {
		t.Fatalf("mutate project file: %v", err)
	}

	executor := &fakeBackupExecutor{}
	service := NewBackupService(executor)
	result, err := service.Restore(created.Manifest, RestoreOptions{
		ArchivePath:         archivePath,
		RestoreProjectFiles: true,
		Force:               true,
	})
	if err != nil {
		t.Fatalf("restore backup archive: %v", err)
	}

	if !result.RestoredProjectFiles {
		t.Fatal("expected project files to be restored")
	}
	if string(executor.stdin) != "CREATE TABLE restored (id INT);\n" {
		t.Fatalf("unexpected restore stdin: %s", string(executor.stdin))
	}

	projectContents, err := os.ReadFile(filepath.Join(projectRoot, "index.php"))
	if err != nil {
		t.Fatalf("read restored project file: %v", err)
	}
	if !strings.Contains(string(projectContents), "before") {
		t.Fatalf("expected backup project file to be restored, got %s", string(projectContents))
	}

	var sawReset bool
	var sawRestore bool
	for _, call := range executor.calls {
		joinedArgs := strings.Join(call.args, " ")
		if strings.Contains(joinedArgs, "DROP DATABASE IF EXISTS `restore_demo`; CREATE DATABASE `restore_demo`;") {
			sawReset = true
		}
		if call.withInput && strings.Contains(joinedArgs, "mariadb -u root 'restore_demo'") {
			sawRestore = true
		}
		if call.withInput && strings.Contains(joinedArgs, "mysql -u root 'restore_demo'") {
			sawRestore = true
		}
	}
	if !sawReset {
		t.Fatalf("expected database reset call, got %+v", executor.calls)
	}
	if !sawRestore {
		t.Fatalf("expected database restore call, got %+v", executor.calls)
	}
}

func TestDatabaseScriptsSupportMySQLAndMariaDBTools(t *testing.T) {
	t.Parallel()

	if script := databaseReadinessScript("secret"); !strings.Contains(script, "mariadb-admin") || !strings.Contains(script, "mysqladmin") {
		t.Fatalf("expected readiness script to support mariadb-admin and mysqladmin, got %s", script)
	}

	if script := databaseDumpScript("secret", "demo_db"); !strings.Contains(script, "mariadb-dump") || !strings.Contains(script, "mysqldump") {
		t.Fatalf("expected dump script to support mariadb-dump and mysqldump, got %s", script)
	}

	if script := databaseResetScript("secret", "SELECT 1"); !strings.Contains(script, "mariadb -u root") || !strings.Contains(script, "mysql -u root") {
		t.Fatalf("expected reset script to support mariadb and mysql clients, got %s", script)
	}

	if script := databaseRestoreScript("secret", "demo_db"); !strings.Contains(script, "mariadb -u root") || !strings.Contains(script, "mysql -u root") {
		t.Fatalf("expected restore script to support mariadb and mysql clients, got %s", script)
	}
}

func readBackupArchiveForTest(t *testing.T, archivePath string) (map[string][]byte, BackupMetadata) {
	t.Helper()

	archiveFile, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer archiveFile.Close()

	gzipReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		t.Fatalf("open gzip reader: %v", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	entries := map[string][]byte{}
	var metadata BackupMetadata
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read archive entry: %v", err)
		}

		cleanName := path.Clean(header.Name)
		if header.FileInfo().IsDir() {
			entries[cleanName] = nil
			continue
		}
		contents, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("read archive contents: %v", err)
		}
		entries[cleanName] = contents
		if cleanName == backupMetadataName {
			if err := json.Unmarshal(contents, &metadata); err != nil {
				t.Fatalf("decode metadata: %v", err)
			}
		}
	}

	return entries, metadata
}
