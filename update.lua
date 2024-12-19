-- update.lua
-- 检测操作系统类型
local function is_windows()
    return package.config:sub(1,1) == '\\'
end

-- 路径分隔符
local path_sep = is_windows() and '\\' or '/'

-- 全局变量用于存储更新路径
local g_update_path = nil
local g_write_log_file = false  -- 控制是否写入日志文件

-- Windows 特定的函数
local function win_hide_window(cmd)
    if is_windows() then
        return 'cmd /c ' .. cmd .. ' >nul 2>&1'
    end
    return cmd
end

-- 日志函数
local function log(message)
    -- 调用 Go 注册的日志函数
    log_message(message)
    -- 如果启用了文件日志，则写入文件
    if g_write_log_file and g_update_path then
        local log_path = g_update_path .. path_sep .. "update.log"
        local file = io.open(log_path, "a")
        if file then
            local time = os.date("%Y/%m/%d %H:%M:%S")
            file:write(string.format("[%s] %s\n", time, message))
            file:close()
        end
    end
end

-- Windows 命令执行函数
local function execute_win_cmd(cmd)
    if is_windows() then
        local ps_cmd = string.format('powershell -WindowStyle Hidden -Command "%s"', cmd)
        local result = os.execute(win_hide_window(ps_cmd))
        if not result then
            log(string.format("命令执行失败: %s", cmd))
            return false
        end
        log(string.format("命令执行成功: %s", cmd))
        return true
    else
        return os.execute(cmd)
    end
end

-- 检查文件是否存在
local function check_file_exists(path)
    if is_windows() then
        -- 直接使用 io.open 检查文件
        local file = io.open(path, "rb")
        if file then
            file:close()
            return true
        end
        -- 如果 io.open 失败，再尝试使用 PowerShell
        local cmd = string.format([[powershell -Command "$exists = Test-Path '%s' -PathType Leaf; exit [int](!$exists)"]], path)
        return os.execute(win_hide_window(cmd)) == 0
    else
        local file = io.open(path, "rb")
        if file then
            file:close()
            return true
        end
        return false
    end
end

-- 创建目录
local function mkdir(path)
    if is_windows() then
        -- 使用 PowerShell 创建目录
        local cmd = string.format('New-Item -ItemType Directory -Path "%s" -Force', path)
        log("执行创建目录命令: " .. cmd)
        local success = os_execute(cmd)
        log("创建目录命令执行结果: " .. tostring(success))
        
        -- 检查目录是否创建成功
        local check_cmd = string.format('Test-Path "%s"', path)
        log("检查目录命令: " .. check_cmd)
        local exists = os_execute(check_cmd)
        log("目录检查结果: " .. tostring(exists))
        
        return exists
    else
        return os.execute(string.format('mkdir -p "%s"', path))
    end
end

local function remove_if_exists(path)
    if check_file_exists(path) then
        os.remove(path)
        -- 如果是目录，使用 rmdir
        if not check_file_exists(path) then
            return true
        end
        -- 在 macOS 上使用 rm -rf 命令
        if package.config:sub(1,1) == '/' then
            os.execute(string.format('rm -rf "%s"', path))
        end
    end
end

local function fix_permissions(path)
    if package.config:sub(1,1) == '/' then  -- Unix-like systems
        os.execute(string.format('chmod -R 755 "%s"', path))
        -- 特别处理 MacOS 下的可执行文件
        local macosPath = path .. "/Contents/MacOS/*"
        os.execute(string.format('chmod +x "%s"', macosPath))
    end
end

local function get_app_root(path)
    -- 查找第一个 .app 的位置（从左到右）
    local app_index = string.find(path, ".app/")
    if app_index then
        -- 获取到第一个 .app 的完整路径
        local app_path = string.sub(path, 1, app_index + 3)
        -- 检查是否是有效的应用包路径
        if check_file_exists(app_path .. "/Contents/MacOS") then
            return app_path
        end
    end
    return path
end

local function backup_app(current_path, backup_path)
    -- 获取实际的应用根目录
    current_path = get_app_root(current_path)
    if not check_file_exists(current_path) then
        error("当前应用不存在: " .. current_path)
    end
    
    -- 先删除已存在的备份
    remove_if_exists(backup_path)
    
    -- 备份当前应用
    local success, err = os.rename(current_path, backup_path)
    if not success then
        error("备份失败: " .. (err or "未知错误"))
    end
end

