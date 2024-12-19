//go:build windows
// +build windows

package hotupdater

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	lua "github.com/yuin/gopher-lua"
)

// WinUpdater Windows 更新器
type WinUpdater struct {
	config     Config
	ctx        context.Context
	luaState   *lua.LState
	currentExe string
	helper     *helper
}

func newPlatformUpdater(config Config, ctx context.Context) Updater {
	return newWinUpdater(config, ctx)
}

func newWinUpdater(config Config, ctx context.Context) *WinUpdater {
	exe, _ := os.Executable()
	return &WinUpdater{
		config:     config,
		ctx:        ctx,
		luaState:   lua.NewState(),
		currentExe: exe,
		helper:     newHelper(config.Logger, config.EventEmitter),
	}
}

func (w *WinUpdater) Update(newVersion string) error {
	w.sendLog("当前程序路径: %s", w.currentExe)
	w.sendLog("新版本路径: %s", newVersion)

	// 构建更新参数
	params := map[string]string{
		"app_path":    w.currentExe,
		"new_version": newVersion,
		"backup_path": w.config.BackupPath,
		"update_path": w.config.UpdatePath,
		"app_root":    filepath.Dir(w.currentExe), // Windows 使用可执行文件所在目录作为根目录
		"script_path": w.config.ScriptPath,
	}

	// 执行更新脚本
	if err := w.helper.executeLuaScript(w.luaState, w.config.ScriptPath, params); err != nil {
		return fmt.Errorf("执行更新脚本失败: %v", err)
	}
	return nil
}

func (w *WinUpdater) Close() {
	if w.luaState != nil {
		w.luaState.Close()
	}
}

func (w *WinUpdater) GetCurrentExe() string {
	return w.currentExe
}

func (w *WinUpdater) GetConfig() Config {
	return w.config
}

func (w *WinUpdater) Restart() error {
	// Windows 下不需要重启，批处理脚本会处理
	return nil
}

func (w *WinUpdater) sendLog(format string, args ...interface{}) {
	if w.config.Logger == nil {
		fmt.Printf("WARNING: Logger not set - "+format+"\n", args...)
		return
	}
	w.config.Logger.Logf(format, args...)
}

// 添加解析进度的方法
func (w *WinUpdater) parseProgress(data string) {
	progress := ParseProgressMessage(data)
	if progress == nil {
		return
	}

	// 发送进度事件
	if w.config.EventEmitter != nil {
		w.config.EventEmitter.EmitProgress(*progress)
	}
}
