//go:build darwin
// +build darwin

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func init() {
	// 设置日志格式，不包含时间戳
	log.SetFlags(0) // 移除所有标志，包括时间戳
}

type UpdateInfo struct {
	AppPath    string `json:"app_path"`
	NewVersion string `json:"new_version"`
	ScriptPath string `json:"script_path"`
	BackupPath string `json:"backup_path"`
	UpdatePath string `json:"update_path"`
	DoneFile   string `json:"done_file"`
}

func main() {
	log.Printf("更新助手启动，进程ID: %d", os.Getpid())
	updateFile := flag.String("update", "", "更新信息文件路径")
	pipePath := flag.String("pipe", "", "命名管道路径")
	flag.Parse()

	if *updateFile == "" {
		log.Fatal("需要提供更新信息文件路径")
		os.Exit(1)
	}

	// 创建根 context 用于管理所有协程
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // 确保在主函数退出时取消所有协程

	// 添加自检测机制
	if *pipePath != "" {
		// 这是提权后的进程
		go func() {
			// 每30秒检查一次父进程
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			ppid := os.Getppid()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// 检查父进程是否还存在
					if _, err := os.FindProcess(ppid); err != nil {
						log.Printf("父进程已退出，更新助手即将退出")
						cancel() // 取消所有协程
						os.Exit(1)
					}

					// 检查更新是否已完成
					if _, err := os.Stat(*updateFile); err != nil {
						log.Printf("更新文件已不存在，更新助手即将退出")
						cancel() // 取消所有协程
						os.Exit(0)
					}
				}
			}
		}()

		// 打开命名管道写入端
		pipe, err := os.OpenFile(*pipePath, os.O_WRONLY, os.ModeNamedPipe)
		if err != nil {
			log.Fatalf("打开管道失败: %v", err)
		}
		defer pipe.Close()
		// 重定向标准输出到管道
		os.Stdout = pipe
	}

	// 运行更新，传入 context
	if err := runUpdate(ctx, *updateFile); err != nil {
		log.Printf("更新失败: %v\n", err)
		os.Exit(1)
	}
}

func runUpdate(ctx context.Context, updateFile string) error {
	log.Printf("读取更新信息文件: %s", updateFile)
	info, err := readUpdateInfo(updateFile)
	if err != nil {
		return err
	}

	// 请求提升权限
	if os.Geteuid() != 0 {
		log.Println("请求管理员权限...")
		return requestPrivileges(updateFile)
	}

	log.Printf("当前进程已获得管理员权限")
	log.Printf("等待原应用退出...")
	time.Sleep(2 * time.Second)

	// 在执行更新脚本前检查 context 是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// 继续执行
	}

	// 执行 Lua 更新脚本
	log.Printf("开始执行更新脚本...")
	if err := executeLuaScript(ctx, info); err != nil {
		return fmt.Errorf("执行更新脚本失败: %v", err)
	}

	// 设置权限
	appRoot := getAppRoot(info.AppPath)
	if err := setPermissions(appRoot); err != nil {
		log.Printf("设置权限失败: %v", err)
	}

	log.Printf("更新完成，准备重启应用")
	return nil
}

