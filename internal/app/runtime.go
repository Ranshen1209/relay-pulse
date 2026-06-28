package app

import (
	"os"
	"path/filepath"

	"monitor/internal/config"
	"monitor/internal/inlineprobe"
	"monitor/internal/storage"
)

// buildChannelMigrationMappings 从配置构建 channel 迁移映射（同一 provider+service 取第一个非空 channel）
func buildChannelMigrationMappings(monitors []config.ServiceConfig) []storage.ChannelMigrationMapping {
	seen := make(map[string]bool)
	mappings := make([]storage.ChannelMigrationMapping, 0, len(monitors))

	for _, monitor := range monitors {
		// 跳过已禁用的监测项
		if monitor.Disabled {
			continue
		}
		// 跳过空 channel
		if monitor.Channel == "" {
			continue
		}

		key := monitor.Provider + "|" + monitor.Service
		if seen[key] {
			continue
		}
		seen[key] = true

		mappings = append(mappings, storage.ChannelMigrationMapping{
			Provider: monitor.Provider,
			Service:  monitor.Service,
			Channel:  monitor.Channel,
		})
	}

	return mappings
}

// resolveConfigDir 解析配置文件所在目录的绝对路径
func resolveConfigDir(configFile string) string {
	configDir := filepath.Dir(configFile)
	if configDir == "" || configDir == "." {
		if cwd, err := os.Getwd(); err == nil {
			return cwd
		}
	}
	return configDir
}

// initProbeTemplates 初始化 inlineprobe 包的模板注册表
func initProbeTemplates(configFile string) error {
	dir := filepath.Join(resolveConfigDir(configFile), "templates")
	inlineprobe.SetTemplatesDir(dir)
	return inlineprobe.InitTemplates(dir)
}

// announcementsConfigChanged 检测公告配置是否需要重建 Service
func announcementsConfigChanged(oldCfg, newCfg *config.AppConfig) bool {
	if oldCfg == nil || newCfg == nil {
		return true
	}
	oEnabled, nEnabled := oldCfg.Announcements.IsEnabled(), newCfg.Announcements.IsEnabled()
	if oEnabled != nEnabled {
		return true
	}
	if !oEnabled && !nEnabled {
		return false
	}
	o, n := oldCfg.Announcements, newCfg.Announcements
	return o.Owner != n.Owner ||
		o.Repo != n.Repo ||
		o.CategoryName != n.CategoryName ||
		o.PollIntervalDuration != n.PollIntervalDuration ||
		o.WindowHours != n.WindowHours ||
		o.MaxItems != n.MaxItems ||
		o.APIMaxAge != n.APIMaxAge ||
		oldCfg.GitHub.Token != newCfg.GitHub.Token ||
		oldCfg.GitHub.Proxy != newCfg.GitHub.Proxy ||
		oldCfg.GitHub.TimeoutDuration != newCfg.GitHub.TimeoutDuration
}
