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
- 提供图形化恢复助手
  - 支持备份版本管理和恢复
  - 自动处理管理员权限
  - 支持中文界面
  - 提供详细的恢复日志

wails案例: https://github.com/562589540/hotupdaterdemo

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

## 平台特定说明

### macOS 更新助手

macOS 平台需要使用专门的更新助手程序来处理 .app 包的更新。

#### 目录结构
```
YourApp.app/
└── Contents/
    └── Resources/
        ├── updater     # macOS更新助手程序
        └── update.lua  # 更新脚本
```

#### 构建脚本示例
```bash
#!/bin/bash
# 设置变量
APP_NAME="your_app_name"
BUILD_DIR="build/bin"
RESOURCES_DIR="$BUILD_DIR/$APP_NAME.app/Contents/Resources"
APP_DEST="/Applications/$APP_NAME.app"

# 构建更新助手
go build -o updater cmd/updater/main.go

# 创建资源目录
mkdir -p "$RESOURCES_DIR"

# 复制更新助手和脚本
cp updater "$RESOURCES_DIR/"
cp update.lua "$RESOURCES_DIR/"

# 设置权限
chmod +x "$RESOURCES_DIR/updater"
chmod 644 "$RESOURCES_DIR/update.lua"
```

### 路径说明

#### macOS
- 更新助手位置: `/Applications/YourApp.app/Contents/Resources/updater`
- 脚本相对路径: 相对于更新助手所在目录
- 示例: `ScriptPath: "./update.lua"` 指向 `/Applications/YourApp.app/Contents/Resources/update.lua`

#### Windows
- 脚本相对路径: 相对于主程序执行文件
- 示例: `ScriptPath: "./update.lua"` 指向与可执行文件同级目录
```
C:/path/to/
├── your.exe
└── update.lua
```

#### 目录结构示例

##### Windows
```
C:/path/to/
├── your.exe          # 主程序
├── update.lua        # 更新脚本
└── resources/        # 资源目录
    └── ...
```

##### macOS
```
/Applications/
└── YourApp.app/
    └── Contents/
        ├── MacOS/
        │   └── YourApp    # 主程序
        └── Resources/
            ├── updater    # 更新助手
            ├── update.lua # 更新脚本
            └── ...
```

### 最佳实践
```go
config := hotupdater.Config{
    // ... 其他配置 ...
    ScriptPath: "/absolute/path/to/update.lua", // 建议使用绝对路径
}
```

> **注意**: 建议使用绝对路径来避免不同平台的路径解析问题。如果必须使用相对路径，请注意 macOS 和 Windows 的基准路径差异。

### Lua 配置说明

#### Windows 更新助手配置
在 `update.lua` 中，你可以通过全局配置来控制 Windows 更新助手的行为：

```lua
-- 全局配置
local g_config = {
    windows_updater = {
        use_gui = true,  -- 是否使用GUI更新助手
        updater_path = "hotupdater" .. path_sep .. "updater.exe"  -- 更新助手的主执行文件相对路径
    }
}
```

配置项说明：
- `use_gui`: 布尔值，控制是否使用图形界面更新助手
  - `true`: 使用带进度条的图形界面（推荐）
  - `false`: 使用批处理 可能存在编码问题
- `updater_path`: 更新助手可执行文件的相对路径
  - 路径相对于主程序所在目录
  - 使用 `path_sep` 确保跨平台兼容性

推荐配置：
```lua
local g_config = {
    windows_updater = {
        use_gui = true,  -- 启用图形界面提供更好的用户体验
        updater_path = "hotupdater" .. path_sep .. "updater.exe"
    }
}
```

使用图形界面更新助手的优势：
1. 提供清晰的更新进度显示
2. 支持中文等本地化界面
3. 自动处理管理员权限申请
4. 提供更友好的错误提示

## 构建说明

### macOS 构建步骤

1. 构建主程序（如果使用 Wails）:
```bash
wails build
```

2. 构建更新助手:
```bash
go build -o updater cmd/updater/main.go
```

3. 创建应用程序包结构:
```bash
mkdir -p "build/bin/YourApp.app/Contents/Resources"
```

4. 复制必要文件:
```bash
cp updater "build/bin/YourApp.app/Contents/Resources/"
cp update.lua "build/bin/YourApp.app/Contents/Resources/"
```

