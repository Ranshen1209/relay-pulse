package api

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"monitor/internal/logger"
)

//go:embed frontend/dist
var frontendFS embed.FS

// setupStaticFiles 设置静态文件服务（前端）
func setupStaticFiles(router *gin.Engine, handler *Handler) {
	// 获取嵌入的前端文件系统
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		logger.Warn("api", "无法加载前端文件系统", "error", err)
		return
	}

	// 获取 assets 子目录文件系统
	// StaticFS("/assets", ...) 会将 /assets/file.js 映射到文件系统根目录的 file.js
	// 所以需要创建一个子文件系统指向 assets 目录
	assetsFS, err := fs.Sub(distFS, "assets")
	if err != nil {
		logger.Warn("api", "无法加载 assets 文件系统", "error", err)
		return
	}

	// 静态资源路径（CSS、JS等）
	router.StaticFS("/assets", http.FS(assetsFS))

	// vite.svg 等根目录静态文件
	router.GET("/vite.svg", func(c *gin.Context) {
		data, err := fs.ReadFile(distFS, "vite.svg")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "image/svg+xml", data)
	})

	// SPA 路由回退 - 所有未匹配的路由返回 index.html
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// API 路径返回 404
		if strings.HasPrefix(path, "/api/") {
			apiError(c, http.StatusNotFound, ErrCodeNotFound, "API 接口不存在")
			return
		}

		// 静态资源缺失直接返回 404，避免 SPA 回退导致 MIME 类型错误
		// 当 /assets/ 下的文件不存在时，StaticFS 不处理，请求会落入 NoRoute
		// 如果回退到 index.html，浏览器会因为 MIME 类型是 text/html 而报错
		if strings.HasPrefix(path, "/assets/") {
			c.Status(http.StatusNotFound)
			return
		}

		// 尝试从 embed FS 读取静态文件（favicon.svg、manifest.json 等）
		// 移除所有前导斜杠（Nginx 代理可能产生 //favicon.svg）
		filePath := strings.TrimLeft(path, "/")
		filePath = filepath.Clean(filePath)

		// 空路径或 "." 返回 index.html
		if filePath == "." || filePath == "" {
			filePath = "index.html"
		}

		// 防止路径穿越攻击
		if strings.Contains(filePath, "..") {
			logger.Warn("api", "路径穿越尝试", "path", path)
			c.Status(http.StatusBadRequest)
			return
		}

		// 尝试打开文件
		if file, err := distFS.Open(filePath); err == nil {
			defer file.Close()
			info, _ := file.Stat()

			// 根据文件扩展名确定 MIME 类型
			mimeType := mime.TypeByExtension(filepath.Ext(filePath))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}

			// 特殊处理: index.html 需要走 Meta 注入逻辑，不直接返回
			if filePath == "index.html" {
				// 不直接返回，让它进入后面的 Meta 注入逻辑
			} else {
				c.DataFromReader(http.StatusOK, info.Size(), mimeType, file, nil)
				return
			}
		}

		// 文件不存在，回退到 index.html（SPA 路由）
		data, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load frontend")
			return
		}

		// 动态注入 Meta 标签（SEO 优化）
		handler.cfgMu.RLock()
		cfg := handler.config
		handler.cfgMu.RUnlock()

		html, isNotFound := injectMetaTags(string(data), path, cfg)

		// 如果是 404（provider 不存在），返回 404 状态码
		if isNotFound {
			c.Data(http.StatusNotFound, "text/html; charset=utf-8", []byte(html))
		} else {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
		}
	})
}
