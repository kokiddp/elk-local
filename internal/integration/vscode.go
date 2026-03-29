package integration

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type VSCodeOpener struct {
	LookPath    func(file string) (string, error)
	Start       func(name string, args []string) error
	IsWSL       func() bool
	WSLDistro   string
	Environment map[string]string
}

func (opener VSCodeOpener) OpenFolder(path string) error {
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

func (opener VSCodeOpener) commandFor(path string) (string, []string, error) {
	resolvedPath, err := filepath.Abs(path)
	if err != nil {
		return "", nil, fmt.Errorf("resolve folder path: %w", err)
	}

	lookPath := opener.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}

	if opener.isWSL() {
		for _, candidate := range []string{"code", "code-insiders"} {
			if executable, err := lookPath(candidate); err == nil {
				return executable, []string{resolvedPath}, nil
			}
		}

		folderURI := opener.wslFolderURI(resolvedPath)
		if folderURI != "" {
			for _, candidate := range []string{"code.cmd", "code-insiders.cmd"} {
				if executable, err := lookPath(candidate); err == nil {
					return executable, []string{"--folder-uri", folderURI}, nil
				}
			}
		}
	}

	for _, candidate := range []string{"code", "code-insiders", "code.cmd", "code-insiders.cmd"} {
		if executable, err := lookPath(candidate); err == nil {
			return executable, []string{resolvedPath}, nil
		}
	}

	return "", nil, fmt.Errorf("could not find VS Code on PATH; install the code command or VS Code WSL integration first")
}

func (opener VSCodeOpener) isWSL() bool {
	if opener.IsWSL != nil {
		return opener.IsWSL()
	}

	contents, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return false
	}

	return strings.Contains(strings.ToLower(string(contents)), "microsoft")
}

func (opener VSCodeOpener) wslFolderURI(path string) string {
	distroName := strings.TrimSpace(opener.WSLDistro)
	if distroName == "" {
		distroName = strings.TrimSpace(opener.getenv("WSL_DISTRO_NAME"))
	}
	if distroName == "" {
		return ""
	}

	uri := &url.URL{
		Scheme: "vscode-remote",
		Host:   "wsl+" + distroName,
		Path:   filepath.ToSlash(path),
	}
	return uri.String()
}

func (opener VSCodeOpener) getenv(key string) string {
	if opener.Environment != nil {
		return opener.Environment[key]
	}

	return os.Getenv(key)
}

func defaultStartProcess(name string, args []string) error {
	command := exec.Command(name, args...)
	if err := command.Start(); err != nil {
		return err
	}

	return command.Process.Release()
}
