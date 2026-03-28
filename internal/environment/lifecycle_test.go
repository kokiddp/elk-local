package environment

import (
	"strings"
	"testing"
)

type fakeComposeExecutor struct {
	composeFile string
	args        []string
	output      string
	err         error
}

func (executor *fakeComposeExecutor) Run(composeFile string, args ...string) (string, error) {
	executor.composeFile = composeFile
	executor.args = append([]string{}, args...)
	return executor.output, executor.err
}

func TestLifecycleStartUsesComposeUpDetached(t *testing.T) {
	t.Parallel()

	executor := &fakeComposeExecutor{output: "started"}
	service := NewLifecycleService(executor)
	manifest := Manifest{Compose: Compose{File: "/tmp/example-compose.yaml"}}

	output, err := service.Start(manifest)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if output != "started" {
		t.Fatalf("unexpected output: %s", output)
	}

	if executor.composeFile != manifest.Compose.File {
		t.Fatalf("unexpected compose file: %s", executor.composeFile)
	}

	if strings.Join(executor.args, " ") != "up -d" {
		t.Fatalf("unexpected args: %v", executor.args)
	}
}

func TestLifecycleStatusUsesComposePSJson(t *testing.T) {
	t.Parallel()

	executor := &fakeComposeExecutor{output: "[]"}
	service := NewLifecycleService(executor)
	manifest := Manifest{Compose: Compose{File: "/tmp/example-compose.yaml"}}

	if _, err := service.Status(manifest); err != nil {
		t.Fatalf("status: %v", err)
	}

	if strings.Join(executor.args, " ") != "ps --format json" {
		t.Fatalf("unexpected args: %v", executor.args)
	}
}
