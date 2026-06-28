package app

import (
	"reflect"
	"testing"
	"time"

	"monitor/internal/config"
	"monitor/internal/storage"
)

func TestBuildChannelMigrationMappings(t *testing.T) {
	monitors := []config.ServiceConfig{
		{Provider: "sakrylle", Service: "cc", Channel: ""},
		{Provider: "sakrylle", Service: "cc", Channel: "claude-kiro"},
		{Provider: "sakrylle", Service: "cc", Channel: "claude-kiro-special"},
		{Provider: "sakrylle", Service: "cx", Channel: "gpt-pro", Disabled: true},
		{Provider: "sakrylle", Service: "cx", Channel: "gpt-pro-special"},
		{Provider: "other", Service: "cc", Channel: "other-cc"},
	}

	got := buildChannelMigrationMappings(monitors)
	want := []storage.ChannelMigrationMapping{
		{Provider: "sakrylle", Service: "cc", Channel: "claude-kiro"},
		{Provider: "sakrylle", Service: "cx", Channel: "gpt-pro-special"},
		{Provider: "other", Service: "cc", Channel: "other-cc"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildChannelMigrationMappings() = %#v, want %#v", got, want)
	}
}

func TestAnnouncementsConfigChanged(t *testing.T) {
	enabled := true
	disabled := false
	base := &config.AppConfig{
		Announcements: config.AnnouncementsConfig{
			Enabled:              &enabled,
			Owner:                "Sakrylle",
			Repo:                 "Sakrylle Status",
			CategoryName:         "Announcements",
			PollIntervalDuration: 15 * time.Minute,
			WindowHours:          72,
			MaxItems:             20,
			APIMaxAge:            60,
		},
		GitHub: config.GitHubConfig{
			Token:           "token-a",
			Proxy:           "https://proxy.example",
			TimeoutDuration: 30 * time.Second,
		},
	}

	t.Run("same config", func(t *testing.T) {
		next := *base
		if announcementsConfigChanged(base, &next) {
			t.Fatal("announcementsConfigChanged() = true, want false")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		if !announcementsConfigChanged(nil, base) {
			t.Fatal("announcementsConfigChanged(nil, base) = false, want true")
		}
	})

	t.Run("disabled stays disabled", func(t *testing.T) {
		oldCfg := *base
		newCfg := *base
		oldCfg.Announcements.Enabled = &disabled
		newCfg.Announcements.Enabled = &disabled
		newCfg.Announcements.Owner = "changed"
		if announcementsConfigChanged(&oldCfg, &newCfg) {
			t.Fatal("announcementsConfigChanged(disabled, disabled) = true, want false")
		}
	})

	t.Run("enable flag changes", func(t *testing.T) {
		next := *base
		next.Announcements.Enabled = &disabled
		if !announcementsConfigChanged(base, &next) {
			t.Fatal("announcementsConfigChanged() = false, want true")
		}
	})

	t.Run("github token changes", func(t *testing.T) {
		next := *base
		next.GitHub.Token = "token-b"
		if !announcementsConfigChanged(base, &next) {
			t.Fatal("announcementsConfigChanged() = false, want true")
		}
	})
}
