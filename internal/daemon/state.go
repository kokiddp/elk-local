package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"elk-local/internal/config"
)

type State struct {
	PID         int    `json:"pid"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	ProjectRoot string `json:"projectRoot"`
	LogFile     string `json:"logFile"`
	StaticDir   string `json:"staticDir,omitempty"`
	StartedAt   string `json:"startedAt"`
}

type RemoteStatus struct {
	OK          bool   `json:"ok"`
	ProjectRoot string `json:"projectRoot"`
	StaticDir   string `json:"staticDir,omitempty"`
}

func ResolveProjectRoot(projectRoot string) (string, error) {
	resolvedPath, err := config.ResolveRegistryRoot(projectRoot)
	if err != nil {
		return "", err
	}

	return filepath.Clean(resolvedPath), nil
}

func Dir(projectRoot string) string {
	return filepath.Join(projectRoot, ".elk-local", "daemon")
}

func StatePath(projectRoot string) string {
	return filepath.Join(Dir(projectRoot), "state.json")
}

func DefaultLogPath(projectRoot string) string {
	return filepath.Join(Dir(projectRoot), "daemon.log")
}

func SaveState(projectRoot string, state State) error {
	if err := os.MkdirAll(Dir(projectRoot), 0o755); err != nil {
		return fmt.Errorf("create daemon directory: %w", err)
	}

	contents, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal daemon state: %w", err)
	}

	if err := os.WriteFile(StatePath(projectRoot), append(contents, '\n'), 0o644); err != nil {
		return fmt.Errorf("write daemon state: %w", err)
	}

	return nil
}

func LoadState(projectRoot string) (State, error) {
	contents, err := os.ReadFile(StatePath(projectRoot))
	if err != nil {
		return State{}, err
	}

	var state State
	if err := json.Unmarshal(contents, &state); err != nil {
		return State{}, fmt.Errorf("decode daemon state: %w", err)
	}

	return state, nil
}

func RemoveState(projectRoot string) error {
	err := os.Remove(StatePath(projectRoot))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove daemon state: %w", err)
	}
	return nil
}

func BaseURL(host string, port int) string {
	return fmt.Sprintf("http://%s:%d", host, port)
}

func (state State) BaseURL() string {
	return BaseURL(state.Host, state.Port)
}

func QueryStatus(state State, timeout time.Duration) (RemoteStatus, error) {
	client := &http.Client{Timeout: timeout}
	response, err := client.Get(state.BaseURL() + "/api/daemon/status")
	if err != nil {
		return RemoteStatus{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return RemoteStatus{}, fmt.Errorf("daemon status endpoint returned %s", response.Status)
	}

	var status RemoteStatus
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		return RemoteStatus{}, fmt.Errorf("decode daemon status: %w", err)
	}

	return status, nil
}

func Shutdown(state State, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	request, err := http.NewRequest(http.MethodPost, state.BaseURL()+"/api/daemon/shutdown", nil)
	if err != nil {
		return err
	}

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("daemon shutdown endpoint returned %s", response.Status)
	}

	return nil
}

func WaitUntilReachable(state State, timeout time.Duration) (RemoteStatus, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		status, err := QueryStatus(state, 2*time.Second)
		if err == nil && status.OK {
			return status, nil
		}
		lastErr = err
		time.Sleep(250 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("daemon did not become reachable")
	}
	return RemoteStatus{}, lastErr
}

func WaitUntilStopped(state State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := QueryStatus(state, 500*time.Millisecond); err != nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	return fmt.Errorf("daemon remained reachable after shutdown timeout")
}
