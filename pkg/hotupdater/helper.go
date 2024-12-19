package hotupdater

import (
	"encoding/json"
	"os"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// helper 提供共享的辅助函数
type helper struct {
	logger  Logger
	emitter EventEmitter
}

func newHelper(logger Logger, emitter EventEmitter) *helper {
	return &helper{
		logger:  logger,
		emitter: emitter,
	}
}

// 日志函数注册到 Lua
func (h *helper) registerLogger(L *lua.LState) {
	L.SetGlobal("log_message", L.NewFunction(func(L *lua.LState) int {
		msg := L.ToString(1)

		// 检查是否是进度消息
		if strings.HasPrefix(msg, "@PROGRESS@") {
			if h.emitter != nil {
				if progress := ParseProgressMessage(strings.TrimPrefix(msg, "@PROGRESS@")); progress != nil {
					h.emitter.EmitProgress(*progress)
				}
			}
		} else {
			// 普通日志消息
			if h.logger != nil {
				h.logger.Logf("%s", msg)
			}
		}
		return 0
	}))
}

// writeUpdateInfo 写入更新信息到文件
func (h *helper) writeUpdateInfo(path string, info map[string]string) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// executeLuaScript 执行 Lua 更新脚本
func (h *helper) executeLuaScript(L *lua.LState, scriptPath string, params map[string]string) error {
	// 注册日志函数
	h.registerLogger(L)

	// 注册系统命令执行函数
	L.SetGlobal("os_execute", L.NewFunction(func(L *lua.LState) int {
		cmd := L.ToString(1)
		result := ExecuteCommand(cmd)
		L.Push(lua.LBool(result))
		return 1
	}))

	// 加载并执行脚本
	if err := L.DoFile(scriptPath); err != nil {
		return err
	}

	// 创建参数表
	paramsTable := L.NewTable()
	for k, v := range params {
		L.SetField(paramsTable, k, lua.LString(v))
	}

	// 调用更新函数，传入参数表
	return L.CallByParam(lua.P{
		Fn:      L.GetGlobal("perform_update"),
		NRet:    0,
		Protect: true,
	}, paramsTable)
}
