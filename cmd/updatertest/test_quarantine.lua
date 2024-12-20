-- test_quarantine.lua

-- 模拟日志函数
function log_message(msg)
    print(msg)
end

-- 执行命令并返回输出
function execute_command(cmd)
    local handle = io.popen(cmd)
    local result = handle:read("*a")
    handle:close()
    return result
end

-- 测试函数
function test_remove_quarantine()
    -- 测试路径
    local test_app = "/Users/zhaojian/Downloads/绘影.app"
    
    -- 第一步: 检查应用状态
    print("\n=== 初始状态检查 ===")
    print("扩展属性列表:")
    print(execute_command(string.format("xattr -l '%s'", test_app)))
    print("\n文件权限:")
    print(execute_command(string.format("ls -la '%s'", test_app)))
    
    -- 第二步: 逐个测试命令
    print("\n=== 测试1: xattr -cr ===")
    local start_time = os.time()
    local result = execute_command(string.format("xattr -cr '%s' 2>&1", test_app))
    print(string.format("耗时: %d秒, 输出: %s", os.time() - start_time, result))
    
    print("\n=== 测试2: xattr -r -d com.apple.provenance ===")
    start_time = os.time()
    result = execute_command(string.format("xattr -r -d com.apple.provenance '%s' 2>&1", test_app))
    print(string.format("耗时: %d秒, 输出: %s", os.time() - start_time, result))
    
    print("\n=== 测试3: xattr -r -d com.apple.quarantine ===")
    start_time = os.time()
    result = execute_command(string.format("xattr -r -d com.apple.quarantine '%s' 2>&1", test_app))
    print(string.format("耗时: %d秒, 输出: %s", os.time() - start_time, result))
    
    print("\n=== 测试4: spctl --add ===")
    start_time = os.time()
    result = execute_command(string.format("spctl --add '%s' 2>&1", test_app))
    print(string.format("耗时: %d秒, 输出: %s", os.time() - start_time, result))
    
    print("\n=== 测试5: chmod -R 755 ===")
    start_time = os.time()
    result = execute_command(string.format("chmod -R 755 '%s' 2>&1", test_app))
    print(string.format("耗时: %d秒, 输出: %s", os.time() - start_time, result))
    
    print("\n=== 测试6: chmod +x MacOS/* ===")
    start_time = os.time()
    result = execute_command(string.format("chmod +x '%s/Contents/MacOS/*' 2>&1", test_app))
    print(string.format("耗时: %d秒, 输出: %s", os.time() - start_time, result))
    
    -- 最后检查状态
    print("\n=== 最终状态检查 ===")
    print("扩展属性列表:")
    print(execute_command(string.format("xattr -l '%s'", test_app)))
    print("\n文件权限:")
    print(execute_command(string.format("ls -la '%s'", test_app)))
    
    -- 系统状态
    print("\n=== 系统状态 ===")
    print("xattr 进程:")
    print(execute_command("ps aux | grep xattr"))
    print("\nspctl 进程:")
    print(execute_command("ps aux | grep spctl"))
    print("\n文件系统状态:")
    print(execute_command("lsof | grep 绘影.app"))
end

-- 运行测试
test_remove_quarantine() 