func executeLuaScript(ctx context.Context, info *UpdateInfo) error {
	log.Printf("验证更新信息...")
	// 获取真实的应用路径
	appRoot := getAppRoot(info.AppPath)
	if _, err := os.Stat(appRoot); err != nil {
		return fmt.Errorf("当前应用路径无效: %v", err)
	}
	if _, err := os.Stat(info.NewVersion); err != nil {
		return fmt.Errorf("新版本路径无效: %v", err)
	}

	// 使用传入的脚本路径
	scriptPath := info.ScriptPath
	log.Printf("更新脚本路径: %s", scriptPath)
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("更新脚本不存在: %v", err)
	}

	log.Printf("初始化 Lua 环境...")
	L := lua.NewState()
	defer L.Close()

	log.Printf("执行更新脚本: %s", scriptPath)
	if err := L.DoFile(scriptPath); err != nil {
		return fmt.Errorf("加载脚本失败: %v", err)
	}

	// 构建参数
	params := map[string]string{
		"app_path":    info.AppPath,
		"new_version": info.NewVersion,
		"backup_path": info.BackupPath,
		"update_path": info.UpdatePath,
		"app_root":    appRoot,
	}

	log.Printf("调用更新函数...")
	// 创建参数表
	paramsTable := L.NewTable()
	for k, v := range params {
		L.SetField(paramsTable, k, lua.LString(v))
	}

	// 注册日志函数，直接输出到标准输出
	L.SetGlobal("log_message", L.NewFunction(func(L *lua.LState) int {
		msg := L.ToString(1)
		fmt.Printf("%s\n", msg) // 输出到标准输出，会被 mac_updater 捕获
		return 0
	}))

	// 在执行 Lua 脚本期间定期检查 context
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				L.Close() // 强制关闭 Lua 状态
				return
			case <-ticker.C:
				// 继续检查
			}
		}
	}()

	return L.CallByParam(lua.P{
		Fn:      L.GetGlobal("perform_update"),
		NRet:    0,
		Protect: true,
	}, paramsTable)
}

func readUpdateInfo(path string) (*UpdateInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取更新信息失败: %v", err)
	}

	var info UpdateInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("解析更新信息失败: %v", err)
	}

	return &info, nil
}

func requestPrivileges(updateFile string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	// 创建命名管道前先检查并清理
	pipePath := filepath.Join(filepath.Dir(updateFile), "updater.pipe")
	if _, err := os.Stat(pipePath); err == nil {
		// 管道已存在，先尝试删除
		if err := os.Remove(pipePath); err != nil {
			log.Printf("警告: 清理旧管道失败: %v", err)
			// 继续执行，因为可能是权限问题，新建时会用新的权限
		}
	}

	// 创建命名管道
	if err := syscall.Mkfifo(pipePath, 0666); err != nil {
		return fmt.Errorf("创建命名管道失败: %v", err)
	}
	defer os.Remove(pipePath)

	// 启动管道监控
	pipeCtx, pipeCancel := context.WithCancel(context.Background())
	defer pipeCancel()

	go func() {
		pipe, err := os.OpenFile(pipePath, os.O_RDONLY, os.ModeNamedPipe)
		if err != nil {
			log.Printf("打开管道失败: %v", err)
			return
		}
		defer pipe.Close()

		// 创建一个定时器用于检测管道是否活跃
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		scanner := bufio.NewScanner(pipe)
		for {
			select {
			case <-pipeCtx.Done():
				return
			case <-ticker.C:
				// 如果30秒没有数据，认为可能出现问题
				log.Printf("警告: 管道30秒未收到数据")
			default:
				if !scanner.Scan() {
					if err := scanner.Err(); err != nil {
						log.Printf("读取管道错误: %v", err)
					}
					return
				}
				fmt.Println(scanner.Text())
				ticker.Reset(30 * time.Second) // 重置定时器
			}
		}
	}()

	script := fmt.Sprintf(
		`do shell script "'%s' --update '%s' --pipe '%s'" with administrator privileges`,
		exe, updateFile, pipePath)

	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

// 将权限设置抽取为单独的函数
func setPermissions(appRoot string) error {
	log.Printf("设置应用权限...")
	// 设置整个应用包的权限
	if err := os.Chmod(appRoot, 0755); err != nil {
		return fmt.Errorf("设置应用权限失败: %v", err)
	}

	// 特别处理 MacOS 目录
	macosPath := filepath.Join(appRoot, "Contents", "MacOS")
	files, err := os.ReadDir(macosPath)
	if err != nil {
		return fmt.Errorf("读取 MacOS 目录失败: %v", err)
	}

	for _, file := range files {
		fullPath := filepath.Join(macosPath, file.Name())
		if err := os.Chmod(fullPath, 0755); err != nil {
			return fmt.Errorf("设置执行权限失败 %s: %v", fullPath, err)
		}
	}

	return nil
}

// 添加一个新的辅助函数来获取应用根目录
func getAppRoot(path string) string {
	if idx := strings.Index(path, ".app/"); idx != -1 {
		return path[:idx+4]
	}
	return path
}
