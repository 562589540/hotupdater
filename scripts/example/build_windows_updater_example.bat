:: windows更新助手 外部调用本库打包构建脚本案例

@echo off
chcp 65001 >nul
:: 切换到项目根目录
cd /d "%~dp0\.."

echo 开始构建更新助手...

:: 设置变量
set UPDATER_PATH=resources
set GOPATH_BIN=
set MODULE_PATH=

:: 获取 GOBIN 路径
for /f "tokens=*" %%i in ('go env GOPATH') do set GOPATH_BIN=%%i\bin

:: 检查 go.mod 是否存在
if not exist "go.mod" (
    echo 错误: 未找到 go.mod 文件
    exit /b 1
)

:: 检查并安装依赖
echo 安装依赖...
go get github.com/lxn/walk
go get github.com/lxn/win
go get github.com/562589540/hotupdater/cmd/updater@latest

:: 获取模块路径
for /f "tokens=*" %%i in ('go mod download -json github.com/562589540/hotupdater ^| findstr "Dir"') do set MODULE_PATH=%%~i
set MODULE_PATH=%MODULE_PATH:~10,-2%

if "%MODULE_PATH%"=="" (
    echo 错误: 无法获取模块路径
    exit /b 1
)

echo 模块路径: %MODULE_PATH%

:: 检查必要工具
if not exist "%GOPATH_BIN%\rsrc.exe" (
    echo 安装 rsrc 工具...
    go install github.com/akavel/rsrc@latest
)

:: 创建临时目录
if not exist "temp" mkdir temp
cd temp

:: 复制整个 updater 目录
echo 复制源代码...
xcopy /E /I /Y "%MODULE_PATH%\cmd\updater" updater >nul
if errorlevel 1 (
    echo 错误: 无法复制源代码
    cd ..
    rmdir /S /Q temp
    exit /b 1
)

:: 检查并使用本地图标 你的图标
if exist "..\updater.ico" (
    echo 使用本地图标...
    copy /Y "..\updater.ico" updater\updater.ico >nul
)

:: 生成资源文件
echo 生成资源文件...
"%GOPATH_BIN%\rsrc.exe" -manifest updater\updater.manifest -ico updater\updater.ico -o updater\rsrc.syso
if errorlevel 1 (
    echo 错误: 生成资源文件失败
    cd ..
    rmdir /S /Q temp
    exit /b 1
)

:: 编译更新助手
cd updater
echo 构建更新助手...
go build -tags walk_use_cgo -ldflags="-H windowsgui" -o ..\..\%UPDATER_PATH%\updater.exe
if errorlevel 1 (
    cd ..\..
    rmdir /S /Q temp
    exit /b 1
)

:: 返回并清理
cd ..\..
rmdir /S /Q temp

echo 构建完成！
echo 更新助手已生成到 %UPDATER_PATH%\updater.exe

:: 返回成功
exit /b 0