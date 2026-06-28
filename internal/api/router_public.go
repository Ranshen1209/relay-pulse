package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"monitor/internal/buildinfo"
)

func registerPublicAPIRoutes(router *gin.Engine, handler *Handler) {
	// 注册 API 路由
	router.GET("/api/status", handler.GetStatus)
	router.GET("/api/status/query", handler.GetStatusQuery)
	router.POST("/api/status/batch", handler.PostStatusBatch)

	// 事件 API 路由
	router.GET("/api/events", handler.GetEvents)
	router.GET("/api/events/latest", handler.GetLatestEventID)

	// 自助收录 API 路由
	router.GET("/api/onboarding/meta", handler.GetOnboardingMeta)
	router.POST("/api/onboarding/test", handler.OnboardingTest)
	router.POST("/api/onboarding/submit", handler.SubmitOnboarding)
	router.GET("/api/onboarding/:id", handler.GetOnboardingStatus)
}

func registerChangeRoutes(router *gin.Engine, handler *Handler) {
	// 变更请求 API 路由
	router.POST("/api/change/auth", handler.AuthChange)
	router.POST("/api/change/submit", handler.SubmitChange)
	router.GET("/api/change/:id", handler.GetChangeStatus)
}

func registerSEORoutes(router *gin.Engine, handler *Handler) {
	// SEO 路由
	router.GET("/sitemap.xml", handler.GetSitemap)
	router.GET("/robots.txt", handler.GetRobots)
}

func registerVersionRoute(router *gin.Engine) {
	// 版本信息 API
	router.GET("/api/version", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{
			"version":    buildinfo.GetVersion(),
			"git_commit": buildinfo.GetGitCommit(),
			"build_time": buildinfo.GetBuildTime(),
			"go_version": buildinfo.GetGoVersion(),
		})
	})
}
