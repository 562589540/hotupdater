#!/bin/bash
#wails mac 打包测试案例
# 切换到项目根目录
cd "$(dirname "$0")/.."


# 设置变量
APP_NAME="your_app_name"
BUILD_DIR="build/bin"
RESOURCES_DIR="$BUILD_DIR/$APP_NAME.app/Contents/Resources"
APP_DEST="/Applications/$APP_NAME.app"

# 清理旧的构建
echo "清理旧的构建..."
rm -rf $BUILD_DIR

# 构建主程序
echo "构建主程序..."
wails build

# 构建更新助手
echo "构建更新助手..."
go build -o updater cmd/updater/main.go

# 创建资源目录（如果不存在）
echo "创建资源目录..."
mkdir -p "$RESOURCES_DIR"

# 复制更新助手和脚本到资源目录
echo "复制更新助手和脚本..."
cp updater "$RESOURCES_DIR/"
cp update.lua "$RESOURCES_DIR/"

# 设置权限
echo "设置权限..."
chmod +x "$RESOURCES_DIR/updater"
chmod 644 "$RESOURCES_DIR/update.lua"

# 如果目标目录存在旧版本，先删除
if [ -d "$APP_DEST" ]; then
    echo "删除旧版本..."
    rm -rf "$APP_DEST"
fi

# 移动到应用程序目录
echo "移动到应用程序目录..."
mv "$BUILD_DIR/$APP_NAME.app" "/Applications/"

# 清理临时文件
echo "清理临时文件..."
rm -f updater

echo "构建完成！"
echo "应用程序已安装到: $APP_DEST" 