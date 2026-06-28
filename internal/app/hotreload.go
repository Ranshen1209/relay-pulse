package app

import (
	"context"
	"sync"

	"monitor/internal/announcements"
	"monitor/internal/api"
	"monitor/internal/automove"
	"monitor/internal/change"
	"monitor/internal/config"
	"monitor/internal/logger"
	"monitor/internal/scheduler"
	"monitor/internal/storage"
)

func startConfigWatcher(
	ctx context.Context,
	loader *config.Loader,
	configFile string,
	runtimeMu *sync.Mutex,
	runtimeCfg **config.AppConfig,
	server *api.Server,
	autoMover *automove.Service,
	sched *scheduler.Scheduler,
	store storage.Storage,
	cleaner **storage.Cleaner,
	archiver **storage.Archiver,
	changeSvc *change.Service,
	announcementsHandler *announcements.Handler,
	announcementsSvc **announcements.Service,
	announcementsAppliedCfg **config.AppConfig,
) {
	watcher, err := config.NewWatcher(loader, configFile, func(newCfg *config.AppConfig) {
		// 关闭中不再处理热更新
		if ctx.Err() != nil {
			return
		}

		// 序列化热更新回调，防止与关闭序列竞态
		runtimeMu.Lock()
		defer runtimeMu.Unlock()

		if ctx.Err() != nil {
			return
		}

		// === 已有热更新支持的组件 ===
		// 顺序重要：先更新 autoMover（可能清除/生成 cold override），
		// 再更新 scheduler（基于最新 override 重建任务堆），最后更新 server
		*runtimeCfg = newCfg
		server.UpdateConfig(newCfg)
		autoMover.UpdateConfig(newCfg)
		sched.UpdateConfig(newCfg)

		// 重新运行 channel 迁移（支持运行时添加 channel）
		if err := store.MigrateChannelData(buildChannelMigrationMappings(newCfg.Monitors)); err != nil {
			logger.Warn("main", "热更新时 channel 迁移失败", "error", err)
		}
		// 注意：不再调用 TriggerNow()，rebuildTasks 已安排错峰首次执行
		// 避免与 rebuildTasks 的首轮调度产生竞态导致重复探测

		// === Cleaner: ApplyConfig（在线更新配置 + 唤醒调度） ===
		switch {
		case newCfg.Storage.Retention.IsEnabled() && *cleaner == nil:
			*cleaner = storage.NewCleaner(store, &newCfg.Storage.Retention)
			go (*cleaner).Start(ctx)
			logger.Info("main", "历史数据清理任务已在热更新后启动",
				"retention_days", newCfg.Storage.Retention.Days,
				"cleanup_interval", newCfg.Storage.Retention.CleanupInterval)
		case newCfg.Storage.Retention.IsEnabled() && *cleaner != nil:
			(*cleaner).UpdateRetentionConfig(&newCfg.Storage.Retention)
		case !newCfg.Storage.Retention.IsEnabled() && *cleaner != nil:
			(*cleaner).Stop()
			*cleaner = nil
			logger.Info("main", "历史数据清理任务已在热更新后停用")
		}

		// === Archiver: ApplyConfig（在线更新配置 + 唤醒调度） ===
		switch {
		case newCfg.Storage.Archive.IsEnabled() && *archiver == nil:
			if _, ok := store.(storage.ArchiveStorage); !ok {
				logger.Warn("main", "热更新后启用了归档功能，但当前存储不支持（仅 PostgreSQL 支持）",
					"storage_type", newCfg.Storage.Type)
			} else {
				*archiver = storage.NewArchiver(store, &newCfg.Storage.Archive)
				go (*archiver).Start(ctx)
				logger.Info("main", "历史数据归档任务已在热更新后启动",
					"archive_days", newCfg.Storage.Archive.ArchiveDays,
					"backfill_days", newCfg.Storage.Archive.BackfillDays,
					"output_dir", newCfg.Storage.Archive.OutputDir,
					"format", newCfg.Storage.Archive.Format)
			}
		case newCfg.Storage.Archive.IsEnabled() && *archiver != nil:
			(*archiver).UpdateArchiveConfig(&newCfg.Storage.Archive)
		case !newCfg.Storage.Archive.IsEnabled() && *archiver != nil:
			(*archiver).Stop()
			*archiver = nil
			logger.Info("main", "历史数据归档任务已在热更新后停用")
		}

		// === Probe: 刷新模板变体注册表（模板文件可能已新增/删除） ===
		if err := initProbeTemplates(configFile); err != nil {
			logger.Warn("main", "热更新 inlineprobe 模板失败", "error", err)
		}

		// === ChangeRequests: 热更新认证索引 ===
		if changeSvc != nil && newCfg.ChangeRequests.Enabled {
			changeSvc.UpdateConfig(&newCfg.ChangeRequests, newCfg.Monitors)
		}

		// === Announcements: RecreateOnChange（stop + 重建） ===
		if announcementsConfigChanged(*announcementsAppliedCfg, newCfg) {
			if !newCfg.Announcements.IsEnabled() {
				announcementsHandler.SetService(nil)
				if *announcementsSvc != nil {
					(*announcementsSvc).Stop()
					*announcementsSvc = nil
					logger.Info("main", "公告服务已在热更新后停用")
				}
				*announcementsAppliedCfg = newCfg
			} else {
				newSvc, err := announcements.NewService(newCfg.Announcements, newCfg.GitHub)
				if err != nil {
					logger.Error("main", "热更新创建公告服务失败，继续使用旧实例", "error", err)
				} else {
					// 先启动新实例，再切换引用，最后停止旧实例（减少不可用窗口）
					newSvc.Start(ctx)
					oldSvc := *announcementsSvc
					*announcementsSvc = newSvc
					announcementsHandler.SetService(newSvc)
					if oldSvc != nil {
						oldSvc.Stop()
					}
					*announcementsAppliedCfg = newCfg
					logger.Info("main", "公告服务配置热更新已生效",
						"owner", newCfg.Announcements.Owner,
						"repo", newCfg.Announcements.Repo,
						"category", newCfg.Announcements.CategoryName,
						"poll_interval", newCfg.Announcements.PollInterval,
						"window_hours", newCfg.Announcements.WindowHours)
				}
			}
		}
	})

	if err != nil {
		logger.Warn("main", "配置监听器创建失败，热更新功能不可用", "error", err)
	} else {
		if err := watcher.Start(ctx); err != nil {
			logger.Warn("main", "配置监听器启动失败，热更新功能不可用", "error", err)
		} else {
			logger.Info("main", "配置热更新已启用")
		}
	}
}