5. 设置权限:
```bash
chmod +x "build/bin/YourApp.app/Contents/Resources/updater"
chmod 644 "build/bin/YourApp.app/Contents/Resources/update.lua"
```

完整的构建脚本请参考项目中的 `build.sh`。

### Windows 构建步骤

Windows 平台不需要特殊的更新助手，直接构建主程序即可：

```bash
go build
# 或者如果使用 Wails
wails build
```

确保 `update.lua` 脚本放在正确的相对路径下。

## 更新日志

### v0.1.0 (2024-12-21)
- 初始版本发布
- 基本功能实现:
  - Windows 和 macOS 平台支持
  - 自动备份和回滚机制
  - 进度通知和日志记录
  - 可自定义更新脚本
  - 支持自定义下载实现

### v0.1.1 (2024-12-21)
- Windows 更新助手改进:
  - 添加窗口图标支持
  - 优化窗口显示效果，避免闪烁
  - 改进 DPI 缩放支持
  - 增强错误处理和日志记录
  - 优化进程等待和终止逻辑

### v0.1.2 (2025-01-08)
- Windows 平台改进:
  - 修复更新助手中文编码显示问题
  - 优化恢复助手的管理员权限请求逻辑
  - 更新配置建议使用 GUI 更新助手提升用户体验
  - 改进错误提示的本地化支持
  - 增强进程权限检查机制

### 计划中的功能
- [ ] 支持多语言界面
- [ ] 添加更新包完整性校验
- [ ] 支持增量更新
- [ ] 添加更新前自动检查磁盘空间
- [ ] 支持自定义更新界面
- [ ] 添加更新任务队列管理
- [ ] 支持并行下载和校验
- [ ] 添加更新统计和报告功能

## 已知问题
1. 在某些 Windows 系统上可能需要管理员权限
2. macOS 上更新 .app 包时需要特别注意权限问题
3. 更新过程中不要手动关闭更新窗口

## 贡献指南
欢迎提交 Issue 和 Pull Request。如果你想贡献代码，请确保：
1. 遵循现有的代码风格
2. 添加必要的测试用例
3. 更新相关文档
4. 在 PR 中详细描述改动

### 恢复助手使用说明

#### 配置文件
恢复助手使用 JSON 格式的配置文件 `restore_config.json`：

```json
{
    "app_path": "/Applications/YourApp.app",     // 应用程序路径
    "backup_path": "/path/to/backup",            // 备份存储路径
    "current_path": "/path/to/current/version"   // 当前版本路径（可选）
}
```

配置项说明：
- `app_path`: 应用程序路径
  - Windows: 通常是 `.exe` 文件路径，如 `C:/Program Files/YourApp/app.exe`
  - macOS: 通常是 `.app` 包路径，如 `/Applications/YourApp.app`
- `backup_path`: 备份文件存储目录
  - 建议使用绝对路径
  - 确保该目录有足够的存储空间和写入权限
- `current_path`: 当前版本路径（可选）
  - 用于版本比较和更新检查

#### 配置文件位置
恢复助手会按以下顺序查找配置文件：
1. 当前目录下的 `restore_config.json`
2. 可执行文件所在目录的 `restore_config.json`
3. 如果都未找到，将在可执行文件所在目录创建默认配置

#### 目录结构示例

##### Windows
```
C:/Program Files/YourApp/
├── app.exe           # 主程序
├── restore.exe       # 恢复助手
├── restore_config.json
└── backup/          # 备份目录
    ├── backup_1.0.0_20240301_120000.exe
    └── backup_1.0.1_20240302_150000.exe
```

##### macOS
```
/Applications/
├── YourApp.app/     # 主程序
└── YourApp_Restore.app/  # 恢复助手
    └── Contents/
        ├── MacOS/
        │   └── restore
        └── Resources/
            └── restore_config.json

/Users/username/Library/Application Support/YourApp/
└── backup/         # 备份目录
    ├── backup_1.0.0_20240301_120000.tar.gz
    └── backup_1.0.1_20240302_150000.tar.gz
```

#### 使用建议
1. 建议定期清理过旧的备份文件
2. 恢复前确保目标应用已完全退出
3. 在 Windows 上可能需要管理员权限
4. 建议保留至少最近三个版本的备份
5. 定期验证备份文件的完整性