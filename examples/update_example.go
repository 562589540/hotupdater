package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/562589540/hotupdater/pkg/hotupdater"
)

// CustomEventEmitter 自定义事件发射器
type CustomEventEmitter struct{}

func (e *CustomEventEmitter) EmitLog(message string) {
	fmt.Println("日志:", message)
}

func (e *CustomEventEmitter) EmitProgress(progress hotupdater.UpdateProgress) {
	fmt.Printf("事件进度: 阶段=%s, 进度=%d%%, 消息=%s\n",
		progress.Phase,
		progress.Percentage,
		progress.Message)
}

// CustomDownloader 自定义下载器实现
type CustomDownloader struct{}

func (d *CustomDownloader) Execute(ctx context.Context, onProgress func(current, total int64, speed float64)) error {
	// 实现下载逻辑
	total := int64(1000)
	current := int64(0)

	for current < total {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			current += 100
			onProgress(current, total, 1024*1024)
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}

// 实现自定义的日志接口
type CustomLogger struct {
	logger *log.Logger
}

func (l *CustomLogger) Log(message string) {
	l.logger.Println(message)
}

func (l *CustomLogger) Logf(format string, args ...interface{}) {
	l.logger.Printf(format, args...)
}

func main() {
	ctx := context.Background()

	// 创建配置
	config := hotupdater.Config{
		UpdatePath:   "/path/to/updates",    //更新使用资源存放的路径 非安装包路径 内部存放update_info.json等数据使用的路径
		BackupPath:   "/path/to/backup",     //备份路径 备份当前包的路径
		ScriptPath:   "./update.lua",        //lua脚本放的路径  如果使用相对路径 mac是相当于启动助手的路径 win是相对于执行文件的路径
		DownloadImpl: &CustomDownloader{},   //下载器实现 你自己的下载逻辑 我们接受onProgress方法中的进度 触发进度更新
		EventEmitter: &CustomEventEmitter{}, //事件发射器实现 你自己的事件逻辑
		Logger:       &CustomLogger{},       //日志实现 你自己的日志逻辑
	}

	// 创建更新器
	updater := hotupdater.NewFastUpdate(config, ctx)
	// 执行更新
	if err := updater.Update("path/to/new/app", func(ctx context.Context) {
		fmt.Println("窗口已隐藏")
	}); err != nil {
		fmt.Printf("更新失败: %v\n", err)
		return
	}

	fmt.Println("更新成功!")
	//执行你的关闭进程逻辑
	//os.Exit(0)
}
