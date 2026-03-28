package environment

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	tempHome, err := os.MkdirTemp("", "elk-local-environment-tests-")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "create temp HOME: %v\n", err)
		os.Exit(1)
	}

	if err := os.Setenv("HOME", tempHome); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "set HOME: %v\n", err)
		_ = os.RemoveAll(tempHome)
		os.Exit(1)
	}

	if err := os.Unsetenv("ELK_LOCAL_HOME"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unset ELK_LOCAL_HOME: %v\n", err)
		_ = os.RemoveAll(tempHome)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(tempHome)
	os.Exit(code)
}
