//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

type UpdateInfo struct {
	AppPath        string `json:"app_path"`
	NewVersion     string `json:"new_version"`
	BackupPath     string `json:"backup_path"`
	BackupFile     string `json:"backup_file"`
	CurrentVersion string `json:"current_version"`
	UpdateVersion  string `json:"update_version"`
}

type UpdaterWindow struct {
	*walk.MainWindow
	progressBar *walk.ProgressBar
	statusLabel *walk.TextLabel
}

const (
	CREATE_NO_WINDOW = 0x08000000
	WINDOW_WIDTH     = 400
	WINDOW_HEIGHT    = 200
	MARGIN_SIZE      = 15
	VSPACE_SIZE      = 8
	PROGRESS_HEIGHT  = 5

	// Windows API 常量
	SPI_GETWORKAREA = 0x0030
	SM_CXSCREEN     = 0
	SM_CYSCREEN     = 1

	// 资源类型常量
	RT_ICON       uintptr = 3
	RT_GROUP_ICON uintptr = 14

	PROCESS_QUERY_INFORMATION = 0x0400
	STILL_ACTIVE              = 259
)

var (
	enumResourceNames = syscall.NewLazyDLL("kernel32.dll").NewProc("EnumResourceNamesW")
	logFilePath       string
)

type PROCESSENTRY32 struct {
	dwSize              uint32
	cntUsage            uint32
	th32ProcessID       uint32
	th32DefaultHeapID   uintptr
	th32ModuleID        uint32
	cntThreads          uint32
	th32ParentProcessID uint32
	pcPriClassBase      int32
	dwFlags             uint32
	szExeFile           [260]uint16
}

// 枚举资源名称
func EnumResourceNames(hModule win.HMODULE, lpszType uintptr, lpEnumFunc uintptr, lParam uintptr) bool {
	ret, _, _ := enumResourceNames.Call(
		uintptr(hModule),
		lpszType,
		lpEnumFunc,
		lParam,
	)
	return ret != 0
}

func init() {
	// 设置UTF-8编码
	SetUTF8Encoding()

	// 初始化日志文件路径
	execDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		execDir = "."
	}
	logFilePath = filepath.Join(execDir, "updater.log")
}

