package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"monitor/internal/logger"
	"monitor/internal/storage"
)

func registerHealthRoutes(router *gin.Engine, store storage.Storage) {
	// 健康检查（支持 GET 和 HEAD）— 存活探针（liveness）
	healthHandler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	router.GET("/health", healthHandler)
	router.HEAD("/health", healthHandler)

	// 就绪探针（readiness）— 检查存储连通性
	readyHandler := func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		if err := store.WithContext(ctx).Ping(); err != nil {
			logger.Warn("api", "readiness check failed", "error", err)
			apiError(c, http.StatusServiceUnavailable, ErrCodeServiceUnavailable, "storage not ready")
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	router.GET("/ready", readyHandler)
	router.HEAD("/ready", readyHandler)
}
