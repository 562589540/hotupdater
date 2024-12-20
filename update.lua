-- 全局配置
local g_config = {
    windows_updater = {
        use_gui = false,  -- 是否使用GUI更新助手
        updater_path = "updater.exe"  -- 更新助手的主执行文件相对路径
    }
}

-- 全局变量用于存储更新路径
local g_update_path = nil
local g_write_log_file = false  -- 控制是否写入日志文件


-- 检测操作系统类型
local function is_windows()
    return package.config:sub(1,1) == '\\'
end

-- 路径分隔符
local path_sep = is_windows() and '\\' or '/'

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
        -- 检查文件是否存在
        if not check_file_exists(path) then
            log("文件不存在，无需删除: " .. path)
            return true
        end

        -- 先尝试关闭进程
        local taskkill_cmd = string.format('taskkill /F /IM "%s" >nul 2>&1', path:match("([^\\]+)$"))
        os_execute(taskkill_cmd)

        -- 删除文件
        local cmd = string.format('del /F /Q "%s" >nul 2>&1', path)
        local success = os_execute(cmd)

        -- 验证删除结果
        if check_file_exists(path) then
            log("删除失败，文件仍然存在: " .. path)
            return false
        end

        return true
    else
        return os.execute(string.format('rm -rf "%s"', path))
    end
end

-- 执行命令并返回输出
local function execute_command(cmd)
    log_message("执行命令: " .. cmd)
    local handle = io.popen(cmd .. " 2>&1")
    if not handle then
        log_message("命令执行失败: 无法创建进程")
        return nil
    end
    
    local result = handle:read("*a")
    local success, exit_type, exit_code = handle:close()
    
    if not success then
        log_message(string.format("命令执行失败: %s (exit_type=%s, exit_code=%s)", 
            result or "无输出", exit_type or "unknown", exit_code or "unknown"))
        return nil
    end
    
    log_message("命令执行成功，输出: " .. (result or "无"))
    return result
end

-- 添加超时执行函数
local function execute_with_timeout(cmd, timeout)
    log_message(string.format("开始执行命令(超时=%d秒): %s", timeout, cmd))
    
    local done = false
    local result = nil
    local start_time = os.time()
    
    -- 在新线程中执行命令
    local co = coroutine.create(function()
        local handle = io.popen(cmd .. " 2>&1")
        if not handle then
            log_message("命令执行失败: 无法创建进程")
            result = nil
            done = true
            return
        end
        
        result = handle:read("*a")
        local success, exit_type, exit_code = handle:close()
        
        if not success then
            log_message(string.format("命令执行失败: %s (exit_type=%s, exit_code=%s)", 
                result or "无输出", exit_type or "unknown", exit_code or "unknown"))
            result = nil
        end
        
        done = true
    end)
    coroutine.resume(co)
    
    -- 等待完成或超时
    while not done and os.time() - start_time < timeout do
        os.execute("sleep 1")
    end
    
    if not done then
        log_message(string.format("命令执行超时: %s", cmd))
        -- 尝试杀死可能卡住的进程
        os.execute("pkill -f xattr")
        os.execute("pkill -f spctl")
        return false, "timeout"
    end
    
    if result == nil then
        return false, "execution_failed"
    end
    
    log_message("命令执行成功，输出: " .. (result or "无"))
    return true, result
end