func main() {
	// 设置日志文件，每次启动时清空之前的日志
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("无法创建日志文件: ", err)
	}
	defer logFile.Close()

	// 同时输出到文件和控制台
	log.SetOutput(io.MultiWriter(logFile, os.Stdout))
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	log.Println("更新助手启动...")

	// 解析命令行参数
	updateFile := flag.String("update", "", "更新信息文件路径")
	flag.Parse()

	if *updateFile == "" {
		log.Fatal("需要提供更新信息文件路径")
	}
	log.Printf("更新信息文件: %s", *updateFile)

	// 读取更新信息
	data, err := os.ReadFile(*updateFile)
	if err != nil {
		log.Fatalf("读取更新信息失败: %v", err)
	}
	log.Printf("读取到更新信息: %s", string(data))

	var info UpdateInfo
	if err := json.Unmarshal(data, &info); err != nil {
		log.Fatalf("解析更新信息失败: %v", err)
	}
	log.Printf("解析的更新信息: %+v", info)

	// 创建并显示更新窗口
	var updater UpdaterWindow
	log.Println("准备创建更新窗口...")

	// 加载图标前，先列出所有资源
	var icons []uint16
	hInstance := win.GetModuleHandle(nil)
	callback := func(h win.HMODULE, t, name uintptr) uintptr {
		icons = append(icons, uint16(name))
		//log.Printf("找到图标资源: %d", uint16(name))
		return 1 // 继续枚举
	}

	enumProc := syscall.NewCallback(callback)
	EnumResourceNames(
		win.HMODULE(hInstance),
		RT_GROUP_ICON,
		enumProc,
		0,
	)
	//log.Printf("枚举到的图标资源: %v", icons)

	// 使用找到的资源 ID 加载图标
	var icon *walk.Icon
	if len(icons) > 0 {
		hIcon := win.LoadIcon(hInstance, win.MAKEINTRESOURCE(uintptr(icons[0])))
		if hIcon == 0 {
			log.Printf("从资源加载图标失败: %d，将使用默认图标", win.GetLastError())
		} else {
			// 获取当前 DPI
			hdc := win.GetDC(0)
			dpi := int(win.GetDeviceCaps(hdc, win.LOGPIXELSX))
			win.ReleaseDC(0, hdc)

			// 使用新的 API 创建图标
			var err error
			icon, err = walk.NewIconFromHICONForDPI(hIcon, dpi)
			if err != nil {
				log.Printf("创建图标失败: %v，将使用默认图标", err)
				icon = nil
			}
		}
	}

	// 先创建 MainWindow 结构
	mw := MainWindow{
		AssignTo:   &updater.MainWindow,
		Title:      "正在更新...",
		MinSize:    Size{Width: WINDOW_WIDTH, Height: WINDOW_HEIGHT},
		MaxSize:    Size{Width: WINDOW_WIDTH, Height: WINDOW_HEIGHT},
		Layout:     VBox{Margins: Margins{Left: MARGIN_SIZE, Top: MARGIN_SIZE, Right: MARGIN_SIZE, Bottom: MARGIN_SIZE}},
		Background: SolidColorBrush{Color: walk.RGB(250, 250, 250)},
		Font:       Font{Family: "Microsoft YaHei"},
		Visible:    false,
		Children: []Widget{
			TextLabel{
				AssignTo: &updater.statusLabel,
				Text:     "准备更新...",
				MinSize:  Size{Height: 16},
				Font:     Font{PointSize: 9},
			},
			VSpacer{Size: VSPACE_SIZE},
			ProgressBar{
				AssignTo: &updater.progressBar,
				MinValue: 0,
				MaxValue: 100,
				MinSize:  Size{Height: PROGRESS_HEIGHT},
				MaxSize:  Size{Height: PROGRESS_HEIGHT},
			},
			VSpacer{Size: VSPACE_SIZE},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					HSpacer{},
					TextLabel{
						Text:      "请勿关闭此窗口...",
						Font:      Font{PointSize: 8},
						TextColor: walk.RGB(160, 160, 160),
					},
				},
			},
		},
	}

	if icon != nil {
		mw.Icon = icon
	}

	// 然后再创建窗口
	err = mw.Create()

	if err != nil {
		log.Fatalf("创建窗口失败: %v", err)
	}

	// 设置窗口样式和位置
	updater.Synchronize(func() {
		// 1. 设置窗口样式
		style := win.GetWindowLong(updater.Handle(), win.GWL_STYLE)
		style &= ^(win.WS_MAXIMIZEBOX | win.WS_MINIMIZEBOX | win.WS_THICKFRAME)
		win.SetWindowLong(updater.Handle(), win.GWL_STYLE, style)

		// 2. 获取 DPI 缩放比例
		hdc := win.GetDC(0)
		dpiX := float64(win.GetDeviceCaps(hdc, win.LOGPIXELSX)) / 96.0
		win.ReleaseDC(0, hdc)
		//log.Printf("DPI 缩放比例: %.2f", dpiX)

		// 3. 获取主显示器工作区
		var rect win.RECT
		if win.SystemParametersInfo(SPI_GETWORKAREA, 0, unsafe.Pointer(&rect), 0) {
			// 计算窗口位置，使其居中显示（考虑 DPI 缩放）
			x := rect.Left + (rect.Right-rect.Left-int32(float64(WINDOW_WIDTH)*dpiX))/2
			y := rect.Top + (rect.Bottom-rect.Top-int32(float64(WINDOW_HEIGHT)*dpiX))/2

			//log.Printf("计算的窗口位置: x=%d, y=%d", x, y)

			// 4. 设置窗口位置和大小
			if !win.SetWindowPos(updater.Handle(), 0,
				x, y, int32(WINDOW_WIDTH), int32(WINDOW_HEIGHT),
				win.SWP_NOZORDER|win.SWP_NOACTIVATE|win.SWP_FRAMECHANGED) {
				//log.Printf("SetWindowPos失败，错误码: %d", win.GetLastError())
			}
		}

		// 5. 设置窗口置顶
		win.SetWindowPos(updater.Handle(), win.HWND_TOPMOST,
			0, 0, 0, 0,
			win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE)

		// 6. 延迟显示窗口
		time.Sleep(100 * time.Millisecond) // 给系统一点时间处理之前的设置
		updater.Show()
	})

	log.Println("更新窗口创建成功")

	// 在新协程中执行更新
	go func() {
		log.Println("开始执行新...")
		// 更新版本
		if err := performUpdate(info, &updater); err != nil {
			log.Printf("更新失败: %v", err)
			updater.SetStatus("更新失败, 详细请检查日志: " + logFilePath)
			return
		}
		log.Println("更新完成")
		updater.SetStatus("更新完成！")

		//删除新版本
		os.Remove(info.NewVersion)

		//等待1秒确保文件句柄释放
		time.Sleep(1 * time.Second)

		// 启动新版本
		log.Printf("准备启动新版本: %s", info.AppPath)
		cmd := RunCommand("cmd", "/c", "start", "", info.AppPath)
		if err := cmd.Start(); err != nil {
			log.Printf("启动新版本失败: %v", err)
			updater.SetStatus("启动新版本失败, 详细请检查日志: " + logFilePath)
			return
		}

		// 等待一段时间确保程序启动
		time.Sleep(2 * time.Second)

		// 验证程序是否成功启动
		if isProcessRunning(filepath.Base(info.AppPath)) {
			log.Println("新版本已成功启动")
			updater.Close()
		} else {
			log.Println("新版本启动失败")
			updater.SetStatus("新版本启动失败, 详细请检查日志: " + logFilePath)
			return
		}
	}()

	// 开始消息循环
	log.Println("开始运行更新窗口...")
	updater.Run()
	log.Println("更新助手退出")
}

