# HotUpdater - Go应用热更新库

HotUpdater 是一个用 Go 语言编写的应用程序热更新库，支持 Windows 和 macOS 平台。它提供了简单的接口来实现应用程序的热更新功能。

## 功能特性

- 支持 Windows 和 macOS 平台
- 提供优雅的更新流程管理
- 支持更新进度通知
- 内置备份和回滚机制
- 可自定义更新脚本
- 提供日志记录接口
- 支持自定义下载实现

## 安装

```bash
go get github.com/562589540/hotupdater
```

## 快速开始

### 1. 实现必要的接口

#### 事件发射器
```go
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
```

#### 日志接口
```go
type CustomLogger struct {
    logger *log.Logger
}

func (l *CustomLogger) Log(message string) {
    l.logger.Println(message)
}

func (l *CustomLogger) Logf(format string, args ...interface{}) {
    l.logger.Printf(format, args...)
}
```

#### 下载实现
```go
type CustomDownloader struct{}

func (d *CustomDownloader) Execute(ctx context.Context, onProgress func(current, total int64, speed float64)) error {
    // 实现你的下载逻辑
    // 通过 onProgress 回调报告下载进度
    return nil
}
```

### 2. 配置和使用

```go
func main() {
    ctx := context.Background()

    // 创建配置
    config := hotupdater.Config{
        UpdatePath:   "/path/to/updates",    // 更新使用资源存放的路径，内部存放update_info.json等数据
        BackupPath:   "/path/to/backup",     // 备份路径，备份当前包的路径
        ScriptPath:   "./update.lua",        // lua脚本路径，mac相对于助手路径，win相对于执行文件路径
        DownloadImpl: &CustomDownloader{},   // 下载器实现，处理实际的下载逻辑
        EventEmitter: &CustomEventEmitter{}, // 事件发射器实现，处理进度通知
        Logger:       &CustomLogger{},       // 日志实现，处理日志记录
    }

    // 创建快速更新器
    updater := hotupdater.NewFastUpdate(config, ctx)

    // 执行更新
    err := updater.Update("path/to/new/app", func(ctx context.Context) {
        fmt.Println("窗口已隐藏")
    })
    if err != nil {
        fmt.Printf("更新失败: %v\n", err)
        return
    }

    fmt.Println("更新成功!")
    // 执行你的关闭进程逻辑
    // os.Exit(0)
}
```

## 更新流程

1. 下载阶段 (可选)：
   - 如果提供了 DownloadImpl，执行下载
   - 通过 EventEmitter 报告下载进度

2. 更新阶段：
   - 检查新版本
   - 备份当前版本
   - 执行更新脚本
   - 验证更新结果

3. 完成阶段：
   - 隐藏窗口（如果提供了回调）
   - 重启应用
   - 清理资源

## 进度通知

更新过程中会通过 EventEmitter 发送进度事件：

```go
type UpdateProgress struct {
    Phase      string // 当前阶段
    Percentage int    // 总体进度百分比(0-100)
    Speed      float64// 下载速度（下载阶段）
    Message    string // 用户友好的提示信息
    Detail     string // 详细信息
}
```

更新阶段包括：
- PhaseDownload: 下载新版本
- PhasePreCheck: 更新前检查
- PhaseBackup: 备份当前版本
- PhaseInstall: 安装新版本
- PhaseVerify: 验证安装
- PhaseComplete: 更新完成

## 注意事项

1. 确保更新目录具有适当的写入权限
2. 在 macOS 上更新 .app 包时需要特别注意权限问题
3. 建议在更新前进行版本检查和完整性验证
4. 更新失败时会自动回滚到备份版本
5. lua 脚本路径在不同平台下的相对路径基准不同

## 贡献

欢迎提交 Issue 和 Pull Request。

## 许可证

MIT License