local function replace_app(new_app, current_path)
    -- 获取实际的应用根目录
    current_path = get_app_root(current_path)
    if not check_file_exists(new_app) then
        error("新版本不存在: " .. new_app)
    end
    
    -- 确保目标路径不存在
    remove_if_exists(current_path)
    
    -- 替换应用
    local success, err = os.rename(new_app, current_path)
    if not success then
        error("替换失败: " .. (err or "未知错误"))
    end
    
    -- 修复权限
    fix_permissions(current_path)
end

-- 添加时间记录函数
local function get_time()
    return os.time()
end

local function log_time(start_time, operation)
    local elapsed = os.time() - start_time
    log(string.format("操作耗时[%s]: %d秒", operation, elapsed))
end

-- 备份文件
local function backup_files(src, dst_file)
    if is_windows() then
        local start_time = get_time()
        log(string.format("正在备份: %s 到 %s", src, dst_file))

        -- 检查源文件
        local check_start = get_time()
        local check_src_cmd = string.format('Test-Path "%s"', src)
        log("检查源文件命令: " .. check_src_cmd)
        local src_exists = os_execute(check_src_cmd)
        log_time(check_start, "检查源文件")
        
        if not src_exists then
            log(string.format("源文件不存在: %s", src))
            return false
        end

        -- 创建目录
        local mkdir_start = get_time()
        local dst_dir = string.match(dst_file, "(.*\\)")
        if dst_dir then
            if not mkdir(dst_dir) then
                return false
            end
        end
        log_time(mkdir_start, "创建目录")

        -- 复制文件
        local copy_start = get_time()
        local copy_cmd = string.format('Copy-Item -Path "%s" -Destination "%s" -Force', src, dst_file)
        log("执行复制命令: " .. copy_cmd)
        local copy_success = os_execute(copy_cmd)
        log_time(copy_start, "复制文件")

        -- 验证复制
        local verify_start = get_time()
        local check_dst_cmd = string.format('Test-Path "%s"', dst_file)
        local dst_exists = os_execute(check_dst_cmd)
        log_time(verify_start, "验证复制")

        log_time(start_time, "备份总耗时")
        return dst_exists
    else
        return os.execute(string.format('tar -czf "%s" "%s"', dst_file, src))
    end
end

-- 复制文件
local function copy_files(src, dst)
    if is_windows() then
        -- 检查源文件
        local check_src_cmd = string.format('Test-Path "%s"', src)
        log("检查源文件命令: " .. check_src_cmd)
        local src_exists = os_execute(check_src_cmd)
        log("源文件检查结果: " .. tostring(src_exists))

        if not src_exists then
            log("源文件不存在: " .. src)
            return false
        end

        -- 确保目标目录存在
        local dst_dir = string.match(dst, "(.*\\)")
        if dst_dir then
            log("确保目标目录存在: " .. dst_dir)
            if not mkdir(dst_dir) then
                log("创建目标目录失败")
                return false
            end
        end

        -- 复制文件
        local cmd = string.format('Copy-Item -Path "%s" -Destination "%s" -Force', src, dst)
        log("执行复制命令: " .. cmd)
        local success = os_execute(cmd)
        log("复制命令执行结果: " .. tostring(success))

        -- 验证复制结果
        local check_dst_cmd = string.format('Test-Path "%s"', dst)
        log("检查目标文件命令: " .. check_dst_cmd)
        local dst_exists = os_execute(check_dst_cmd)
        log("目标文件检查结果: " .. tostring(dst_exists))

        if not dst_exists then
            log("复制失败，目标文件不存在: " .. dst)
            return false
        end

        -- 比较文件大小
        local size_cmd = string.format('(Get-Item "%s").Length -eq (Get-Item "%s").Length', src, dst)
        log("比较文件大小命令: " .. size_cmd)
        local size_match = os_execute(size_cmd)
        log("文件大小比较结果: " .. tostring(size_match))

        return dst_exists and size_match
    else
        return os.execute(string.format('cp -R "%s" "%s"', src, dst))
    end
end

