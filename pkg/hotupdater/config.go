package hotupdater

// Config 热更新配置
type Config struct {
	UpdatePath   string       // 更新文件存放路径
	BackupPath   string       // 备份路径
	ScriptPath   string       // Lua脚本路径
	OnUpdate     func(error)  // 更新回调
	Logger       Logger       // 日志接口
	EventEmitter EventEmitter // 事件发送器
	DownloadImpl DownloadImplementation
}

// Logger 日志接口
type Logger interface {
	Log(message string)
	Logf(format string, args ...interface{})
}

// EventEmitter 事件发送接口
type EventEmitter interface {
	// EmitLog 发送日志事件
	EmitLog(message string)
	// EmitProgress 发送进度事件
	EmitProgress(progress UpdateProgress)
}

// UpdateStatus 更新状态
type UpdateStatus struct {
	Type    string `json:"type"`    // 状态类型
	Message string `json:"message"` // 状态消息
	Error   string `json:"error"`   // 错误信息（如果有）
}

// 添加复制方法
func (c Config) Clone() Config {
	return Config{
		UpdatePath:   c.UpdatePath,
		BackupPath:   c.BackupPath,
		ScriptPath:   c.ScriptPath,
		OnUpdate:     c.OnUpdate,
		Logger:       c.Logger,
		EventEmitter: c.EventEmitter,
		DownloadImpl: c.DownloadImpl,
	}
}
