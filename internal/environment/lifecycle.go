package environment

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type ComposeExecutor interface {
	Run(composeFile string, args ...string) (string, error)
}

type DockerComposeExecutor struct{}

func (DockerComposeExecutor) Run(composeFile string, args ...string) (string, error) {
	return runDockerCompose(composeFile, nil, args...)
}

func (DockerComposeExecutor) RunWithInput(composeFile string, input []byte, args ...string) (string, error) {
	return runDockerCompose(composeFile, bytes.NewReader(input), args...)
}

func runDockerCompose(composeFile string, stdin io.Reader, args ...string) (string, error) {
	commandArgs := append([]string{"compose", "-f", composeFile}, args...)
	command := exec.Command("docker", commandArgs...)
	if stdin != nil {
		command.Stdin = stdin
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	combined := strings.TrimSpace(stdout.String())
	if stderr.Len() > 0 {
		if combined != "" {
			combined += "\n"
		}
		combined += strings.TrimSpace(stderr.String())
	}

	if err != nil {
		return combined, fmt.Errorf("run docker compose: %w", err)
	}

	return combined, nil
}

type LifecycleService struct {
	Executor ComposeExecutor
}

func NewLifecycleService(executor ComposeExecutor) LifecycleService {
	if executor == nil {
		executor = DockerComposeExecutor{}
	}

	return LifecycleService{Executor: executor}
}

func (service LifecycleService) Start(manifest Manifest) (string, error) {
	return service.Executor.Run(manifest.Compose.File, "up", "-d")
}

func (service LifecycleService) Stop(manifest Manifest) (string, error) {
	return service.Executor.Run(manifest.Compose.File, "stop")
}

func (service LifecycleService) Destroy(manifest Manifest) (string, error) {
	return service.Executor.Run(manifest.Compose.File, "down", "--remove-orphans")
}

func (service LifecycleService) Status(manifest Manifest) (string, error) {
	return service.Executor.Run(manifest.Compose.File, "ps", "--format", "json")
}
