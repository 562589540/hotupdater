//go:build !windows
// +build !windows

package hotupdater

import "os/exec"

func ExecuteCommand(cmd string) bool {
	return exec.Command("sh", "-c", cmd).Run() == nil
}
