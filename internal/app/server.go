package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"monitor/internal/announcements"
	"monitor/internal/api"
	"monitor/internal/apikey"
	"monitor/internal/automove"
	"monitor/internal/buildinfo"
	"monitor/internal/change"
	"monitor/internal/config"
	"monitor/internal/events"
	"monitor/internal/identity"
	"monitor/internal/inlineprobe"
	"monitor/internal/logger"
	"monitor/internal/onboarding"
	"monitor/internal/scheduler"
	"monitor/internal/storage"
)

// Run 启动主服务并阻塞直到收到关闭信号。
func Run(configFile string) error {
	// 打印版本信息
	logger.Info("main", "Relay Pulse Monitor 启动",
		"version", buildinfo.GetVersion(),
		"git_commit", buildinfo.GetGitCommit(),
		"build_time", buildinfo.GetBuildTime())

	// 创建配置加载器
	loader := config.NewLoader()

	// 初始加载配置
	cfg, err := loader.Load(configFile)
	if err != nil {
		logger.Error("main", "无法加载配置文件", "error", err)
		return fmt.Errorf("加载配置文件失败: %w", err)
	}

	logger.Info("main", "配置加载完成",
		"monitors", len(cfg.Monitors),
		"interval", cfg.Interval,
		"max_concurrency", cfg.MaxConcurrency,
		"stagger_probes", cfg.StaggerProbes,
		"slow_latency", cfg.SlowLatency,
		"degraded_weight", cfg.DegradedWeight,
	)

	// 初始化存储（支持 SQLite 和 PostgreSQL）
	store, err := storage.New(&cfg.Storage)
	if err != nil {
		logger.Error("main", "初始化存储失败", "error", err)
		return fmt.Errorf("初始化存储失败: %w", err)
	}
	defer store.Close()

	if err := store.Init(); err != nil {
		logger.Error("main", "初始化数据库失败", "error", err)
		return fmt.Errorf("初始化数据库失败: %w", err)
	}

	// 自动迁移旧数据的 channel
	if err := store.MigrateChannelData(buildChannelMigrationMappings(cfg.Monitors)); err != nil {
		logger.Warn("main", "channel 数据迁移失败", "error", err)
	}

	storageType := cfg.Storage.Type
	if storageType == "" {
		storageType = "sqlite"
	}
	logger.Info("main", "存储已就绪", "type", storageType)

	// 创建上下文（用于优雅关闭）
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动历史数据清理任务
	var cleaner *storage.Cleaner
	if cfg.Storage.Retention.IsEnabled() {
		cleaner = storage.NewCleaner(store, &cfg.Storage.Retention)
		go cleaner.Start(ctx)
		logger.Info("main", "历史数据清理任务已启动",
			"retention_days", cfg.Storage.Retention.Days,
			"cleanup_interval", cfg.Storage.Retention.CleanupInterval)
	}

	// 启动历史数据归档任务（仅 PostgreSQL 支持）
	var archiver *storage.Archiver
	if cfg.Storage.Archive.IsEnabled() {
		// 检查存储是否支持归档（仅 PostgreSQL 支持）
		if _, ok := store.(storage.ArchiveStorage); !ok {
			logger.Warn("main", "归档功能已启用但当前存储不支持（仅 PostgreSQL 支持），归档任务将不会执行",
				"storage_type", cfg.Storage.Type)
		} else {
			archiver = storage.NewArchiver(store, &cfg.Storage.Archive)
			go archiver.Start(ctx)
			logger.Info("main", "历史数据归档任务已启动",
				"archive_days", cfg.Storage.Archive.ArchiveDays,
				"backfill_days", cfg.Storage.Archive.BackfillDays,
				"output_dir", cfg.Storage.Archive.OutputDir,
				"format", cfg.Storage.Archive.Format)
		}
	}

	// 创建自动移板服务（始终创建，内部根据 enabled 决定是否实际评估）
	// 先从 DB 恢复持久化 override，再基于历史做首次评估，
	// 避免重启后丢失 sticky cold 状态，导致 scheduler 首轮调度到本应停探的通道。
	autoMover := automove.NewService(store, cfg)
	if err := autoMover.Restore(); err != nil {
		logger.Error("main", "恢复自动移板 override 失败", "error", err)
		return fmt.Errorf("恢复自动移板 override 失败: %w", err)
	}
	autoMover.Evaluate(ctx)

	// 创建调度器（支持通过 config.yaml 配置 interval）
	interval := cfg.IntervalDuration
	if interval <= 0 {
		interval = time.Minute
	}
	userIDMgr := identity.NewUserIDManager()
	sched := scheduler.NewScheduler(store, interval, userIDMgr)
	sched.SetAutoMover(autoMover)

	// 创建事件服务（如果启用）
	eventSvc, err := events.NewService(events.ServiceConfig{
		DetectorConfig: events.DetectorConfig{
			DownThreshold: cfg.Events.DownThreshold,
			UpThreshold:   cfg.Events.UpThreshold,
		},
		ChannelDetectorConfig: events.ChannelDetectorConfig{
			DownThreshold: cfg.Events.ChannelDownThreshold,
		},
		Mode:             cfg.Events.Mode,
		ChannelCountMode: cfg.Events.ChannelCountMode,
		Enabled:          cfg.Events.Enabled,
	}, store)
	if err != nil {
		logger.Error("main", "创建事件服务失败", "error", err)
		return fmt.Errorf("创建事件服务失败: %w", err)
	}
	if eventSvc.IsEnabled() {
		sched.SetEventService(eventSvc)
		logger.Info("main", "事件服务已启用",
			"mode", eventSvc.GetMode(),
			"down_threshold", cfg.Events.DownThreshold,
			"up_threshold", cfg.Events.UpThreshold,
			"channel_down_threshold", cfg.Events.ChannelDownThreshold,
			"channel_count_mode", cfg.Events.ChannelCountMode)
	}

	sched.Start(ctx, cfg)

	// 初始化 monitors.d/ 目录和 MonitorStore
	monitorsDirPath := filepath.Join(resolveConfigDir(configFile), config.MonitorsDirName)
	if err := os.MkdirAll(monitorsDirPath, 0755); err != nil {
		logger.Warn("main", "创建 monitors.d 目录失败", "error", err)
	}
	monitorStore := config.NewMonitorStore(monitorsDirPath)

	// 创建API服务器
	server := api.NewServer(store, cfg, "8080", autoMover)
	server.GetHandler().SetMonitorStore(monitorStore)

	// runtimeMu 保护热更新回调与关闭序列之间对 mutable 组件实例的并发访问
	var runtimeMu sync.Mutex
	runtimeCfg := cfg

	// 连接 override 变更回调：当 autoMover 产生新 cold override 时通知 scheduler/events 刷新
	autoMover.SetOnOverrideChange(func() {
		runtimeMu.Lock()
		defer runtimeMu.Unlock()
		if ctx.Err() != nil {
			return
		}
		sched.UpdateConfig(runtimeCfg)
	})
	autoMover.Start(ctx)
	if cfg.Boards.Enabled && cfg.Boards.AutoMove.Enabled {
		logger.Info("main", "自动移板服务已启用",
			"threshold_cold", cfg.Boards.AutoMove.ThresholdCold,
			"threshold_down", cfg.Boards.AutoMove.ThresholdDown,
			"threshold_up", cfg.Boards.AutoMove.ThresholdUp,
			"check_interval", cfg.Boards.AutoMove.CheckInterval,
			"min_probes", cfg.Boards.AutoMove.MinProbes)
	}

	// 提前注册公告 API 路由（使用支持动态替换的 Handler），避免热启用后无路由入口
	announcementsHandler := announcements.NewHandler(nil)
	server.RegisterAnnouncementsHandler(announcementsHandler.GetAnnouncements)

	// announcementsAppliedCfg 初始为 nil：仅在服务创建成功或明确禁用时设置，
	// 避免启动失败时标记为"已应用"导致后续热更新不再重试
	var announcementsAppliedCfg *config.AppConfig

	// 初始化内联探测器（供收录测试和管理后台使用）
	if err := initProbeTemplates(configFile); err != nil {
		logger.Warn("main", "初始化 inlineprobe 模板失败（非致命）", "error", err)
	}
	inlineProber := inlineprobe.NewInlineProber(5, userIDMgr)
	probeLimiter := inlineprobe.NewIPLimiter(10, 10) // 每 IP 每分钟 10 次
	server.GetHandler().SetInlineProber(inlineProber)
	server.GetHandler().SetProbeLimiter(probeLimiter)
	logger.Info("main", "内联探测器已初始化")

	// 初始化自助收录服务（如果启用）
	var onboardingSvc *onboarding.Service
	if cfg.Onboarding.Enabled {
		var obStore onboarding.Store
		switch s := store.(type) {
		case *storage.SQLiteStorage:
			sqlStore := onboarding.NewSQLStore(s.SqlDB())
			if err := sqlStore.InitTable(ctx); err != nil {
				logger.Error("main", "初始化 onboarding 表失败", "error", err)
				return fmt.Errorf("初始化 onboarding 表失败: %w", err)
			}
			obStore = sqlStore
		case *storage.PostgresStorage:
			pgxStore := onboarding.NewPgxStore(s.PgxPool())
			if err := pgxStore.InitTable(ctx); err != nil {
				logger.Error("main", "初始化 onboarding 表失败", "error", err)
				return fmt.Errorf("初始化 onboarding 表失败: %w", err)
			}
			obStore = pgxStore
		default:
			logger.Error("main", "不支持的存储类型，onboarding 功能不可用")
			return fmt.Errorf("不支持的存储类型，onboarding 功能不可用")
		}

		configDir := resolveConfigDir(configFile)
		var err error
		onboardingSvc, err = onboarding.NewService(obStore, &cfg.Onboarding, configDir)
		if err != nil {
			logger.Error("main", "创建自助收录服务失败", "error", err)
			return fmt.Errorf("创建自助收录服务失败: %w", err)
		}
		onboardingSvc.SetMonitorStore(monitorStore)
		onboardingSvc.SetConfigMonitorCheck(func(provider, service, channel string) bool {
			runtimeMu.Lock()
			defer runtimeMu.Unlock()
			targetKey := config.MonitorFileKeyFromPSC(provider, service, channel)
			for _, m := range runtimeCfg.Monitors {
				if config.MonitorFileKeyFromPSC(m.Provider, m.Service, m.Channel) == targetKey {
					return true
				}
			}
			return false
		})
		server.GetHandler().SetOnboardingService(onboardingSvc)
		logger.Info("main", "自助收录功能已启用",
			"max_per_ip_per_day", cfg.Onboarding.MaxPerIPPerDay,
			"proof_ttl", cfg.Onboarding.ProofTTL)
	}

	// 初始化变更请求服务（如果启用）
	var changeSvc *change.Service
	if cfg.ChangeRequests.Enabled {
		// 需要 onboarding 的 encryption_key 和 proof_secret
		if cfg.Onboarding.EncryptionKey == "" || cfg.Onboarding.ProofSecret == "" {
			logger.Error("main", "变更请求需要 onboarding.encryption_key 和 onboarding.proof_secret")
			return fmt.Errorf("变更请求需要 onboarding.encryption_key 和 onboarding.proof_secret")
		}

		cipher, err := apikey.NewKeyCipher(cfg.Onboarding.EncryptionKey)
		if err != nil {
			logger.Error("main", "创建 API Key 加密器失败", "error", err)
			return fmt.Errorf("创建 API Key 加密器失败: %w", err)
		}
		proofIssuer := apikey.NewProofIssuer(cfg.Onboarding.ProofSecret, cfg.Onboarding.ProofTTLDuration)

		var chStore change.Store
		switch s := store.(type) {
		case *storage.SQLiteStorage:
			sqlStore := change.NewSQLStore(s.SqlDB())
			if err := sqlStore.InitTable(ctx); err != nil {
				logger.Error("main", "初始化 change_requests 表失败", "error", err)
				return fmt.Errorf("初始化 change_requests 表失败: %w", err)
			}
			chStore = sqlStore
		case *storage.PostgresStorage:
			pgxStore := change.NewPgxStore(s.PgxPool())
			if err := pgxStore.InitTable(ctx); err != nil {
				logger.Error("main", "初始化 change_requests 表失败", "error", err)
				return fmt.Errorf("初始化 change_requests 表失败: %w", err)
			}
			chStore = pgxStore
		default:
			logger.Error("main", "不支持的存储类型，变更请求功能不可用")
			return fmt.Errorf("不支持的存储类型，变更请求功能不可用")
		}

		changeSvc = change.NewService(chStore, cipher, proofIssuer, &cfg.ChangeRequests)
		changeSvc.SetMonitorStore(monitorStore)
		changeSvc.UpdateConfig(&cfg.ChangeRequests, cfg.Monitors)
		server.GetHandler().SetChangeService(changeSvc)
		logger.Info("main", "变更请求功能已启用",
			"max_per_ip_per_day", cfg.ChangeRequests.MaxPerIPPerDay)
	}

	// 初始化公告服务（如果启用）
	var announcementsSvc *announcements.Service
	if cfg.Announcements.IsEnabled() {
		var err error
		announcementsSvc, err = announcements.NewService(cfg.Announcements, cfg.GitHub)
		if err != nil {
			logger.Error("main", "创建公告服务失败", "error", err)
			// 公告服务失败不影响主服务启动，仅警告
			// 注意：不设置 announcementsAppliedCfg，下次热更新将重试创建
		} else {
			announcementsHandler.SetService(announcementsSvc)
			announcementsSvc.Start(ctx)
			announcementsAppliedCfg = cfg
			logger.Info("main", "公告服务已启用",
				"owner", cfg.Announcements.Owner,
				"repo", cfg.Announcements.Repo,
				"category", cfg.Announcements.CategoryName,
				"poll_interval", cfg.Announcements.PollInterval,
				"window_hours", cfg.Announcements.WindowHours)
		}
	} else {
		announcementsAppliedCfg = cfg // 明确禁用也标记为已应用
	}

	startConfigWatcher(ctx, loader, configFile, &runtimeMu, &runtimeCfg, server, autoMover, sched, store, &cleaner, &archiver, changeSvc, announcementsHandler, &announcementsSvc, &announcementsAppliedCfg)
	waitForShutdown(cancel, server, sched, autoMover, &runtimeMu, announcementsHandler, &announcementsSvc, &cleaner, &archiver)

	logger.Info("main", "服务已安全退出")
	return nil
}
