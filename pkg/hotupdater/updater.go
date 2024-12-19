package hotupdater

import (
	"context"
	"fmt"
	"os"
	"time"
)

// 事件名称常量
const (
	EventLog      = "log"             // 日志事件
	EventProgress = "update-progress" // 热更新进度事件
)

// Updater 热更新接口
type Updater interface {
	// Update 执行更新
	Update(newVersion string) error
	// Close 清理资源
	Close()
	// GetCurrentExe 获取当前执行文件路径
	GetCurrentExe() string
	// GetConfig 获取配置
	GetConfig() Config
	// Restart 重启应用
	Restart() error
}

// New 创建平台特定的更新器
func New(config Config, ctx context.Context) Updater {
	// 由于使用了构建标签，编译器会自动选择正确的实现
	return newPlatformUpdater(config, ctx)
}

type fastUpdater struct {
	config  Config
	ctx     context.Context
	updater Updater
}

// NewFastUpdate 快速更新
func NewFastUpdate(config Config, ctx context.Context) *fastUpdater {
	updater := New(config, ctx)
	return &fastUpdater{
		config:  config,
		ctx:     ctx,
		updater: updater,
	}
}

// Update 方法中添加下载阶段
func (f *fastUpdater) Update(newAppPath string, WindowHide func(ctx context.Context)) error {
	defer f.updater.Close()

	// 如果提供了下载实现，执行下载
	if f.config.DownloadImpl != nil {
		downloader := NewDownloader(f.ctx, f.config, f.config.DownloadImpl)
		if err := downloader.Execute(); err != nil {
			return fmt.Errorf("下载失败: %v", err)
		}
	}

	// 检查新版本是否存在
	if _, err := os.Stat(newAppPath); err != nil {
		f.config.Logger.Logf("新版本不存在: %v", err)
		return fmt.Errorf("新版本不存在: %v", err)
	}

	// 执行更新
	f.config.Logger.Log("开始执行更新操作...")
	if err := f.updater.Update(newAppPath); err != nil {
		// 更新失败
		if f.config.EventEmitter != nil {
			f.config.EventEmitter.EmitProgress(UpdateProgress{
				Phase:      PhaseInstall,
				Percentage: 0,
				Message:    "更新失败",
				Detail:     err.Error(),
			})
		}
		f.config.Logger.Logf("更新操作失败: %v", err)
		return err
	}

	// 更新完成
	if f.config.EventEmitter != nil {
		f.config.EventEmitter.EmitProgress(UpdateProgress{
			Phase:      PhaseComplete,
			Percentage: 100,
			Message:    PhaseMessages[PhaseComplete],
			Detail:     "更新完成，准备重启...",
		})
	}

	// 更新成功，准备重启
	f.config.Logger.Log("更新成功，准备重启...")
	// 延迟一下让用户看到提示
	time.Sleep(1500 * time.Millisecond)

	// 如果提供了隐藏窗口的函数，执行隐藏
	if WindowHide != nil {
		f.config.Logger.Log("准备隐藏窗口...")
		WindowHide(f.ctx)
		f.config.Logger.Log("窗口已隐藏")
	}

	// 延迟一下让用户看到提示
	time.Sleep(500 * time.Millisecond)

	// 使用更新器的重启方法
	if err := f.updater.Restart(); err != nil {
		f.config.Logger.Logf("重启失败: %v", err)
		return err
	}

	f.config.Logger.Log("重启命令已执行，准备退出当前程序...")
	time.Sleep(1 * time.Second)

	f.config.Logger.Log("正在退出当前程序...")
	return nil
}
