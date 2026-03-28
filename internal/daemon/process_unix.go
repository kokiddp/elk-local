//go:build !windows

package daemon

import (
	"os/exec"
	"syscall"
)

func configureBackgroundProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func signalProcess(pid int) error {
	if pid <= 0 {
		return nil
	}

	return syscall.Kill(pid, syscall.SIGTERM)
}