-- 检查是否存在隔离属性
local function check_quarantine(path)
    log_message("开始检查隔离属性: " .. path)
    
    -- 先检查文件是否存在
    if not execute_command(string.format('test -e "%s"', path)) then
        log_message("错误: 文件不存在: " .. path)
        return true
    end
    
    -- 检查是否有权限访问文件
    if not execute_command(string.format('test -r "%s"', path)) then
        log_message("错误: 无法访问文件: " .. path)
        return true
    end
    
    local success, output = execute_with_timeout(string.format('xattr -l "%s"', path), 5)
    if not success then
        log_message("检查隔离属性失败: " .. (output or "未知错误"))
        return true -- 如果检查失败，认为可能存在问题
    end
    
    -- 打印所有属性以便分析
    log_message("当前路径: " .. path)
    log_message("所有扩展属性:")
    log_message(output or "无")
    
    -- 只检查真正影响运行的隔离属性
    local critical_attrs = {
        "com.apple.quarantine"  -- 只检查这个关键属性
    }
    
    -- 记录所有发现的属性，但只对关键属性报错
    local found_attrs = {}
    for _, attr in ipairs(critical_attrs) do
        if output:find(attr) then
            table.insert(found_attrs, attr)
        end
    end
    
    -- 如果发现非关键属性，只记录不报错
    if output:find("com.apple.provenance") then
        log_message("注意: 发现 com.apple.provenance 属性，但这不影响应用运行")
    end
    if output:find("com.apple.macl") then
        log_message("注意: 发现 com.apple.macl 属性，但这不影响应用运行")
    end
    
    if #found_attrs > 0 then
        log_message("发现以下关键隔离属性:")
        for _, attr in ipairs(found_attrs) do
            -- 获取具体属性值
            local attr_value = execute_command(string.format('xattr -p "%s" "%s" 2>/dev/null', attr, path))
            log_message(string.format("  %s: %s", attr, attr_value or ""))
        end
        return true
    end
    
    -- 检查子目录中的关键属性
    local success, output = execute_with_timeout(
        string.format('find "%s" -type f -exec xattr -l {} \\;', path),
        10
    )
    if success and output and output ~= "" then
        log_message("子目录中的扩展属性:")
        log_message(output)
        
        -- 只检查子目录中的关键属性
        for _, attr in ipairs(critical_attrs) do
            if output:find(attr) then
                log_message("在子目录中发现关键隔离属性: " .. attr)
                return true
            end
        end
    end
    
    log_message("未发现影响运行的隔离属性")
    return false
end

-- 修改 remove_quarantine 函数
local function remove_quarantine(path)
    log_message("开始移除安全属性: " .. path)
    
    -- 先检查文件是否存在和权限
    if not execute_command(string.format('test -e "%s"', path)) then
        log_message("错误: 文件不存在: " .. path)
        return false
    end
    
    if not execute_command(string.format('test -w "%s"', path)) then
        log_message("错误: 无写入权限: " .. path)
        return false
    end
    
    -- 先打印初始状态
    log_message("移除前的属性状态:")
    local initial_attrs = execute_command(string.format('xattr -l "%s" 2>&1', path))
    log_message(initial_attrs or "无")
    
    -- 1. 尝试使用 -c 参数清除所有属性
    log_message("尝试使用 xattr -c 清除所有属性")
    local success, output = execute_with_timeout(
        string.format('xattr -c "%s" 2>&1', path),
        10
    )
    if not success then
        log_message("xattr -c 失败，尝试其他方法")
        log_message("错误输出: " .. (output or "无"))
    end
    
    -- 2. 尝试逐个移除特定属性
    local attrs = {
        "com.apple.macl",
        "com.apple.provenance",
        "com.apple.quarantine"
    }
    
    for _, attr in ipairs(attrs) do
        log_message("尝试移除属性: " .. attr)
        -- 先尝试普通移除
        local success, output = execute_with_timeout(
            string.format('xattr -r -d %s "%s" 2>&1', attr, path),
            10
        )
        if not success then
            log_message(string.format("普通移除失败: %s", attr))
            -- 尝试使用 sudo
            success, output = execute_with_timeout(
                string.format('sudo xattr -r -d %s "%s" 2>&1', attr, path),
                10
            )
            if not success then
                log_message(string.format("sudo 移除也失败: %s", attr))
                log_message("错误输出: " .. (output or "无"))
            end
        end
    end
    
    -- 3. 递归处理子目录
    log_message("处理子目录...")
    success, output = execute_with_timeout(
        string.format('find "%s" -type f -exec xattr -c {} \\;', path),
        30
    )
    if not success then
        log_message("处理子目录失败")
        log_message("错误输出: " .. (output or "无"))
    end
    
    -- 4. 修复权限
    success, output = execute_with_timeout(
        string.format('chmod -R 755 "%s"', path),
        10
    )
    if not success then
        log_message("修复权限失败")
        log_message("错误输出: " .. (output or "无"))
        return false
    end
    
    -- 5. 特别处理可执行文件
    local macosPath = path .. "/Contents/MacOS"
    -- 先检查目录是否存在
    if execute_command(string.format('test -d "%s"', macosPath)) then
        success, output = execute_with_timeout(
            string.format('find "%s" -type f -exec chmod +x {} \\;', macosPath),
            5
        )
        if not success then
            log_message("设置可执行权限失败")
            log_message("错误输出: " .. (output or "无"))
            log_message("继续执行...")
        end
    else
        log_message("MacOS 目录不存在，跳过设置可执行权限")
    end
    
    -- 6. 最终验证
    log_message("最终属性状态:")
    local final_attrs = execute_command(string.format('xattr -l "%s"', path))
    if final_attrs and final_attrs ~= "" then
        log_message("警告: 仍存在以下属性:")
        log_message(final_attrs)
        -- 对于顽固的属性，尝试使用 -c 强制清除
        success, output = execute_with_timeout(
            string.format('sudo xattr -c "%s" 2>&1', path),
            10
        )
        if not success then
            log_message("最终清除失败")
            log_message("错误输出: " .. (output or "无"))
            return false
        end
    else
        log_message("所有属性已清除")
    end
    
    log_message("安全属性移除完成")
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
        
        -- 复制新文件并立即检查结果
        file:write(string.format('copy /y "%s" "%s"\n', src, dst))
        
        -- 检查文件是否存在
        file:write(string.format('if not exist "%s" (\n', dst))
        -- 从备份恢复
        file:write(string.format('    copy /y "%s" "%s"\n', backup, dst))
        file:write(string.format('    start "" "%s"\n', dst))
        file:write('    exit /b 1\n')
        file:write(')\n')
        
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

