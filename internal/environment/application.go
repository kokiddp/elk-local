package environment

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type ApplicationInstallRequest struct {
	Preset           Preset
	ProjectRoot      string
	RequestedVersion string
}

type InstalledApplication struct {
	Name     string
	Version  string
	Existing bool
}

type ApplicationInstaller interface {
	Install(request ApplicationInstallRequest) (InstalledApplication, error)
}

type DefaultApplicationInstaller struct {
	HTTPClient *http.Client
}

var wordPressVersionPattern = regexp.MustCompile(`\$wp_version\s*=\s*'([^']+)'`)

func (installer DefaultApplicationInstaller) Install(request ApplicationInstallRequest) (InstalledApplication, error) {
	if !request.Preset.InstallsApplication() {
		return InstalledApplication{}, nil
	}

	switch request.Preset.Name {
	case "wordpress":
		return installer.installWordPress(request)
	case "laravel":
		return installer.installComposerApplication(request, "laravel/laravel")
	case "symfony":
		return installer.installComposerApplication(request, "symfony/skeleton")
	default:
		return InstalledApplication{}, nil
	}
}

func (installer DefaultApplicationInstaller) installWordPress(request ApplicationInstallRequest) (InstalledApplication, error) {
	if existing, ok, err := detectExistingWordPress(request.ProjectRoot); err != nil {
		return InstalledApplication{}, err
	} else if ok {
		return existing, nil
	}

	if err := ensureInstallableProjectRoot(request.ProjectRoot, request.Preset); err != nil {
		return InstalledApplication{}, err
	}

	requestedVersion := normalizeWordPressRequestedVersion(request.RequestedVersion, request.Preset.DefaultAppVersion)
	archivePath, err := installer.downloadApplicationArchive(wordPressDownloadURL(requestedVersion))
	if err != nil {
		return InstalledApplication{}, err
	}
	defer os.Remove(archivePath)

	extractRoot, err := os.MkdirTemp("", "elk-local-wordpress-*")
	if err != nil {
		return InstalledApplication{}, fmt.Errorf("create extraction directory: %w", err)
	}
	defer os.RemoveAll(extractRoot)

	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		if err := extractZipArchive(archivePath, extractRoot); err != nil {
			return InstalledApplication{}, err
		}
	} else {
		if err := extractTarGzArchive(archivePath, extractRoot); err != nil {
			return InstalledApplication{}, err
		}
	}

	sourceRoot := filepath.Join(extractRoot, "wordpress")
	if _, err := os.Stat(sourceRoot); err != nil {
		return InstalledApplication{}, fmt.Errorf("locate extracted wordpress files: %w", err)
	}

	if err := copyDirectoryContents(sourceRoot, request.ProjectRoot); err != nil {
		return InstalledApplication{}, err
	}

	installedApplication, _, err := detectExistingWordPress(request.ProjectRoot)
	if err != nil {
		return InstalledApplication{}, err
	}

	installedApplication.Name = request.Preset.ApplicationName
	installedApplication.Version = firstNonEmpty(installedApplication.Version, requestedVersion)
	return installedApplication, nil
}

func (installer DefaultApplicationInstaller) installComposerApplication(request ApplicationInstallRequest, packageName string) (InstalledApplication, error) {
	if existing, ok, err := detectExistingComposerApplication(request.ProjectRoot, request.Preset.Name, request.Preset.ApplicationName); err != nil {
		return InstalledApplication{}, err
	} else if ok {
		return existing, nil
	}

	if err := ensureInstallableProjectRoot(request.ProjectRoot, request.Preset); err != nil {
		return InstalledApplication{}, err
	}

	requestedVersion := firstNonEmpty(strings.TrimSpace(request.RequestedVersion), request.Preset.DefaultAppVersion)
	if err := runComposerCreateProject(request.ProjectRoot, packageName, requestedVersion); err != nil {
		return InstalledApplication{}, err
	}

	return InstalledApplication{
		Name:    request.Preset.ApplicationName,
		Version: firstNonEmpty(requestedVersion, "latest"),
	}, nil
}

