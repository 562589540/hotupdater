#!/bin/bash

# 获取提交信息，如果没有提供则使用默认信息
commit_message=${1:-"更新代码"}

# 添加所有更改的文件
git add .

# 提交更改
git commit -m "$commit_message"

# 推送到远程仓库
git push

# 清理 ssh-agent
ssh-agent -k > /dev/null

echo "代码已成功推送到远程仓库！" 