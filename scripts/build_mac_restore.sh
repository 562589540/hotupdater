#!/bin/bash
# 恢复助手打包脚本 - Mac 版本

# 切换到项目根目录
cd "$(dirname "$0")/.."

# 设置颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}开始构建 Mac 版本...${NC}"

# 创建输出目录
mkdir -p build/mac

# 设置编译环境
export GOOS=darwin
export GOARCH=amd64
export CGO_ENABLED=1

# 使用 fyne 打包
echo "开始打包..."
cd cmd/restore && fyne package \
    -os darwin \
    -name "restore" \
    -appID "com.helper.restore" \
    -release \
    -icon ../../icon.ico \
    .

if [ $? -eq 0 ]; then
    # 移动文件到正确的输出目录
    mv restore.app ../../build/mac/
    echo -e "${GREEN}更新助手构建成功: build/mac/restore.app${NC}"
else
    echo -e "${RED}更新助手构建失败${NC}"
    exit 1
fi

cd ../..
echo -e "${GREEN}构建完成!${NC}"
echo "输出目录: build/mac/" 