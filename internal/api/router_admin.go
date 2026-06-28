package api

import "github.com/gin-gonic/gin"

func registerAdminSubmissionRoutes(router *gin.Engine, handler *Handler) {
	// 管理后台 API 路由（需 Bearer token 鉴权）
	router.GET("/api/admin/submissions", handler.AdminListSubmissions)
	router.GET("/api/admin/submissions/:id", handler.AdminGetSubmission)
	router.PUT("/api/admin/submissions/:id", handler.AdminUpdateSubmission)
	router.DELETE("/api/admin/submissions/:id", handler.AdminDeleteSubmission)
	router.POST("/api/admin/submissions/:id/test", handler.AdminTestSubmission)
	router.POST("/api/admin/submissions/:id/reject", handler.AdminRejectSubmission)
	router.POST("/api/admin/submissions/:id/publish", handler.AdminPublishSubmission)
}

func registerAdminChangeRoutes(router *gin.Engine, handler *Handler) {
	// 管理后台 — 变更请求 API（需 Bearer token 鉴权）
	router.GET("/api/admin/changes", handler.AdminListChanges)
	router.GET("/api/admin/changes/:id", handler.AdminGetChange)
	router.PUT("/api/admin/changes/:id", handler.AdminUpdateChange)
	router.POST("/api/admin/changes/:id/approve", handler.AdminApproveChange)
	router.POST("/api/admin/changes/:id/reject", handler.AdminRejectChange)
	router.POST("/api/admin/changes/:id/apply", handler.AdminApplyChange)
	router.DELETE("/api/admin/changes/:id", handler.AdminDeleteChange)
}

func registerAdminMonitorRoutes(router *gin.Engine, handler *Handler) {
	// 管理后台 — monitors.d/ CRUD API（需 Bearer token 鉴权）
	router.GET("/api/admin/templates", handler.AdminListTemplates)
	router.GET("/api/admin/monitors", handler.AdminListMonitors)
	router.GET("/api/admin/monitors/:key", handler.AdminGetMonitor)
	router.POST("/api/admin/monitors", handler.AdminCreateMonitor)
	router.PUT("/api/admin/monitors/:key", handler.AdminUpdateMonitor)
	router.DELETE("/api/admin/monitors/:key", handler.AdminDeleteMonitor)
	router.POST("/api/admin/monitors/:key/toggle", handler.AdminToggleMonitor)
	router.POST("/api/admin/monitors/:key/probe", handler.AdminProbeMonitor)
	router.GET("/api/admin/monitors/:key/logs", handler.AdminGetMonitorLogs)
}
