package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"elk-local/internal/config"
)

var ErrNotRunning = errors.New("daemon not running")

type StartOptions struct {
	Host        string
	Port        int
	ProjectRoot string
	WebUIDist   string
}

type Status struct {
	State   State
	Remote  RemoteStatus
	Running bool
	Stale   bool
	Error   string
}

func Start(options StartOptions) (Status, error) {
	resolvedProjectRoot, err := ResolveProjectRoot(options.ProjectRoot)
	if err != nil {
		return Status{}, err
	}

	options.ProjectRoot = resolvedProjectRoot
	options.Host = firstNonEmpty(options.Host, "127.0.0.1")
	if options.Port == 0 {
		options.Port, err = config.ResolveWebUIPort(0)
		if err != nil {
			return Status{}, err
		}
	}

	status, err := Inspect(resolvedProjectRoot)
	if err != nil {
		return Status{}, err
	}
	if status.Running {
		return Status{}, fmt.Errorf("daemon already running on %s", status.State.BaseURL())
	}
	if status.State.PID != 0 && !status.Running {
		_ = RemoveState(resolvedProjectRoot)
	}

	logPath := DefaultLogPath(resolvedProjectRoot)
	pid, err := startDetachedProcess(buildServeArgs(options), logPath)
	if err != nil {
		return Status{}, fmt.Errorf("start daemon process: %w", err)
	}

	state := State{
		PID:         pid,
		Host:        options.Host,
		Port:        options.Port,
		ProjectRoot: resolvedProjectRoot,
		LogFile:     logPath,
		StaticDir:   resolveStaticDir(options.WebUIDist),
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := SaveState(resolvedProjectRoot, state); err != nil {
		_ = signalProcess(pid)
		return Status{}, err
	}

	remote, err := WaitUntilReachable(state, 12*time.Second)
	if err != nil {
		_ = RemoveState(resolvedProjectRoot)
		_ = signalProcess(pid)
		return Status{}, fmt.Errorf("daemon did not become reachable: %w (see %s)", err, logPath)
	}

	if remote.StaticDir != "" {
		state.StaticDir = remote.StaticDir
		if err := SaveState(resolvedProjectRoot, state); err != nil {
			return Status{}, err
		}
	}

	return Status{State: state, Remote: remote, Running: true}, nil
}

func Restart(options StartOptions) (Status, error) {
	if _, err := Stop(options.ProjectRoot); err != nil && !errors.Is(err, ErrNotRunning) {
		return Status{}, err
	}

	return Start(options)
}

func Stop(projectRoot string) (Status, error) {
	status, err := Inspect(projectRoot)
	if err != nil {
		return Status{}, err
	}
	if status.State.PID == 0 {
		return Status{}, ErrNotRunning
	}

	if status.Running {
		if err := Shutdown(status.State, 2*time.Second); err != nil {
			return status, fmt.Errorf("request daemon shutdown: %w", err)
		}
		if err := WaitUntilStopped(status.State, 12*time.Second); err != nil {
			if processExists(status.State.PID) {
				if signalErr := signalProcess(status.State.PID); signalErr != nil {
					return status, fmt.Errorf("wait for daemon shutdown: %w", err)
				}
				if waitErr := waitForProcessExit(status.State.PID, 5*time.Second); waitErr != nil {
					return status, fmt.Errorf("wait for daemon shutdown: %w", err)
				}
			} else {
				return status, fmt.Errorf("wait for daemon shutdown: %w", err)
			}
		}
	} else if processExists(status.State.PID) {
		if err := signalProcess(status.State.PID); err != nil {
			return status, fmt.Errorf("stop unreachable daemon process: %w", err)
		}
		if err := waitForProcessExit(status.State.PID, 5*time.Second); err != nil {
			return status, err
		}
	}

	if err := RemoveState(status.State.ProjectRoot); err != nil {
		return status, err
	}

	status.Running = false
	status.Stale = false
	status.Error = ""
	return status, nil
}

func Inspect(projectRoot string) (Status, error) {
	resolvedProjectRoot, err := ResolveProjectRoot(projectRoot)
	if err != nil {
		return Status{}, err
	}

	state, err := LoadState(resolvedProjectRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return Status{}, nil
		}
		return Status{}, err
	}

	status := Status{State: state}
	remote, err := QueryStatus(state, 2*time.Second)
	if err == nil && remote.OK {
		status.Remote = remote
		status.Running = true
		return status, nil
	}

	status.Stale = true
	if err != nil {
		status.Error = err.Error()
	}
	if !processExists(state.PID) {
		_ = RemoveState(resolvedProjectRoot)
	}

	return status, nil
}

func buildServeArgs(options StartOptions) []string {
	args := []string{
		"serve",
		"--host", options.Host,
		"--port", strconv.Itoa(options.Port),
		"--project-root", options.ProjectRoot,
	}
	if strings.TrimSpace(options.WebUIDist) != "" {
		args = append(args, "--webui-dist", options.WebUIDist)
	}

	return args
}

func startDetachedProcess(args []string, logPath string) (int, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return 0, fmt.Errorf("create daemon log directory: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open daemon log file: %w", err)
	}

	executable, err := os.Executable()
	if err != nil {
		_ = logFile.Close()
		return 0, fmt.Errorf("resolve executable path: %w", err)
	}

	command := exec.Command(executable, args...)
	command.Stdout = logFile
	command.Stderr = logFile
	command.Stdin = nil
	command.Env = os.Environ()
	configureBackgroundProcess(command)

	if err := command.Start(); err != nil {
		_ = logFile.Close()
		return 0, err
	}

	pid := command.Process.Pid
	_ = logFile.Close()
	_ = command.Process.Release()

	return pid, nil
}

func resolveStaticDir(webuiDist string) string {
	if strings.TrimSpace(webuiDist) == "" {
		return ""
	}

	absPath, err := filepath.Abs(webuiDist)
	if err != nil {
		return ""
	}

	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		return ""
	}

	return filepath.Clean(absPath)
}

func waitForProcessExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	return fmt.Errorf("daemon process %d did not exit within %s", pid, timeout)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}
