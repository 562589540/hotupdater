package hotupdater

import (
	"strconv"
	"strings"
)

type UpdatePhase string

const (
	PhaseDownload UpdatePhase = "download" // 下载新版本
	PhasePreCheck UpdatePhase = "precheck" // 更新前检查
	PhaseBackup   UpdatePhase = "backup"   // 备份当前版本
	PhaseInstall  UpdatePhase = "install"  // 安装新版本
	PhaseVerify   UpdatePhase = "verify"   // 验证安装
	PhaseComplete UpdatePhase = "complete" // 更新完成
)

// 每个阶段的进度范围
var PhaseRanges = map[UpdatePhase]struct{ Start, End int }{
	PhaseDownload: {0, 70},   // 0-70%，下载占大部分时间
	PhasePreCheck: {70, 75},  // 70-75%，检查很快
	PhaseBackup:   {75, 85},  // 75-85%，备份也较快
	PhaseInstall:  {85, 95},  // 85-95%，安装也不会太慢
	PhaseVerify:   {95, 98},  // 95-98%，验证很快
	PhaseComplete: {98, 100}, // 98-100%，完成
}

// 用户友好的提示信息
var PhaseMessages = map[UpdatePhase]string{
	PhaseDownload: "正在下载更新...",
	PhasePreCheck: "正在准备更新...",
	PhaseBackup:   "正在备份当前版本...",
	PhaseInstall:  "正在安装新版本...",
	PhaseVerify:   "正在验证安装...",
	PhaseComplete: "更新完成",
}

type UpdateProgress struct {
	Phase      UpdatePhase `json:"phase"`      // 当前阶段
	Percentage int         `json:"percentage"` // 总体进度百分比(0-100)
	Speed      float64     `json:"speed"`      // 下载速度(MB/s)
	Message    string      `json:"message"`    // 用户友好的提示信息
	Detail     string      `json:"detail"`     // 详细信息(可选)
}

// 计算阶段内的进度百分比
func CalculateProgress(phase UpdatePhase, current, total int64) int {
	if total == 0 {
		return PhaseRanges[phase].Start
	}

	r := PhaseRanges[phase]
	progress := float64(current) / float64(total)
	return r.Start + int(progress*float64(r.End-r.Start))
}

// ParseProgressMessage 解析进度消息
func ParseProgressMessage(data string) *UpdateProgress {
	parts := strings.Split(data, "|")
	if len(parts) != 3 {
		return nil
	}

	phase := UpdatePhase(parts[0])
	percentage, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil
	}
	detail := parts[2]

	// 计算总体进度
	totalPercentage := CalculateProgress(phase, int64(percentage), 100)

	return &UpdateProgress{
		Phase:      phase,
		Percentage: totalPercentage,
		Message:    PhaseMessages[phase],
		Detail:     detail,
	}
}