-- Windows更新处理函数
local function perform_windows_update(target_path, new_version, backup_path, backup_file, app_root)
    -- 检查是否启用并存在更新助手
    local use_gui = false
    if g_config.windows_updater.use_gui then
        local updater_path = app_root .. path_sep .. g_config.windows_updater.updater_path
        if check_file_exists(updater_path) then
            use_gui = true
            log("找到更新助手: " .. updater_path)
        else
            log("更新助手未找到: " .. updater_path .. "，将使用批处理模式")
        end
    else
        log("更新助手未启用，使用批处理模式")
    end

    if use_gui then
        -- 使用GUI更新助手
        send_progress("install", 0, "准备安装新版本...")
        
        -- 创建更新信息文件
        local info_file = g_update_path .. path_sep .. "update_info.json"
        -- 转义路径中的反斜杠
        local info_str = string.format([[{
    "app_path": "%s",
    "new_version": "%s",
    "backup_path": "%s",
    "backup_file": "%s"
}]], target_path:gsub("\\", "\\\\"), 
    new_version:gsub("\\", "\\\\"), 
    backup_path:gsub("\\", "\\\\"), 
    backup_file:gsub("\\", "\\\\"))

        -- 写入文件
        local file = io.open(info_file, "w")
        if file then
            file:write(info_str)
            file:close()
            log("已创建更新信息文件: " .. info_file)
        else
            error("创建更新信息文件失败")
            return false
        end
        
        -- 修改启动更新助手的方式
        local updater_path = app_root .. path_sep .. g_config.windows_updater.updater_path
        -- 使用 cmd /c start 来启动独立进程
        local cmd = string.format(
            'cmd /c start "" /b "%s" -update "%s"',
            updater_path, 
            info_file
        )
        if not os_execute(cmd) then
            error("启动更新助手失败")
            return false
        end
        
        send_progress("install", 100, "更新助手已启动")
        log("更新助手已启动，程序即将重启...")
        return true
    else
        -- 使用批处理脚本
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
        return true
    end
end

-- 修改 perform_update 函数中的 Windows 处理部分
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
        if not remove_quarantine(new_version) then
            error("移除新版本隔离属性失败")
        end
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
        return perform_windows_update(target_path, new_version, backup_path, backup_file, app_root)
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
        
        if not is_windows() then
            -- 验证隔离属性是否已清除
            if check_quarantine(target_path) then
                log_message("错误: 仍存在隔离属性，准备回滚...")
                send_progress("verify", 50, "发现问题，准备回滚...")
                
                -- 删除更新后的文件
                remove_files(target_path)
                
                -- 恢复备份
                os.execute(string.format('tar -xzf "%s" -C "/"', backup_file))
                
                send_progress("verify", 100, "已回滚到备份版本")
                error("更新失败: 无法完全移除隔离属性")
                return false
            end
        end
        
        send_progress("verify", 100, "验证完成")
        log("更新完成")
        send_progress("complete", 100, "更新完成")
    end

    log_time(total_start, "更新总耗时")
    return true
end 