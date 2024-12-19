#!/bin/bash
#./gittag.sh v1.0.0 "第一个正式版本发布"
# 检查是否提供了标签名称
if [ -z "$1" ]; then
    echo "请提供标签名称！"
    echo "使用方法: ./gittag.sh <标签名称> [标签说明]"
    exit 1
fi

# 获取标签名称和说明
tag_name=$1
tag_message=${2:-"版本 $1"}

# 创建带注释的标签
git tag -a "$tag_name" -m "$tag_message"

# 推送标签到远程仓库
git push origin "$tag_name"

# 生成版本文件
echo "version=$tag_name" > version.txt
echo "create_time=$(date '+%Y-%m-%d %H:%M:%S')" >> version.txt
echo "commit_id=$(git rev-parse HEAD)" >> version.txt
echo "tag_message=$tag_message" >> version.txt

# 将版本文件添加到git
git add version.txt
git commit -m "更新版本文件到 $tag_name"
git push

echo "标签 $tag_name 已创建并推送到远程仓库！"
echo "版本信息已保存到 version.txt" 