func (w *UpdaterWindow) SetProgress(value int) {
	if w == nil || w.progressBar == nil {
		log.Printf("警告: progressBar 未初始化")
		return
	}
	w.Synchronize(func() {
		w.progressBar.SetValue(value)
	})
}

func (w *UpdaterWindow) SetStatus(text string) {
	if w == nil || w.statusLabel == nil {
		log.Printf("警告: statusLabel 未初始化")
		return
	}
	w.Synchronize(func() {
		w.statusLabel.SetText(text)
	})
}

func performUpdate(info UpdateInfo, updater *UpdaterWindow) error {
	// 等待原程序退出
	updater.SetStatus("等待程序退出...")
	processName := filepath.Base(info.AppPath)
	log.Printf("等待进程退出: %s", processName)

	if err := waitProcessExit(processName); err != nil {
		return err
	}

	updater.SetProgress(20)

	// 确保备份文件
	backupFile := info.BackupFile
	if backupFile == "" {
		// 如果没有提供备份文件路径，创建新的备份
		timestamp := time.Now().Format("20060102_150405")
		if info.CurrentVersion != "" {
			backupFile = filepath.Join(info.BackupPath, fmt.Sprintf("backup_%s_%s.exe", info.CurrentVersion, timestamp))
		} else {
			backupFile = filepath.Join(info.BackupPath, fmt.Sprintf("backup_%s.exe", timestamp))
		}
	}

	// 检查备份文件
	if _, err := os.Stat(backupFile); err != nil {
		log.Printf("传入的备份文件不存在，尝试创建: %s", backupFile)
		// 如果备文件不存在，创建备份
		updater.SetStatus("创建备份...")
		if err := copyFile(info.AppPath, backupFile); err != nil {
			log.Printf("创建备份失败: %v", err)
			return fmt.Errorf("创建备份失败: %v", err)
		}
		log.Printf("已创建备份: %s", backupFile)
	}
	updater.SetProgress(40)

	log.Printf("删除旧版本: %s", info.AppPath)
	// 删除旧版本前先尝试修改权限
	updater.SetStatus("删除旧版本...")
	if err := os.Chmod(info.AppPath, 0666); err != nil {
		log.Printf("修改文件权限失败: %v", err)
	}

	// 多次尝试删除文件
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		err := os.Remove(info.AppPath)
		if err == nil {
			break
		}
		if i == maxRetries-1 {
			return fmt.Errorf("删除旧版本失败: %v", err)
		}
		log.Printf("删除文件失败，重试 %d/%d: %v", i+1, maxRetries, err)
		time.Sleep(time.Second)
	}
	updater.SetProgress(60)

	// 复制新版本
	updater.SetStatus("安装新版本...")
	if err := copyFile(info.NewVersion, info.AppPath); err != nil {
		// 恢复备份
		log.Printf("复制新版本失败，准备回滚到备份: %s", info.BackupFile)
		updater.SetStatus("更新失败，正在恢复...")
		if restoreErr := copyFile(info.BackupFile, info.AppPath); restoreErr != nil {
			return fmt.Errorf("更新失败且无法恢复备份: %v", restoreErr)
		}
		return fmt.Errorf("复制新版本失败: %v", err)
	}
	updater.SetProgress(80)

	// 验证版本
	updater.SetStatus("验证更新...")
	if _, err := os.Stat(info.AppPath); err != nil {
		// 验证失败，恢复备份
		log.Printf("验证新版本失败: %v，准备回滚", err)
		updater.SetStatus("验证失败，正在恢复...")
		if restoreErr := copyFile(info.BackupFile, info.AppPath); restoreErr != nil {
			return fmt.Errorf("验证失败且无法恢复备份: %v (原始错误: %v)", restoreErr, err)
		}
		return fmt.Errorf("验证新版本失败: %v", err)
	}

	// 额外验证文件大小
	newFileInfo, err := os.Stat(info.AppPath)
	if err != nil || newFileInfo.Size() == 0 {
		log.Printf("新版本文件异常: %v", err)
		updater.SetStatus("验证失败，正在恢复...")
		if restoreErr := copyFile(info.BackupFile, info.AppPath); restoreErr != nil {
			return fmt.Errorf("验证失败且无法恢复备份: %v", restoreErr)
		}
		return fmt.Errorf("新版本文件无效")
	}

	updater.SetProgress(100)
	return nil
}