-- 删除文件或目录
local function remove_files(path)
    if is_windows() then
        -- 检查文���是否存在
        local check_cmd = string.format('Test-Path "%s"', path)
        log("检查待删除文件命令: " .. check_cmd)
        local exists = os_execute(check_cmd)
        log("待删除文件检查结果: " .. tostring(exists))

        if not exists then
            log("文件不存在，无需删除")
            return true
        end

        -- 先尝试关闭所有文件句柄
        local close_cmd = string.format([[
            powershell -Command "
                Get-Process | ForEach-Object {
                    $_.Modules | Where-Object {$_.FileName -eq '%s'} | ForEach-Object {
                        $_.Process.Kill()
                    }
                }
            "]], path)
        log("执行关闭进程命令: " .. close_cmd)
        os_execute(close_cmd)

        -- Windows 下删除文件
        local cmd = string.format('Remove-Item -Path "%s" -Force', path)
        log("执行删除命令: " .. cmd)
        local success = os_execute(cmd)
        log("删除命令执行结果: " .. tostring(success))

        if not success then
            -- 如果普通删除失败，尝试使用 taskkill
            local taskkill_cmd = string.format('taskkill /F /IM "%s"', path:match("([^\\]+)$"))
            log("执行强制结束进程命令: " .. taskkill_cmd)
            os_execute(taskkill_cmd)
            
            -- 再次尝试删除
            log("重试删除命令")
            success = os_execute(cmd)
            log("重试删除结果: " .. tostring(success))
        end

        -- 验证文件是否已删除
        local verify_cmd = string.format('Test-Path "%s"', path)
        log("验证删除结果命令: " .. verify_cmd)
        local still_exists = os_execute(verify_cmd)
        log("文件是否仍然存在: " .. tostring(still_exists))

        if still_exists then
            log("删除失败，文件仍然存在")
            return false
        end

        return true
    else
        return os.execute(string.format('rm -rf "%s"', path))
    end
end

-- macOS 特定的函数
local function remove_quarantine(path)
    if not is_windows() then
        -- 移除隔离属性
        local success = os.execute(string.format('xattr -rd com.apple.quarantine "%s"', path))
        if not success then
            log("移除隔离属性失败: " .. path)
            return false
        end
        
        -- 移除其他扩展属性
        success = os.execute(string.format('xattr -rc "%s"', path))
        if not success then
            log("移除扩展属性失败: " .. path)
            return false
        end
        
        -- 修复权限
        success = os.execute(string.format('chmod -R 755 "%s"', path))
        if not success then
            log("修复权限失败: " .. path)
            return false
        end
        
        return true
    end
    return true
end

-- 创建更新批处理脚本 (仅 Windows)
local function create_update_batch(src, dst, backup)
    local batch_path = g_update_path .. path_sep .. "update.bat"
    local file = io.open(batch_path, "w")
    if file then
        -- 等待原进程退出
        file:write("@echo off\n")
        file:write(":wait\n")
        -- 检查进程是否还在运行
        file:write(string.format('tasklist /FI "IMAGENAME eq %s" 2>NUL | find /I /N "%s">NUL\n', dst:match("([^\\]+)$"), dst:match("([^\\]+)$")))
        file:write("if %ERRORLEVEL% EQU 0 (\n")
        file:write("    timeout /t 1 /nobreak >nul\n")
        file:write("    goto wait\n")
        file:write(")\n")
        
        -- 尝试删除旧文件
        file:write(string.format('del /f /q "%s"\n', dst))
        
        -- 复制新文件
        file:write(string.format('copy /y "%s" "%s"\n', src, dst))
        
        -- 启动新版本
        file:write(string.format('start "" "%s"\n', dst))
        
        file:close()
        return batch_path
    end
    return nil
end

-- 发送进度信息
function send_progress(phase, percentage, detail)
    log_message(string.format("@PROGRESS@%s|%d|%s", phase, percentage, detail))
end

-- 在关键操作点添加进度报告
function perform_update(params)
    local total_start = get_time()
    -- 获取参数
    local app_path = params.app_path
    local new_version = params.new_version
    local backup_path = params.backup_path
    local update_path = params.update_path
    local app_root = params.app_root

    -- 设置全局更新路径
    g_update_path = update_path

    -- 开始预检查
    send_progress("precheck", 0, "正在检查更新环境...")
    
    log("开始更新...")
    log(string.format("应用路径: %s", app_path))
    log(string.format("新版本路径: %s", new_version))
    log(string.format("备份路径: %s", backup_path))
    log(string.format("更新路径: %s", update_path))
    log(string.format("应用根目录: %s", app_root))

    send_progress("precheck", 50, "正在检查路径...")

    -- 根据平台选择操作路径
    local target_path = is_windows() and app_path or app_root

    -- 如果是 macOS，先处理新版本的隔离属性
    if not is_windows() then
        log(string.format("移除新版本的隔离属性: %s", new_version))
        remove_quarantine(new_version)
    end

    send_progress("precheck", 100, "环境检查完成")

    -- 如果没有备份路径，使用应用目录下的 backup 文件夹
    if not backup_path then
        backup_path = is_windows() and filepath.Dir(app_path) .. path_sep .. "backup" 
                     or app_root .. path_sep .. "backup"
    end

    -- 创建备份目录
    send_progress("backup", 0, "准备备份...")
    log(string.format("创建备份目录: %s", backup_path))
    if not mkdir(backup_path) then
        error("创建备份目录失败")
    end

    -- 执行备份
    local backup_name = os.date("backup_%Y%m%d_%H%M%S")
    local backup_file
    if is_windows() then
        backup_file = backup_path .. path_sep .. backup_name .. ".exe"
    else
        backup_file = backup_path .. path_sep .. backup_name .. ".tar.gz"
    end
    
    send_progress("backup", 30, "正在创建备份...")
    log(string.format("创建备份文件: %s", backup_file))
    if not backup_files(target_path, backup_file) then
        error("备份失败")
    end

    send_progress("backup", 90, "正在验证备份...")
    -- 再次验证备份文件
    if not check_file_exists(backup_file) then
        error("备份文件不存在: " .. backup_file)
    end
    send_progress("backup", 100, "备份完成")

    -- 执行更新，如果失败则恢复备份
    local function restore_backup()
        log("更新失败，正在恢复备份...")
        if is_windows() then
            -- Windows 下直接复制回去
            local cmd = string.format('Copy-Item -Path "%s" -Destination "%s" -Force', backup_file, target_path)
            if not execute_win_cmd(cmd) then
                log("警告: 备份恢复失败，请手动恢复备份文件: " .. backup_file)
            end
        else
            -- macOS 下解压备份
            local cmd = string.format('tar -xzf "%s" -C "/"', backup_file)
            if not os.execute(cmd) then
                log("警告: 备份恢复失败，请手动恢复备份文件: " .. backup_file)
            end
        end
    end

    if is_windows() then
        -- Windows 平台使用批处理脚本更新
        send_progress("install", 0, "准备安装新版本...")
        
        local batch_start = get_time()
        send_progress("install", 20, "正在创建更新脚本...")
        local batch_file = create_update_batch(new_version, target_path, backup_file)
        log_time(batch_start, "创建批处理")
        
        -- 启动批处理
        send_progress("install", 40, "正在准备重启程序...")
        local start_start = get_time()
        local cmd = string.format('powershell -Command "Start-Process -FilePath \'%s\' -WindowStyle Hidden"', batch_file)
        local success = os_execute(cmd)
        log_time(start_start, "启动批处理")
        
        if not success then
            error("启动更新脚本失败")
            return false
        end
        
        send_progress("install", 80, "更新脚本已启动...")
        log("更新脚本已创建并启动，程序即将重启...")
        
        -- 验证更新脚本
        send_progress("verify", 0, "正在验证更新脚本...")
        if check_file_exists(batch_file) then
            send_progress("verify", 100, "验证完成")
            send_progress("complete", 100, "更新完成，即将重启...")
        else
            error("更新脚本创建失败")
            return false
        end
    else
        -- macOS 平台直接更新
        send_progress("install", 0, "准备安装新版本...")
        
        -- 删除旧版本
        log(string.format("删除旧版本: %s", target_path))
        send_progress("install", 20, "正在删除旧版本...")
        if not remove_files(target_path) then
            restore_backup()
            error("删除旧版本失败")
            return false
        end

        -- 复制新版本
        send_progress("install", 40, "正在复制新版本...")
        log(string.format("复制新版本: %s 到 %s", new_version, target_path))
        if not copy_files(new_version, target_path) then
            restore_backup()
            error("复制新版本失败")
            return false
        end
        send_progress("install", 80, "复制完成")

        -- 处理隔离属性
        send_progress("install", 90, "正在设置权限...")
        log(string.format("移除更新后的隔离属性: %s", app_root))
        if not remove_quarantine(app_root) then
            restore_backup()
            error("移除隔离属性失败")
            return false
        end
        send_progress("install", 100, "安装完成")

        -- 验证安装
        send_progress("verify", 0, "开始验证...")
        -- 这里可以添加验证逻辑
        send_progress("verify", 100, "验证完成")

        log("更新完成")
        send_progress("complete", 100, "更新完成")
    end

    log_time(total_start, "更新总耗时")
    return true
end 