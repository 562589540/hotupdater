//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

func RunCommand(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	return cmd
}

// 设置UTF-8编码
func SetUTF8Encoding() {
	cmd := RunCommand("cmd", "/C", "chcp 65001 >nul")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// 请求管理员权限 (Windows 特定实现)
func requestAdminPrivileges() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取程序路径失败: %v", err)
	}

	verb := "runas"
	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString("")
	argPtr, _ := syscall.UTF16PtrFromString("")

	var showCmd int32 = 1 // SW_NORMAL

	// 使用 ShellExecute 启动程序
	ret, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW").Call(
		0,
		uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(exePtr)),
		uintptr(unsafe.Pointer(argPtr)),
		uintptr(unsafe.Pointer(cwdPtr)),
		uintptr(showCmd))

	if ret <= 32 { // ShellExecute returns a value greater than 32 if successful
		return fmt.Errorf("请求管理员权限失败，错误码: %d", ret)
	}
	return nil
}

// 检查进程是否在运行 (Windows 特定实现)
func isProcessRunning(processName string) bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	createSnapshot := kernel32.NewProc("CreateToolhelp32Snapshot")
	process32First := kernel32.NewProc("Process32FirstW")
	process32Next := kernel32.NewProc("Process32NextW")
	closeHandle := kernel32.NewProc("CloseHandle")

	handle, _, _ := createSnapshot.Call(0x2, 0) // TH32CS_SNAPPROCESS = 0x2
	if handle == uintptr(syscall.InvalidHandle) {
		return false
	}
	defer closeHandle.Call(handle)

	var entry PROCESSENTRY32
	entry.dwSize = uint32(unsafe.Sizeof(entry))

	ret, _, _ := process32First.Call(handle, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return false
	}

	for {
		name := syscall.UTF16ToString(entry.szExeFile[:])
		if strings.EqualFold(name, processName) {
			return true
		}

		ret, _, _ := process32Next.Call(handle, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	return false
}

// Windows 进程结构体
type PROCESSENTRY32 struct {
	dwSize              uint32
	cntUsage            uint32
	th32ProcessID       uint32
	th32DefaultHeapID   uintptr
	th32ModuleID        uint32
	cntThreads          uint32
	th32ParentProcessID uint32
	pcPriClassBase      int32
	dwFlags             uint32
	szExeFile           [260]uint16
}

// 检查是否以管理员权限运行 (Windows 特定实现)
func checkAdminPrivileges() bool {
	cmd := RunCommand("net", "session")
	err := cmd.Run()
	return err == nil
}
