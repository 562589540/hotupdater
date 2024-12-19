//go:build darwin
// +build darwin

package hotupdater

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// MacUpdater macOS 更新器
type MacUpdater struct {
	config     Config
	ctx        context.Context
	luaState   *lua.LState
	currentExe string
	helper     *helper
}

func newPlatformUpdater(config Config, ctx context.Context) Updater {
	return newMacUpdater(config, ctx)
}

func newMacUpdater(config Config, ctx context.Context) *MacUpdater {
	exe, _ := os.Executable()
	m := &MacUpdater{
		config:     config,
		ctx:        ctx,
		luaState:   lua.NewState(),
		currentExe: exe,
		helper:     newHelper(config.Logger, config.EventEmitter),
	}

	// 添加初始化日志
	if m.config.Logger != nil {
		m.config.Logger.Log("Mac更新器初始化完成")
		m.config.Logger.Logf("当前执行文件: %s", exe)
	}

	return m
}

func (m *MacUpdater) Update(newVersion string) error {
	m.sendLog("开始更新...")
	m.sendLog("当前程序路径: %s", m.currentExe)
	m.sendLog("新版本路径: %s", newVersion)

	appRoot := m.getMacAppRoot()
	if appRoot == "" {
		m.sendLog("无法获取应用根目录")
		return fmt.Errorf("无法确定应用程序包路径")
	}
	m.sendLog("应用根目录: %s", appRoot)

	resourcesDir := filepath.Join(appRoot, "Contents", "Resources")
	m.sendLog("Resources目录: %s", resourcesDir)

	helperPath := filepath.Join(resourcesDir, "updater")
	m.sendLog("更新助手路径: %s", helperPath)
	if _, err := os.Stat(helperPath); os.IsNotExist(err) {
		m.sendLog("更新助手不存在: %s", helperPath)
		return fmt.Errorf("更新助手不存在: %s", helperPath)
	}
	m.sendLog("更新助手存在")

	scriptPath := filepath.Join(resourcesDir, "update.lua")
	m.sendLog("更新脚本路径: %s", scriptPath)
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		m.sendLog("更新脚本不存在: %s", scriptPath)
		return fmt.Errorf("更新脚本不存在: %s", scriptPath)
	}
	m.sendLog("更新脚本存在")

	updateInfo := filepath.Join(m.config.UpdatePath, "update_info.json")
	m.sendLog("更新信息文件路径: %s", updateInfo)

	params := map[string]string{
		"app_path":    m.currentExe,
		"new_version": newVersion,
		"backup_path": m.config.BackupPath,
		"update_path": m.config.UpdatePath,
		"app_root":    appRoot,
		"script_path": scriptPath,
	}

	if err := m.helper.writeUpdateInfo(updateInfo, params); err != nil {
		m.sendLog("写入更新信息失败: %v", err)
		return err
	}
	m.sendLog("更新信息已写入")

	m.sendLog("准备启动更新助手...")

	// 创建管道用于获取更新助手的输出
	pr, pw := io.Pipe()
	cmd := exec.CommandContext(m.ctx, helperPath, "--update", updateInfo)
	cmd.Stdout = pw
	cmd.Stderr = pw

	// 启动一个 goroutine 来读取输出
	go func() {
		defer pw.Close()
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			// 解析进度信息
			if strings.HasPrefix(line, "@PROGRESS@") {
				m.parseProgress(strings.TrimPrefix(line, "@PROGRESS@"))
			} else {
				m.sendLog("助手输出: %s", line)
			}
		}
	}()

	m.sendLog("正在启动更新助手...")
	if err := cmd.Start(); err != nil {
		m.sendLog("启动更新助手失败: %v", err)
		pw.Close()
		return err
	}

	m.sendLog("更新助手启动成功，等待更新完成...")

	// 等待命令完成
	err := cmd.Wait()
	if err != nil {
		m.sendLog("更新助手执行失败: %v", err)
		return err
	}

	// 检查上下文是否已取消
	if m.ctx.Err() != nil {
		m.sendLog("更新被取消: %v", m.ctx.Err())
		return m.ctx.Err()
	}

	m.sendLog("更新助手执行完成")
	return nil
}

func (m *MacUpdater) Close() {
	if m.luaState != nil {
		m.luaState.Close()
	}
}

func (m *MacUpdater) GetCurrentExe() string {
	return m.currentExe
}

func (m *MacUpdater) GetConfig() Config {
	return m.config
}

func (m *MacUpdater) getMacAppRoot() string {
	if idx := strings.Index(m.currentExe, ".app/"); idx != -1 {
		appRoot := m.currentExe[:idx+4]
		m.sendLog("Mac应用根目录: %s", appRoot)
		return appRoot
	}

	if m.config.BackupPath != "" {
		m.sendLog("使用配置的备份目录: %s", m.config.BackupPath)
		return m.config.BackupPath
	}

	currentDir, err := os.Getwd()
	if err != nil {
		m.sendLog("获取当前目录失败，使用临时目录")
		return os.TempDir()
	}

	backupDir := filepath.Join(currentDir, "backup")
	m.sendLog("使用默认备份目录: %s", backupDir)
	return backupDir
}

func (m *MacUpdater) sendLog(format string, args ...interface{}) {
	if m.config.Logger == nil {
		// 如果没有 logger，打印到标准输出
		fmt.Printf("WARNING: Logger not set - "+format+"\n", args...)
		return
	}
	m.config.Logger.Logf(format, args...)
}

// Restart 重启应用
func (m *MacUpdater) Restart() error {
	appRoot := m.getMacAppRoot()
	if appRoot == "" {
		return fmt.Errorf("无法获取应用根目录")
	}

	cmd := exec.Command("/usr/bin/open", "-n", appRoot)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}

// 解析进度信息
func (m *MacUpdater) parseProgress(data string) {
	progress := ParseProgressMessage(data)
	if progress == nil {
		return
	}

	// 发送进度事件
	if m.config.EventEmitter != nil {
		m.config.EventEmitter.EmitProgress(*progress)
	}
}
