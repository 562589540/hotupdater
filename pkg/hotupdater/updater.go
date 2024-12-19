package hotupdater

import (
	"context"
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
