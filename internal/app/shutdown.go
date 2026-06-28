package app

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"monitor/internal/announcements"
	"monitor/internal/api"
	"monitor/internal/automove"
	"monitor/internal/logger"
	"monitor/internal/scheduler"
	"monitor/internal/storage"
)

func waitForShutdown(
	cancel context.CancelFunc,
	server *api.Server,
	sched *scheduler.Scheduler,
	autoMover *automove.Service,
	runtimeMu *sync.Mutex,
	announcementsHandler *announcements.Handler,
	announcementsSvc **announcements.Service,
	cleaner **storage.Cleaner,
	archiver **storage.Archiver,
) {
	// 监听中断信号（优雅关闭）
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 启动HTTP服务器（阻塞）
	go func() {
		if err := server.Start(); err != nil {
			logger.Error("main", "HTTP服务器错误", "error", err)
			cancel()
			// 向信号通道发送信号，确保进程退出
			sigChan <- syscall.SIGTERM
		}
	}()

	// 等待中断信号
	<-sigChan
	logger.Info("main", "收到关闭信号，正在优雅退出")

	// 取消上下文
	cancel()

	// 停止调度器
	sched.Stop()

	// 停止自动移板服务
	autoMover.Stop()
	logger.Info("main", "自动移板服务已关闭")

	// 在 runtimeMu 保护下捕获当前实例引用，防止与热更新回调竞态
	runtimeMu.Lock()
	currentAnnouncementsSvc := *announcementsSvc
	*announcementsSvc = nil
	currentCleaner := *cleaner
	*cleaner = nil
	currentArchiver := *archiver
	*archiver = nil
	announcementsHandler.SetService(nil)
	runtimeMu.Unlock()

	// 停止公告服务（如果启用）
	if currentAnnouncementsSvc != nil {
		currentAnnouncementsSvc.Stop()
		logger.Info("main", "公告服务已关闭")
	}

	// 停止清理和归档任务
	if currentCleaner != nil {
		currentCleaner.Stop()
		logger.Info("main", "历史数据清理任务已关闭")
	}
	if currentArchiver != nil {
		currentArchiver.Stop()
		logger.Info("main", "历史数据归档任务已关闭")
	}

	// 停止HTTP服务器
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Stop(shutdownCtx); err != nil {
		logger.Warn("main", "HTTP服务器关闭错误", "error", err)
	}
}
