package environment

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const wordPressProjectRootInContainer = "/var/www/html"

type ToolProxyInvocation struct {
	Command string
	Args    []string
}

func FindManifestForWorkingDir(projectRoot string, workingDir string) (Manifest, bool, error) {
	resolvedWorkingDir, err := filepath.Abs(strings.TrimSpace(workingDir))
	if err != nil {
		return Manifest{}, false, fmt.Errorf("resolve working directory: %w", err)
	}

	manifests, err := ListManifests(projectRoot)
	if err != nil {
		return Manifest{}, false, err
	}

	bestMatch := Manifest{}
	bestMatchLength := -1
	for _, manifest := range manifests {
		projectRoot := filepath.Clean(strings.TrimSpace(manifest.Project.Root))
		if !samePath(resolvedWorkingDir, projectRoot) && !pathWithin(resolvedWorkingDir, projectRoot) {
			continue
		}

		if len(projectRoot) <= bestMatchLength {
			continue
		}

		bestMatch = manifest
		bestMatchLength = len(projectRoot)
	}

	if bestMatchLength == -1 {
		return Manifest{}, false, nil
	}

	return bestMatch, true, nil
}

func BuildToolProxyInvocation(manifest Manifest, toolName string, workingDir string, toolArgs []string, disableTTY bool) (ToolProxyInvocation, bool, error) {
	composeArgs := []string{"compose", "-f", manifest.Compose.File, "exec"}
	if disableTTY {
		composeArgs = append(composeArgs, "-T")
	}

	switch toolName {
	case "wp":
		if manifest.Project.Type != "wordpress" {
			return ToolProxyInvocation{}, false, nil
		}

		rewrittenArgs, err := rewriteWPCLIArgs(manifest, toolArgs)
		if err != nil {
			return ToolProxyInvocation{}, false, err
		}

		composeArgs = append(
			composeArgs,
			"-e", "XDEBUG_MODE="+wpCLIProxyXdebugMode(),
			"--user", "www-data",
			"-w", wordPressProjectRootInContainer,
			phpServiceName(manifest),
			"wp",
		)
		composeArgs = append(composeArgs, rewrittenArgs...)
		return ToolProxyInvocation{Command: "docker", Args: composeArgs}, true, nil
	case "mysql", "mariadb", "mysqldump", "mariadb-dump":
		composeArgs = append(
			composeArgs,
			databaseServiceHost(),
			"sh",
			"-lc",
			databaseProxyShellCommand(toolName),
			toolName,
		)
		composeArgs = append(composeArgs, rewriteDatabaseClientArgs(manifest, toolArgs)...)
		return ToolProxyInvocation{Command: "docker", Args: composeArgs}, true, nil
	default:
		return ToolProxyInvocation{}, false, nil
	}
}

func phpServiceName(manifest Manifest) string {
	if manifest.Runtime.WebServer == "nginx" {
		return "app"
	}

	return "web"
}

func wpCLIProxyXdebugMode() string {
	mode := strings.TrimSpace(os.Getenv("XDEBUG_MODE"))
	if mode == "" {
		return "off"
	}

	return mode
}

func rewriteWPCLIArgs(manifest Manifest, args []string) ([]string, error) {
	rewritten := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "--path":
			if index+1 >= len(args) {
				return nil, fmt.Errorf("wp --path requires a value")
			}

			rewritten = append(rewritten, argument)
			rewritten = append(rewritten, rewriteWordPressCLIPath(manifest, args[index+1]))
			index++
		case strings.HasPrefix(argument, "--path="):
			rewritten = append(rewritten, "--path="+rewriteWordPressCLIPath(manifest, strings.TrimPrefix(argument, "--path=")))
		default:
			rewritten = append(rewritten, argument)
		}
	}

	return rewritten, nil
}

func rewriteWordPressCLIPath(manifest Manifest, path string) string {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" || !filepath.IsAbs(trimmedPath) {
		return path
	}

	projectRoot := filepath.Clean(strings.TrimSpace(manifest.Project.Root))
	cleanPath := filepath.Clean(trimmedPath)
	if samePath(cleanPath, projectRoot) {
		return wordPressProjectRootInContainer
	}
	if !pathWithin(cleanPath, projectRoot) {
		return path
	}

	relativePath, err := filepath.Rel(projectRoot, cleanPath)
	if err != nil || relativePath == "." {
		return wordPressProjectRootInContainer
	}

	return filepath.ToSlash(filepath.Join(wordPressProjectRootInContainer, relativePath))
}

func rewriteDatabaseClientArgs(manifest Manifest, args []string) []string {
	rewritten := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "-h" || argument == "--host":
			if index+1 >= len(args) {
				rewritten = append(rewritten, argument)
				continue
			}

			rewritten = append(rewritten, argument)
			rewritten = append(rewritten, rewriteDatabaseHostValue(args[index+1]))
			index++
		case argument == "-P" || argument == "--port":
			if index+1 >= len(args) {
				rewritten = append(rewritten, argument)
				continue
			}

			rewritten = append(rewritten, argument)
			rewritten = append(rewritten, rewriteDatabasePortValue(manifest, args[index+1]))
			index++
		case strings.HasPrefix(argument, "--host="):
			rewritten = append(rewritten, "--host="+rewriteDatabaseHostValue(strings.TrimPrefix(argument, "--host=")))
		case strings.HasPrefix(argument, "--port="):
			rewritten = append(rewritten, "--port="+rewriteDatabasePortValue(manifest, strings.TrimPrefix(argument, "--port=")))
		case strings.HasPrefix(argument, "-h") && len(argument) > 2:
			rewritten = append(rewritten, "-h"+rewriteDatabaseHostValue(strings.TrimPrefix(argument, "-h")))
		case strings.HasPrefix(argument, "-P") && len(argument) > 2:
			rewritten = append(rewritten, "-P"+rewriteDatabasePortValue(manifest, strings.TrimPrefix(argument, "-P")))
		default:
			rewritten = append(rewritten, argument)
		}
	}

	return rewritten
}

func rewriteDatabaseHostValue(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", "db", "127.0.0.1", "localhost", "::1", "host.docker.internal":
		return "127.0.0.1"
	default:
		return value
	}
}

func rewriteDatabasePortValue(manifest Manifest, value string) string {
	if strings.TrimSpace(value) == strconv.Itoa(manifest.Network.DatabasePort) {
		return strconv.Itoa(databaseServicePort())
	}

	return value
}

func databaseProxyShellCommand(toolName string) string {
	switch toolName {
	case "mysqldump", "mariadb-dump":
		return `if command -v mariadb-dump >/dev/null 2>&1; then exec mariadb-dump "$@"; else exec mysqldump "$@"; fi`
	default:
		return `if command -v mariadb >/dev/null 2>&1; then exec mariadb "$@"; else exec mysql "$@"; fi`
	}
}