func normalizeWordPressRequestedVersion(version string, fallback string) string {
	trimmedVersion := strings.TrimSpace(version)
	if trimmedVersion == "" {
		return firstNonEmpty(fallback, "latest")
	}

	return trimmedVersion
}

func wordPressDownloadURL(version string) string {
	switch strings.ToLower(strings.TrimSpace(version)) {
	case "", "latest":
		return "https://wordpress.org/latest.tar.gz"
	case "nightly":
		return "https://wordpress.org/nightly-builds/wordpress-latest.zip"
	default:
		return fmt.Sprintf("https://wordpress.org/wordpress-%s.tar.gz", version)
	}
}

func (installer DefaultApplicationInstaller) downloadApplicationArchive(url string) (string, error) {
	client := installer.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}

	response, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("download application archive: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download application archive: unexpected HTTP status %s from %s", response.Status, url)
	}

	extension := ".tar.gz"
	if strings.HasSuffix(strings.ToLower(url), ".zip") {
		extension = ".zip"
	}

	archiveFile, err := os.CreateTemp("", "elk-local-app-*"+extension)
	if err != nil {
		return "", fmt.Errorf("create application archive file: %w", err)
	}
	defer archiveFile.Close()

	if _, err := io.Copy(archiveFile, response.Body); err != nil {
		return "", fmt.Errorf("write application archive: %w", err)
	}

	return archiveFile.Name(), nil
}

func detectExistingWordPress(projectRoot string) (InstalledApplication, bool, error) {
	versionPath := filepath.Join(projectRoot, "wp-includes", "version.php")
	contents, err := os.ReadFile(versionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return InstalledApplication{}, false, nil
		}
		return InstalledApplication{}, false, fmt.Errorf("read wordpress version file: %w", err)
	}

	version := "existing"
	if matches := wordPressVersionPattern.FindStringSubmatch(string(contents)); len(matches) == 2 {
		version = matches[1]
	}

	return InstalledApplication{Name: "WordPress", Version: version, Existing: true}, true, nil
}

func detectExistingComposerApplication(projectRoot string, presetName string, applicationName string) (InstalledApplication, bool, error) {
	markers := map[string][]string{
		"laravel": {"artisan", "composer.json"},
		"symfony": {filepath.Join("bin", "console"), "composer.json"},
	}

	requiredMarkers := markers[presetName]
	if len(requiredMarkers) == 0 {
		return InstalledApplication{}, false, nil
	}

	for _, marker := range requiredMarkers {
		if _, err := os.Stat(filepath.Join(projectRoot, marker)); err != nil {
			if os.IsNotExist(err) {
				return InstalledApplication{}, false, nil
			}
			return InstalledApplication{}, false, fmt.Errorf("inspect existing %s project: %w", applicationName, err)
		}
	}

	return InstalledApplication{Name: applicationName, Version: "existing", Existing: true}, true, nil
}

func ensureInstallableProjectRoot(projectRoot string, preset Preset) error {
	entries, err := os.ReadDir(projectRoot)
	if err != nil {
		return fmt.Errorf("inspect project root: %w", err)
	}

	meaningfulEntries := 0
	for _, entry := range entries {
		if entry.Name() == ".elk-local" {
			continue
		}
		meaningfulEntries++
	}

	if meaningfulEntries == 0 {
		return nil
	}

	return fmt.Errorf("project root %s is not empty; %s installation requires an empty directory or an existing compatible application", projectRoot, preset.ApplicationName)
}

func extractTarGzArchive(archivePath string, destinationDir string) error {
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open tar.gz archive: %w", err)
	}
	defer archiveFile.Close()

	gzipReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar archive: %w", err)
		}

		targetPath, err := archiveTargetPath(destinationDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create extracted directory %s: %w", targetPath, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create extracted file parent %s: %w", filepath.Dir(targetPath), err)
			}
			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create extracted file %s: %w", targetPath, err)
			}
			if _, err := io.Copy(file, tarReader); err != nil {
				_ = file.Close()
				return fmt.Errorf("extract file %s: %w", targetPath, err)
			}
			if err := file.Close(); err != nil {
				return fmt.Errorf("close extracted file %s: %w", targetPath, err)
			}
		}
	}
}

