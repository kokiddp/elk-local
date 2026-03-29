package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"elk-local/internal/environment"

	"github.com/spf13/cobra"
)

func newProxyCommand() *cobra.Command {
	command := &cobra.Command{
		Use:                "proxy TOOL [args...]",
		Hidden:             true,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("tool name is required")
			}

			return runProxy(args[0], args[1:])
		},
	}

	return command
}

func runProxy(toolName string, toolArgs []string) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	manifest, found, err := environment.FindManifestForWorkingDir("", workingDir)
	if err != nil {
		return err
	}

	if found {
		invocation, supported, err := environment.BuildToolProxyInvocation(manifest, toolName, workingDir, toolArgs, stdioNeedsNoTTY())
		if err != nil {
			return err
		}
		if supported {
			return runPassthroughCommand(invocation.Command, invocation.Args...)
		}
	}

	fallbackExecutable, err := fallbackExecutablePath(toolName)
	if err != nil {
		return fmt.Errorf("no ELK-managed %s proxy target was found for %s and no host executable is available in PATH", toolName, workingDir)
	}

	return runPassthroughCommand(fallbackExecutable, toolArgs...)
}

func runPassthroughCommand(commandName string, args ...string) error {
	command := exec.Command(commandName, args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func stdioNeedsNoTTY() bool {
	return !isCharDevice(os.Stdin) || !isCharDevice(os.Stdout) || !isCharDevice(os.Stderr)
}

func isCharDevice(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

func fallbackExecutablePath(toolName string) (string, error) {
	currentExecutable, err := os.Executable()
	if err != nil {
		return "", err
	}

	proxyDir := filepath.Clean(filepath.Dir(currentExecutable))
	for _, directory := range filepath.SplitList(os.Getenv("PATH")) {
		trimmedDirectory := strings.TrimSpace(directory)
		if trimmedDirectory == "" {
			continue
		}

		candidateDir, err := filepath.Abs(trimmedDirectory)
		if err != nil {
			continue
		}
		if filepath.Clean(candidateDir) == proxyDir {
			continue
		}

		candidatePath := filepath.Join(candidateDir, toolName)
		info, err := os.Stat(candidatePath)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}

		return candidatePath, nil
	}

	return "", fmt.Errorf("%s not found", toolName)
}
