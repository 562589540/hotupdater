package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/flopp/go-findfont"
)

// BackupInfo 备份信息结构
type BackupInfo struct {
	Path            string    // 备份文件路径
	Version         string    // 版本号
	BackupTime      time.Time // 备份时间
	OriginalAppPath string    // 原应用路径
}

// Config 配置结构
type Config struct {
	AppPath     string `json:"app_path"`     // 应用程序路径
	BackupPath  string `json:"backup_path"`  // 备份目录路径
	CurrentPath string `json:"current_path"` // 当前程序路径
}

const (
	backupPrefix = "backup_"         // 备份文件前缀
	timeFormat   = "20060102_150405" // 时间格式
)

var (
	logFile     *os.File
	mainWindow  fyne.Window
	backupInfos []BackupInfo
	config      Config
)

func getAppSupportDir() string {
	if runtime.GOOS == "darwin" {
		// 对于 macOS，直接使用 .app 同级目录
		return getExecutableDir()
	}
	// 其他系统直接返回空
	return ""
}

func getExecutableDir() string {
	if runtime.GOOS == "darwin" {
		// 对于 macOS .app 包，尝试获取包内的实际路径
		exe, err := os.Executable()
		if err != nil {
			return "."
		}

		// 检查是否在 .app 包内
		if strings.Contains(exe, ".app/Contents/MacOS") {
			// 获取 .app 包的父目录
			appPath := exe
			for strings.Contains(appPath, ".app/Contents/MacOS") {
				appPath = filepath.Dir(appPath)
			}
			// 再往上两级：跳过 .app 和它的父目录
			return filepath.Dir(filepath.Dir(appPath))
		}
		return filepath.Dir(exe)
	}

	// 其他系统直接返回可执行文件目录
	execDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "."
	}
	return execDir
}

func init() {
	getEnv()
	// 设置命令行UTF-8编码
	SetUTF8Encoding()
	// 设置中文字体
	fontPaths := findfont.List()
	for _, path := range fontPaths {
		if strings.Contains(path, "msyh.ttf") || strings.Contains(path, "simhei.ttf") || strings.Contains(path, "simsun.ttc") || strings.Contains(path, "simkai.ttf") {
			os.Setenv("FYNE_FONT", path)
			break
		}
	}

	// 设置日志文件
	var logPath string
	if runtime.GOOS == "darwin" {
		// macOS: 将日志文件放在程序目录的 support 子目录中
		supportDir := getAppSupportDir()
		if supportDir != "" {
			logPath = filepath.Join(supportDir, "restore.log")
		} else {
			// 如果获取支持目录失败，使用临时目录
			logPath = filepath.Join(os.TempDir(), "restore.log")
		}
	} else {
		// 其他系统保持原样
		execDir := getExecutableDir()
		logPath = filepath.Join(execDir, "restore.log")
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("无法创建日志文件: %v\n", err)
		os.Exit(1)
	}

	// 设置日志输出
	if Dev {
		// 开发环境：同时输出到标准输出和文件
		writer := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(writer)
	} else {
		// 生产环境：只输出到文件
		log.SetOutput(logFile)
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	// 加载配置文件
	if err := loadConfig("restore_config.json"); err != nil {
		log.Printf("配置加载失败: %v", err)
		os.Exit(1)
	}
}

// 加载配置文件
func loadConfig(path string) error {
	var configPaths []string
	execDir := getExecutableDir()

	if runtime.GOOS == "darwin" {
		// macOS: 只搜索 .app 同级目录
		configPaths = append(configPaths, filepath.Join(execDir, "restore_config.json"))
	} else {
		// 其他系统保持原有搜索路径
		configPaths = []string{
			path,
			filepath.Join(filepath.Dir(os.Args[0]), "restore_config.json"),
			filepath.Join(".", "restore_config.json"),
		}
	}

	for _, configPath := range configPaths {
		data, err := os.ReadFile(configPath)
		if err == nil {
			if err := json.Unmarshal(data, &config); err == nil {
				log.Printf("成功从 %s 加载配置", configPath)
				return nil
			}
		}
	}

	// 如果所有路径加载失败，创建默认配置
	log.Printf("未找到配置文件，创建默认配置")

	// 使用正确的目录创建默认配置
	defaultConfig := Config{
		AppPath:    filepath.Join(execDir, "app"),
		BackupPath: filepath.Join(execDir, "backup"),
	}

	// 确保备份目录存在
	if err := os.MkdirAll(defaultConfig.BackupPath, 0755); err != nil {
		return fmt.Errorf("创建备份目录失败: %v", err)
	}

	// 保存默认配置
	config = defaultConfig
	configData, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("序列化默认配置失败: %v", err)
	}

	// 使用正确的目录保存配置文件
	defaultConfigPath := filepath.Join(execDir, "restore_config.json")
	if err := os.WriteFile(defaultConfigPath, configData, 0644); err != nil {
		log.Printf("警告: 无法保存默认配置文件: %v", err)
	} else {
		log.Printf("已创建默认配置文件: %s", defaultConfigPath)
	}

	// 转换为绝对路径
	if absPath, err := filepath.Abs(config.AppPath); err == nil {
		config.AppPath = absPath
	}
	if absPath, err := filepath.Abs(config.BackupPath); err == nil {
		config.BackupPath = absPath
	}
	if absPath, err := filepath.Abs(config.CurrentPath); err == nil {
		config.CurrentPath = absPath
	}

	log.Printf("使用默认配置: %+v", config)
	return nil
}

