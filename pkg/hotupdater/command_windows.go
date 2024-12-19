//go:build windows
// +build windows

package hotupdater

import (
	"os/exec"
	"syscall"
)

func ExecuteCommand(cmd string) bool {
	psCmd := exec.Command("powershell", "-Command", cmd)
	psCmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	return psCmd.Run() == nil
}
