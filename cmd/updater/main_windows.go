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
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

type UpdateInfo struct {
	AppPath    string `json:"app_path"`
	NewVersion string `json:"new_version"`
	BackupPath string `json:"backup_path"`
	BackupFile string `json:"backup_file"`
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
)

var (
	enumResourceNames = syscall.NewLazyDLL("kernel32.dll").NewProc("EnumResourceNamesW")
)

func EnumResourceNames(hModule win.HMODULE, lpszType uintptr, lpEnumFunc uintptr, lParam uintptr) bool {
	ret, _, _ := enumResourceNames.Call(
		uintptr(hModule),
		lpszType,
		lpEnumFunc,
		lParam,
	)
	return ret != 0
}

func main() {
	// 设置日志文件
	logFile, err := os.OpenFile("updater.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
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
		log.Printf("找到图标资源: %d", uint16(name))
		return 1 // 继续枚举
	}

	enumProc := syscall.NewCallback(callback)
	EnumResourceNames(
		win.HMODULE(hInstance),
		RT_GROUP_ICON,
		enumProc,
		0,
	)
	log.Printf("枚举到的图标资源: %v", icons)

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
		log.Printf("DPI 缩放比例: %.2f", dpiX)

		// 3. 获取主显示器工作区
		var rect win.RECT
		if win.SystemParametersInfo(SPI_GETWORKAREA, 0, unsafe.Pointer(&rect), 0) {
			// 计算窗口位置，使其居中显示（考虑 DPI 缩放）
			x := rect.Left + (rect.Right-rect.Left-int32(float64(WINDOW_WIDTH)*dpiX))/2
			y := rect.Top + (rect.Bottom-rect.Top-int32(float64(WINDOW_HEIGHT)*dpiX))/2

			log.Printf("计算的窗口位置: x=%d, y=%d", x, y)

			// 4. 设置窗口位置和大小
			if !win.SetWindowPos(updater.Handle(), 0,
				x, y, int32(WINDOW_WIDTH), int32(WINDOW_HEIGHT),
				win.SWP_NOZORDER|win.SWP_NOACTIVATE|win.SWP_FRAMECHANGED) {
				log.Printf("SetWindowPos失败，错误码: %d", win.GetLastError())
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
		if err := performUpdate(info, &updater); err != nil {
			log.Printf("更新失败: %v", err)
			updater.SetStatus("更新失败: " + err.Error())
			time.Sleep(3 * time.Second)
			updater.Close()
			os.Exit(1)
		}
		log.Println("更新完成")
		updater.SetStatus("更新完成！")
		time.Sleep(1 * time.Second)

		// 启动新版本
		log.Printf("准备启动新版本: %s", info.AppPath)
		cmd := exec.Command("cmd", "/c", "start", "", info.AppPath)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: CREATE_NO_WINDOW,
		}

		if err := cmd.Start(); err != nil {
			log.Printf("启动新版本失败: %v", err)
			updater.SetStatus("启动新版本失败！")
			time.Sleep(2 * time.Second)
			updater.Close()
			os.Exit(1)
		}

		// 等待一段时间确保程序启动
		time.Sleep(2 * time.Second)

		// 验证程序是否成功启动
		if isProcessRunning(filepath.Base(info.AppPath)) {
			log.Println("新版本已成功启动")
			updater.Close()
		} else {
			log.Println("新版本启动失败")
			updater.SetStatus("新版本启动失败")
			time.Sleep(3 * time.Second)
			updater.Close()
			os.Exit(1)
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
	log.Printf("��待进程退出: %s", processName)

	if err := waitProcessExit(processName); err != nil {
		return err
	}

	updater.SetProgress(20)

	// 确保备份文件
	backupFile := info.BackupFile
	if backupFile == "" {
		// 如果没有提供备份文件路径，创建新的备份
		timestamp := time.Now().Format("20060102_150405")
		backupFile = filepath.Join(info.BackupPath, fmt.Sprintf("backup_%s.exe", timestamp))
	}

	// 检查备份文件
	if _, err := os.Stat(backupFile); err != nil {
		// 如果备文件不存在，创建备份
		updater.SetStatus("创建备份...")
		if err := copyFile(info.AppPath, backupFile); err != nil {
			return fmt.Errorf("创建备份失败: %v", err)
		}
		log.Printf("已创建备份: %s", backupFile)
	}
	updater.SetProgress(40)

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
		// 恢复备
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

func isProcessRunning(processName string) bool {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", processName), "/NH", "/FO", "CSV")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: CREATE_NO_WINDOW,
	}
	output, err := cmd.Output()
	if err != nil {
		log.Printf("检查进程状态失败: %v", err)
		return false
	}

	// 检查输出中是否包含进程名（完整匹配）
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, fmt.Sprintf(`"%s"`, processName)) {
			log.Printf("找到进程: %s", line)
			return true
		}
	}
	return false
}

func killProcess(processName string) error {
	cmd := exec.Command("taskkill", "/F", "/IM", processName)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: CREATE_NO_WINDOW,
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("终止进程失败: %v (输出: %s)", err, string(output))
	}
	return nil
}

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

func copyFile(src, dst string) error {
	cmd := exec.Command("cmd", "/c", "copy", "/Y", src, dst)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: CREATE_NO_WINDOW,
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("复制文件失败: %v (输出: %s)", err, string(output))
	}
	return nil
}