func extractZipArchive(archivePath string, destinationDir string) error {
	zipReader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip archive: %w", err)
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		targetPath, err := archiveTargetPath(destinationDir, file.Name)
		if err != nil {
			return err
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return fmt.Errorf("create extracted directory %s: %w", targetPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create extracted file parent %s: %w", filepath.Dir(targetPath), err)
		}

		reader, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", file.Name, err)
		}

		outputFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			_ = reader.Close()
			return fmt.Errorf("create extracted file %s: %w", targetPath, err)
		}

		if _, err := io.Copy(outputFile, reader); err != nil {
			_ = outputFile.Close()
			_ = reader.Close()
			return fmt.Errorf("extract zip file %s: %w", targetPath, err)
		}
		if err := outputFile.Close(); err != nil {
			_ = reader.Close()
			return fmt.Errorf("close extracted file %s: %w", targetPath, err)
		}
		if err := reader.Close(); err != nil {
			return fmt.Errorf("close zip entry %s: %w", file.Name, err)
		}
	}

	return nil
}

func archiveTargetPath(destinationDir string, archiveName string) (string, error) {
	targetPath := filepath.Join(destinationDir, filepath.FromSlash(archiveName))
	cleanDestination := filepath.Clean(destinationDir)
	cleanTarget := filepath.Clean(targetPath)
	if cleanTarget != cleanDestination && !strings.HasPrefix(cleanTarget, cleanDestination+string(filepath.Separator)) {
		return "", fmt.Errorf("archive entry %s escapes extraction root", archiveName)
	}

	return cleanTarget, nil
}

func copyDirectoryContents(sourceDir string, destinationDir string) error {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("read extracted source directory: %w", err)
	}

	for _, entry := range entries {
		sourcePath := filepath.Join(sourceDir, entry.Name())
		destinationPath := filepath.Join(destinationDir, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(destinationPath, 0o755); err != nil {
				return fmt.Errorf("create destination directory %s: %w", destinationPath, err)
			}
			if err := copyDirectoryContents(sourcePath, destinationPath); err != nil {
				return err
			}
			continue
		}

		if err := copyFileContents(sourcePath, destinationPath); err != nil {
			return err
		}
	}

	return nil
}

func copyFileContents(sourcePath string, destinationPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file %s: %w", sourcePath, err)
	}
	defer sourceFile.Close()

	info, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source file %s: %w", sourcePath, err)
	}

	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return fmt.Errorf("create destination parent %s: %w", filepath.Dir(destinationPath), err)
	}

	destinationFile, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
	if err != nil {
		return fmt.Errorf("create destination file %s: %w", destinationPath, err)
	}
	defer destinationFile.Close()

	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		return fmt.Errorf("copy file to %s: %w", destinationPath, err)
	}

	return nil
}

func runComposerCreateProject(projectRoot string, packageName string, version string) error {
	args := []string{"create-project", "--prefer-dist", packageName, "."}
	trimmedVersion := strings.TrimSpace(version)
	if trimmedVersion != "" && !strings.EqualFold(trimmedVersion, "latest") {
		args = append(args, trimmedVersion)
	}

	if composerPath, err := exec.LookPath("composer"); err == nil {
		command := exec.Command(composerPath, args...)
		command.Dir = projectRoot
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("run composer create-project: %w", err)
		}
		return nil
	}

	dockerArgs := []string{"run", "--rm"}
	if currentUser, err := user.Current(); err == nil && numericID(currentUser.Uid) && numericID(currentUser.Gid) {
		dockerArgs = append(dockerArgs, "-u", fmt.Sprintf("%s:%s", currentUser.Uid, currentUser.Gid))
	}
	dockerArgs = append(dockerArgs,
		"-v", fmt.Sprintf("%s:/app", projectRoot),
		"-w", "/app",
		"composer:2",
	)
	dockerArgs = append(dockerArgs, args...)

	command := exec.Command("docker", dockerArgs...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("run docker composer create-project: %w", err)
	}

	return nil
}

func numericID(value string) bool {
	if value == "" {
		return false
	}

	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}

	return true
}
