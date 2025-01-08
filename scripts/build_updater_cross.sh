#!/bin/bash

# 切换到项目根目录
cd "$(dirname "$0")/.."

echo "开始交叉编译更新助手..."

# 检查必要文件
if [ ! -f "cmd/updater/updater.manifest" ]; then
    echo "错误: manifest 文件不存在"
    exit 1
fi

if [ ! -f "icon.ico" ]; then
    echo "错误: icon.ico 文件不存在"
    exit 1
fi

# 创建updater目录（如果不存在）
mkdir -p cmd/updater

# 复制图标文件
echo "复制图标文件..."
cp icon.ico cmd/updater/updater.ico

# 检查是否安装了 rsrc
if ! command -v rsrc &> /dev/null; then
    echo "安装 rsrc 工具..."
    GOBIN=$(go env GOPATH)/bin go install github.com/akavel/rsrc@latest
fi

# 生成资源文件
echo "生成资源文件..."
cd cmd/updater
rsrc -manifest updater.manifest -ico updater.ico -o rsrc.syso
if [ $? -ne 0 ]; then
    echo "生成资源文件失败"
    cd ../..
    exit 1
fi
cd ../..

# 设置交叉编译环境
export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=1
export CC=x86_64-w64-mingw32-gcc
export CXX=x86_64-w64-mingw32-g++

# 检查是否安装了 MinGW
if ! command -v $CC &> /dev/null; then
    echo "错误: 未安装 MinGW-w64，请先安装:"
    echo "brew install mingw-w64"
    exit 1
fi

# 编译更新助手
echo "交叉编译更新助手..."
go build -tags walk_use_cgo -ldflags="-H windowsgui" -o updater.exe ./cmd/updater

# 创建资源目录
echo "创建资源目录..."
mkdir -p build/bin/resources

# 复制更新助手和脚本到资源目录
echo "复制更新助手和脚本..."
cp updater.exe build/bin/resources/
cp update.lua build/bin/resources/

# 清理临时文件
echo "清理临时文件..."
rm -f updater.exe
rm -f cmd/updater/rsrc.syso

echo "交叉编译完成！" 