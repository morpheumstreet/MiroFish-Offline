//go:build windows

package simrunner

import (
	"os"
	"os/exec"
)

func setProcGroup(cmd *exec.Cmd) {}

func procAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = p // Windows: no cheap liveness check
	return true
}

func unixKillGroup(pid int) error {
	if pid <= 0 {
		return nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