// 检查进程是否正在运行
func isProcessRunning(processName string) bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	createSnapshot := kernel32.NewProc("CreateToolhelp32Snapshot")
	process32First := kernel32.NewProc("Process32FirstW")
	process32Next := kernel32.NewProc("Process32NextW")
	closeHandle := kernel32.NewProc("CloseHandle")

	handle, _, _ := createSnapshot.Call(0x2, 0) // TH32CS_SNAPPROCESS = 0x2
	if handle == uintptr(syscall.InvalidHandle) {
		log.Printf("创建进程快照失败")
		return false
	}
	defer closeHandle.Call(handle)

	var entry PROCESSENTRY32
	entry.dwSize = uint32(unsafe.Sizeof(entry))

	ret, _, _ := process32First.Call(handle, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		log.Printf("获取第一个进程失败")
		return false
	}

	for {
		name := syscall.UTF16ToString(entry.szExeFile[:])
		if name == processName {
			log.Printf("找到进程: %s", name)
			return true
		}

		ret, _, _ := process32Next.Call(handle, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	log.Printf("未找到进程: %s", processName)
	return false
}

// 强制终止进程
func killProcess(processName string) error {
	cmd := RunCommand("taskkill", "/F", "/IM", processName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("终止进程失败: %v (输出: %s)", err, string(output))
	}
	return nil
}

// 等待进程退出
func waitProcessExit(processName string) error {
	log.Printf("等待进程退出: %s", processName)

	// 创建定时器，5秒后强制终止
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	// 创建ticker用于持续检测，间隔500ms
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timer.C:
			// 超过5秒，强制结束所有实例
			log.Printf("进程 %s 5秒内未退出，准备强制止所有实例", processName)
			if err := killProcess(processName); err != nil {
				return fmt.Errorf("强制终止进程失败: %v", err)
			}
			// 等待1秒确认进程已终止
			time.Sleep(time.Second)
			if isProcessRunning(processName) {
				return fmt.Errorf("强制终止后进程仍在运行")
			}
			log.Println("进程已强制终止")
			time.Sleep(time.Second) // 额外等待1秒确保文件句柄释放
			return nil

		case <-ticker.C:
			// 每500ms检查一次进程状态
			if !isProcessRunning(processName) {
				log.Println("进程已退出")
				time.Sleep(time.Second) // 等待1秒确保文件句柄释放
				return nil
			}
			log.Printf("进程 %s 仍在运行，等待中...", processName)
		}
	}
}

// 复制文件
func copyFile(src, dst string) error {
	cmd := RunCommand("cmd", "/c", "copy", "/Y", src, dst)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("复制文件失败: %v (输出: %s)", err, string(output))
	}
	return nil
}

// 设置UTF-8编码
func SetUTF8Encoding() {
	cmd := RunCommand("cmd", "/C", "chcp 65001 >nul")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// 运行命令包装隐藏窗口
func RunCommand(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	return cmd
}