func main() {
	defer logFile.Close()

	log.Println("恢复助手启动...")

	// 检查权限
	if !checkAdminPrivileges() {
		log.Println("需要管理员权限，正在请求...")
		if runtime.GOOS == "darwin" {
			// macOS: 使用 osascript 请求权限并执行当前程序
			exe, err := os.Executable()
			if err != nil {
				log.Printf("获取程序路径失败: %v", err)
				os.Exit(1)
			}

			// 构建带引号的命令，以处理路径中的空格
			quotedPath := fmt.Sprintf(`"%s"`, exe)
			cmd := RunCommand("osascript", "-e", fmt.Sprintf(`do shell script %s with administrator privileges`, quotedPath))
			if err := cmd.Run(); err != nil {
				log.Printf("请求管理员权限失败: %v", err)
				os.Exit(1)
			}
			os.Exit(0) // 退出当前进程，新进程会以管理员权限启动
		} else if runtime.GOOS == "windows" {
			// Windows 部分保持不变
			if err := requestAdminPrivileges(); err != nil {
				log.Printf("请求管理员权限失败: %v", err)
				dialog.ShowError(fmt.Errorf("需要管理员权限才能运行此程序"), nil)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	// 创建应用
	myApp := app.New()
	mainWindow = myApp.NewWindow("备份恢复助手")
	mainWindow.Resize(fyne.NewSize(800, 500))

	// 创建界面
	content := createMainUI()
	mainWindow.SetContent(content)

	// 设置关闭回调
	mainWindow.SetCloseIntercept(func() {
		dialog.ShowConfirm("确认", "确定要退出吗？", func(ok bool) {
			if ok {
				mainWindow.Close()
			}
		}, mainWindow)
	})

	// 自动加载备份列表
	refreshBackupList()

	// 窗口居中显示
	mainWindow.CenterOnScreen()

	// 显示窗口
	mainWindow.ShowAndRun()
}

func createMainUI() fyne.CanvasObject {
	var selectedRow = -1

	// 创建备份列表表格
	table := widget.NewTable(
		// 行数
		func() (int, int) {
			return len(backupInfos), 3 // 3列：版本号、备份时间、文件名
		},
		// 创建单元格
		func() fyne.CanvasObject {
			return widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{})
		},
		// 更新单元格
		func(i widget.TableCellID, o fyne.CanvasObject) {
			label := o.(*widget.Label)
			if i.Row < len(backupInfos) {
				info := backupInfos[i.Row]
				switch i.Col {
				case 0:
					label.SetText(info.Version)
				case 1:
					label.SetText(info.BackupTime.Format("2006-01-02 15:04:05"))
				case 2:
					label.SetText(filepath.Base(info.Path))
				}
			}
		},
	)

	// 设置标题和宽度
	table.SetColumnWidth(0, 80)  // 版本号列宽
	table.SetColumnWidth(1, 180) // 时间列宽
	table.SetColumnWidth(2, 300) // 文件名列宽

	// 创建标题行
	headers := container.NewGridWithColumns(3,
		container.NewHBox(
			widget.NewLabelWithStyle("版本号", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
		),
		container.NewHBox(
			widget.NewLabelWithStyle("备份时间", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
		),
		container.NewHBox(
			widget.NewLabelWithStyle("文件名", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
		),
	)

	// 设置选择模式
	table.OnSelected = func(id widget.TableCellID) {
		selectedRow = id.Row
	}

	// 创建操作按钮
	refreshBtn := widget.NewButtonWithIcon("刷新", theme.ViewRefreshIcon(), func() {
		refreshBackupList()
		selectedRow = -1
		table.Refresh()
	})
	refreshBtn.Importance = widget.MediumImportance

	restoreBtn := widget.NewButtonWithIcon("恢复", theme.DocumentSaveIcon(), func() {
		if selectedRow < 0 || selectedRow >= len(backupInfos) {
			dialog.ShowError(fmt.Errorf("请先选择要恢复的备份"), mainWindow)
			return
		}
		confirmRestore(backupInfos[selectedRow])
	})
	restoreBtn.Importance = widget.HighImportance

	// 创建按钮容器
	buttons := container.NewHBox(
		layout.NewSpacer(),
		refreshBtn,
		widget.NewSeparator(),
		restoreBtn,
		layout.NewSpacer(),
	)

	// 创建主布局
	content := container.NewBorder(
		container.NewVBox(
			headers,
			widget.NewSeparator(),
		),
		container.NewVBox(
			widget.NewSeparator(),
			buttons,
		),
		nil,
		nil,
		table,
	)

	// 添加边距
	return container.NewPadded(content)
}

// 刷新备份列表
func refreshBackupList() {
	backupInfos = []BackupInfo{}

	// 读取备份目录
	files, err := os.ReadDir(config.BackupPath)
	if err != nil {
		dialog.ShowError(fmt.Errorf("读取备份目录失败: %v", err), mainWindow)
		return
	}

	// 解析备份文件
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		if !strings.HasPrefix(name, backupPrefix) {
			continue
		}

		path := filepath.Join(config.BackupPath, name)
		info := parseBackupName(name, path)
		if info != nil {
			info.OriginalAppPath = config.AppPath
			backupInfos = append(backupInfos, *info)
		}
	}

	// 按时间顺序
	sort.Slice(backupInfos, func(i, j int) bool {
		return backupInfos[i].BackupTime.After(backupInfos[j].BackupTime)
	})

	// 刷新界面
	if mainWindow != nil && mainWindow.Content() != nil {
		mainWindow.Content().Refresh()
	}
}

// 解析备份文件名
func parseBackupName(name, path string) *BackupInfo {
	// 移除扩展名
	baseName := strings.TrimSuffix(name, filepath.Ext(name))
	// 如果是 .tar.gz，需要再次移除 .tar
	if strings.HasSuffix(baseName, ".tar") {
		baseName = strings.TrimSuffix(baseName, ".tar")
	}

	// 检查前缀
	if !strings.HasPrefix(baseName, backupPrefix) {
		log.Printf("不是备份文件: %s", name)
		return nil
	}

	// 移除前缀
	nameWithoutPrefix := strings.TrimPrefix(baseName, backupPrefix)

	// 分割版本号和时间
	parts := strings.Split(nameWithoutPrefix, "_")
	if len(parts) < 3 {
		log.Printf("无效的备份文件名格式: %s", name)
		return nil
	}

	// 获取版本号（第一个分）
	version := parts[0]
	if version == "" {
		log.Printf("警告: 备份文件没有版本号: %s", name)
	}

	// 获取时间部分（最后两个部分）
	timeStr := parts[len(parts)-2] + "_" + parts[len(parts)-1]
	backupTime, err := time.ParseInLocation(timeFormat, timeStr, time.Local)
	if err != nil {
		log.Printf("解析备份时间失败: %v, 文件: %s, 时间字符串: %s", err, name, timeStr)
		return nil
	}

	log.Printf("成功解析备份文件: 本=%s, 时间=%s, 文件=%s",
		version,
		backupTime.Format("2006-01-02 15:04:05"),
		filepath.Base(path))

	return &BackupInfo{
		Path:       path,
		Version:    version,
		BackupTime: backupTime,
	}
}

// 确认恢复
func confirmRestore(backup BackupInfo) {
	// 确认对话框
	dialog.ShowConfirm(
		"确认恢复",
		fmt.Sprintf("确定要将备份 %s 恢复到应用程序？", filepath.Base(backup.Path)),
		func(ok bool) {
			if ok {
				performRestore(backup)
			}
		},
		mainWindow,
	)
}

// 执行恢复
func performRestore(backup BackupInfo) {
	log.Printf("开始恢复备份: %s -> %s", backup.Path, backup.OriginalAppPath)

	// 检查进程
	if isProcessRunning(filepath.Base(backup.OriginalAppPath)) {
		dialog.ShowError(fmt.Errorf("目标程正在运行，请先关闭"), mainWindow)
		return
	}

	// 创建进度对话框
	progressText := widget.NewLabel("正在准备恢复...")
	progressText.Alignment = fyne.TextAlignCenter

	// 创建一个大尺寸的进度条
	progressBar := widget.NewProgressBar()

	// 创建一个自定义大小的
	progressBarContainer := container.New(&customLayout{minHeight: 40}, progressBar)

	// 创建主容器
	progressContainer := container.NewVBox(
		progressText,
		container.NewPadded(
			container.NewHBox(
				layout.NewSpacer(),
				progressBarContainer,
				layout.NewSpacer(),
			),
		),
	)

	// 创建对话框
	progressDialog := dialog.NewCustom("正在恢复", "取消",
		progressContainer,
		mainWindow,
	)

	// 设置对话框大小
	progressDialog.Resize(fyne.NewSize(500, 200))
	progressDialog.Show()

	// 更新进度的函数
	updateProgress := func(value float64, text string) {
		progressBar.SetValue(value)
		progressText.SetText(text)
		progressBar.Refresh() // 强制刷新进度条
	}

	// 在新协程中执行恢复
	go func() {
		var restoreErr error
		defer func() {
			if restoreErr != nil {
				progressDialog.Hide()
				// 显示错误对话框
				dialog.ShowError(fmt.Errorf("恢复失败: %v", restoreErr), mainWindow)
			} else {
				// 进度完成后等待一会再显示结果
				updateProgress(1.0, "恢复完成！")
				time.Sleep(time.Second * 2) // 等待2秒
				progressDialog.Hide()

				// 创建结果内容
				resultContent := container.NewVBox(
					widget.NewLabelWithStyle("备份恢复成功！", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
					widget.NewSeparator(),
					container.NewHBox(
						widget.NewLabelWithStyle("版本:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
						widget.NewLabel(backup.Version),
					),
					container.NewHBox(
						widget.NewLabelWithStyle("备份时间:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
						widget.NewLabel(backup.BackupTime.Format("2006-01-02 15:04:05")),
					),
					container.NewHBox(
						widget.NewLabelWithStyle("恢复位置:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
						widget.NewLabelWithStyle(backup.OriginalAppPath, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true}),
					),
					widget.NewSeparator(),
					widget.NewLabelWithStyle("是否要查看详细日志？", fyne.TextAlignCenter, fyne.TextStyle{}),
				)

				// 创建结果对话框
				resultDialog := dialog.NewCustomConfirm(
					"恢复完成",
					"查看日志", // 左边按钮
					"取消",   // 右边按钮
					container.NewPadded(resultContent),
					func(viewLog bool) {
						if viewLog {
							// 打开日志文件
							logPath := filepath.Join(getExecutableDir(), "restore.log")
							if runtime.GOOS == "windows" {
								exec.Command("notepad", logPath).Start()
							} else {
								exec.Command("open", logPath).Start()
							}
						}
					},
					mainWindow,
				)

				// 设置对话框大小
				resultDialog.Resize(fyne.NewSize(600, 300))
				resultDialog.Show()
			}
		}()

		// 根据平台执行不同的恢复逻辑
		if runtime.GOOS == "windows" {
			// Windows: 直接替换 exe 文件
			updateProgress(0.3, "正在删除旧文件...")

			// 删除目标文件
			if err := os.Remove(backup.OriginalAppPath); err != nil && !os.IsNotExist(err) {
				restoreErr = fmt.Errorf("删除目标文件失败: %v", err)
				return
			}

			// 复制备份文件
			updateProgress(0.6, "正在复制备份文件...")
			if err := copyFile(backup.Path, backup.OriginalAppPath); err != nil {
				restoreErr = fmt.Errorf("复制文件失败: %v", err)
				return
			}

			// 验证文件
			updateProgress(0.9, "正在验证文件...")
			if _, err := os.Stat(backup.OriginalAppPath); err != nil {
				restoreErr = fmt.Errorf("验证文件失败: %v", err)
				return
			}

		} else {
			// macOS: 解压 tar.gz 到根目录
			updateProgress(0.3, "正在删除旧文件...")

			// 删除目标目录
			if err := os.RemoveAll(backup.OriginalAppPath); err != nil && !os.IsNotExist(err) {
				restoreErr = fmt.Errorf("删除目标目录失败: %v", err)
				return
			}

			// 解压备份文件
			updateProgress(0.6, "正在解压备份文件...")
			cmd := RunCommand("tar", "-xzf", backup.Path, "-C", "/")
			if output, err := cmd.CombinedOutput(); err != nil {
				restoreErr = fmt.Errorf("解压备份文件失败: %v\n%s", err, string(output))
				return
			}

			// 验证恢复
			updateProgress(0.8, "正在验证文件...")
			if _, err := os.Stat(backup.OriginalAppPath); err != nil {
				restoreErr = fmt.Errorf("验证恢复失败: %v", err)
				return
			}

			// 修复权限
			updateProgress(0.9, "正在修复权限...")
			if output, err := RunCommand("chmod", "-R", "755", backup.OriginalAppPath).CombinedOutput(); err != nil {
				restoreErr = fmt.Errorf("修复权限失败: %v\n%s", err, string(output))
				return
			}

			// 修改所有者
			updateProgress(0.95, "正在修改所有者...")
			if output, err := RunCommand("chown", "-R", os.Getenv("USER")+":staff", backup.OriginalAppPath).CombinedOutput(); err != nil {
				restoreErr = fmt.Errorf("修改所有者失败: %v\n%s", err, string(output))
				return
			}
		}

		log.Println("备份恢复成功")
	}()
}

// 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %v", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %v", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("复制文件失败: %v", err)
	}

	return nil
}

// 在文件末尾添加自定义布局
type customLayout struct {
	minHeight float32
}

func (c *customLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w := float32(400)
	h := c.minHeight
	return fyne.NewSize(w, h)
}

func (c *customLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}

var Dev bool

func getEnv() {
	// 尝试获取当前工作目录
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	// 检查是否在开发环境（假设在开发环境中有一个标识文件或目录，如 go.mod）
	if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
		Dev = true
		return
	}
	Dev = false
}
