package integration

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

type SystemFolderOpener struct {
	LookPath func(file string) (string, error)
	Start    func(name string, args []string) error
	GOOS     string
	IsWSL    func() bool
}

func (opener SystemFolderOpener) OpenFolder(path string) error {
	command, args, err := opener.commandFor(path)
	if err != nil {
		return err
	}

	start := opener.Start
	if start == nil {
		start = defaultStartProcess
	}

	return start(command, args)
}

func (opener SystemFolderOpener) commandFor(path string) (string, []string, error) {
	resolvedPath, err := filepath.Abs(path)
	if err != nil {
		return "", nil, fmt.Errorf("resolve folder path: %w", err)
	}

	lookPath := opener.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}

	if opener.isWSL() {
		if command, args, err := findFolderOpenCommand(lookPath, []string{"wslview", "explorer.exe"}, resolvedPath); err == nil {
			return command, args, nil
		}
	}

	switch opener.goos() {
	case "darwin":
		return findFolderOpenCommand(lookPath, []string{"open"}, resolvedPath)
	case "windows":
		return findFolderOpenCommand(lookPath, []string{"explorer.exe", "explorer"}, resolvedPath)
	default:
		return findFolderOpenCommand(lookPath, []string{"xdg-open"}, resolvedPath)
	}
}

func (opener SystemFolderOpener) goos() string {
	if opener.GOOS != "" {
		return opener.GOOS
	}

	return runtime.GOOS
}

func (opener SystemFolderOpener) isWSL() bool {
	if opener.IsWSL != nil {
		return opener.IsWSL()
	}

	return VSCodeOpener{}.isWSL()
}

func findFolderOpenCommand(lookPath func(file string) (string, error), candidates []string, resolvedPath string) (string, []string, error) {
	for _, candidate := range candidates {
		if executable, err := lookPath(candidate); err == nil {
			return executable, []string{resolvedPath}, nil
		}
	}

	return "", nil, fmt.Errorf("could not find a system file explorer command on PATH")
}
