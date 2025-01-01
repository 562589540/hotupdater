@echo off
chcp 65001
:: 切换到项目根目录
cd /d "%~dp0\.."

echo 开始构建...

:: 复制并重命名图标文件
echo 复制图标文件...
if exist "icon.ico" (
    if not exist "cmd\updater" mkdir "cmd\updater"
    copy /Y "icon.ico" "cmd\updater\updater.ico"
) else (
    echo 错误: icon.ico 文件不存在
    exit /b 1
)

:: 检查必要文件
if not exist "cmd\updater\updater.manifest" (
    echo 错误: manifest 文件不存在
    exit /b 1
)
if not exist "cmd\updater\updater.ico" (
    echo 错误: 图标文件不存在
    exit /b 1
)

:: 构建主程序
echo 构建主程序...
wails build

:: 设置 GOPATH 和 GOBIN
for /f "tokens=*" %%i in ('go env GOPATH') do set GOPATH=%%i
set GOBIN=%GOPATH%\bin

:: 检查 rsrc.exe 是否存在
if not exist "%GOBIN%\rsrc.exe" (
    echo 安装 rsrc 工具...
    go install github.com/akavel/rsrc@latest
)

:: 生成资源文件
echo 生成资源文件...
cd cmd\updater
"%GOBIN%\rsrc.exe" -manifest updater.manifest -ico updater.ico -o rsrc.syso
if errorlevel 1 (
    echo 生成资源文件失败
    cd ..\..
    exit /b 1
)

:: 编译更新助手
echo 构建更新助手...
cd ..\..
go build -tags walk_use_cgo -ldflags="-H windowsgui" ./cmd/updater

:: 创建资源目录（如果不存在）
echo 创建资源目录...
if not exist "build\bin\resources" mkdir "build\bin\resources"

:: 复制更新助手和脚本到资源目录
echo 复制更新助手和脚本...
copy /Y "updater.exe" "build\bin\resources\"
copy /Y "update.lua" "build\bin\resources\"

:: 清理临时文件
echo 清理临时文件...
del /F /Q "updater.exe"
cd cmd\updater
del /F /Q rsrc.syso
cd ..\..

echo 构建完成！ 