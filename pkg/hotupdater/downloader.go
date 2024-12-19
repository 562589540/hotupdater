package hotupdater

import (
	"context"
	"errors"
)

// DownloadImplementation 实际下载实现接口
type DownloadImplementation interface {
	// Execute 执行下载，并通过 onProgress 报告原始进度 (0-100)
	Execute(ctx context.Context, onProgress func(current, total int64, speed float64)) error
}

// Downloader 内部下载器
type Downloader struct {
	ctx          context.Context
	config       Config
	downloadImpl DownloadImplementation
}

// NewDownloader 创建下载器
func NewDownloader(ctx context.Context, config Config, impl DownloadImplementation) *Downloader {
	return &Downloader{
		ctx:          ctx,
		config:       config,
		downloadImpl: impl,
	}
}

// Execute 执行下载
func (d *Downloader) Execute() error {
	if d.downloadImpl == nil {
		return errors.New("no download implementation provided")
	}

	// 发送下载开始进度
	d.emitProgress(0, 0, 0)

	// 执行下载
	err := d.downloadImpl.Execute(d.ctx, func(current, total int64, speed float64) {
		d.emitProgress(current, total, speed)
	})

	if err != nil {
		return err
	}

	// 发送下载完成进度
	d.emitProgress(100, 100, 0)
	return nil
}

// emitProgress 发送进度信息
func (d *Downloader) emitProgress(current, total int64, speed float64) {
	if d.config.EventEmitter == nil {
		return
	}

	// 计算在下载阶段的总体进度
	percentage := CalculateProgress(PhaseDownload, current, total)

	d.config.EventEmitter.EmitProgress(UpdateProgress{
		Phase:      PhaseDownload,
		Percentage: percentage,
		Speed:      speed,
		Message:    PhaseMessages[PhaseDownload],
		Detail:     "正在下载更新包...",
	})

	// 记录日志
	if d.config.Logger != nil {
		d.config.Logger.Logf("下载进度: %d%%", percentage)
	}
}
