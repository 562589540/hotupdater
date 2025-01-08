//go:build !windows

package main

import (
	"os/exec"
	"strings"
)

func RunCommand(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

// 设置UTF-8编码
func SetUTF8Encoding() {

}

// 请求管理员权限
func requestAdminPrivileges() error {
	return nil
}

// 检查进程是否在运行 (非 Windows 平台实现)
func isProcessRunning(processName string) bool {
	cmd := RunCommand("pgrep", processName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(output) > 0
}

// 检查是否以管理员权限运行 (非 Windows 平台实现)
func checkAdminPrivileges() bool {
	cmd := RunCommand("id", "-u")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "0"
}
