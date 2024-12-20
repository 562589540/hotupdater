package updatertest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func main() {
	// 获取测试脚本路径
	scriptPath := filepath.Join("test_quarantine.lua")
	if _, err := os.Stat(scriptPath); err != nil {
		fmt.Printf("找不到测试脚本: %v\n", err)
		os.Exit(1)
	}

	L := lua.NewState()
	defer L.Close()

	// 注册日志函数
	L.SetGlobal("log_message", L.NewFunction(func(L *lua.LState) int {
		msg := L.ToString(1)
		fmt.Printf("%s\n", msg)
		return 0
	}))

	// 设置超时
	done := make(chan bool)
	go func() {
		if err := L.DoFile(scriptPath); err != nil {
			fmt.Printf("执行测试脚本失败: %v\n", err)
		}
		done <- true
	}()

	// 等待完成或超时
	select {
	case <-done:
		fmt.Println("测试完成")
	case <-time.After(30 * time.Second):
		fmt.Println("错误: 测试超时!")
		// 打印进程状态
		fmt.Println("\n=== 进程状态 ===")
		cmd := exec.Command("ps", "aux")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}
