@echo off
chcp 65001
echo 开始构建更新助手...

:: 设置变量
set UPDATER_PATH=resources
set GOPATH_BIN=

:: 获取 GOBIN 路径
for /f "tokens=*" %%i in ('go env GOPATH') do set GOPATH_BIN=%%i\bin

:: 检查必要工具
if not exist "%GOPATH_BIN%\rsrc.exe" (
    echo 安装 rsrc 工具...
    go install github.com/akavel/rsrc@latest
)

:: 创建临时目录
if not exist "temp" mkdir temp
cd temp

:: 复制必要文件
echo 复制资源文件...
copy /Y "%GOPATH_BIN%\hotupdater\cmd\updater\updater.manifest" .\ >nul
copy /Y "%GOPATH_BIN%\hotupdater\cmd\updater\updater.rc" .\ >nul

:: 检查图标
if exist "..\updater.ico" (
    copy /Y "..\updater.ico" .\ >nul
) else (
    copy /Y "%GOPATH_BIN%\hotupdater\cmd\updater\updater.ico" .\ >nul
)

:: 生成资源文件
echo 生成资源文件...
"%GOPATH_BIN%\rsrc.exe" -manifest updater.manifest -ico updater.ico -o rsrc.syso
if errorlevel 1 (
    echo 生成资源文件失败
    cd ..
    rmdir /S /Q temp
    exit /b 1
)

:: 创建main.go
echo 创建更新助手入口文件...
echo package main > main.go
echo. >> main.go
echo import "github.com/562589540/hotupdater/cmd/updater" >> main.go
echo. >> main.go
echo func main() { >> main.go
echo     updater.Main() >> main.go
echo } >> main.go

:: 编译更新助手
echo 构建更新助手...
go build -tags walk_use_cgo -ldflags="-H windowsgui" -o ..\%UPDATER_PATH%\updater.exe

:: 清理临时文件
cd ..
rmdir /S /Q temp

:: 创建资源目录（如果不存在）
if not exist "%UPDATER_PATH%" mkdir "%UPDATER_PATH%"

echo 构建完成！
echo 更新助手已生成到 %UPDATER_PATH%\updater.exe 