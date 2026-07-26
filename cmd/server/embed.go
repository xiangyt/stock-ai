package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed dist/*
var embeddedStatic embed.FS

// serveStatic 配置静态文件服务。
//
// staticDir: 开发模式下指定磁盘目录（如 web/dist），为空则使用内嵌资源。
func serveStatic(r *gin.Engine, staticDir string) {
	if staticDir != "" {
		log.Printf("使用磁盘静态文件: %s", staticDir)
		setupStaticFromDisk(r, staticDir)
	} else {
		log.Println("使用内嵌静态文件")
		setupStaticFromEmbed(r)
	}
}

// setupStaticFromEmbed 从内嵌的 embed.FS 提供静态文件服务。
func setupStaticFromEmbed(r *gin.Engine) {
	subFS, err := fs.Sub(embeddedStatic, "dist")
	if err != nil {
		log.Printf("内嵌静态文件加载失败: %v（前端页面不可用）", err)
		return
	}

	fileServer := http.FileServer(http.FS(subFS))

	// 所有非 API 路由通过 NoRoute 统一处理：静态资源 → SPA fallback
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// API / health 路由返回 JSON 404
		if strings.HasPrefix(path, "/api/") || path == "/health" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// 静态资源：由 http.FileServer 处理，自动设置正确的 MIME 类型
		if strings.HasPrefix(path, "/assets/") || path == "/favicon.svg" || path == "/icons.svg" {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// SPA fallback：其他路径返回 index.html
		data, err := fs.ReadFile(subFS, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "index.html not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
}

// setupStaticFromDisk 从磁盘目录提供静态文件服务（开发模式）。
func setupStaticFromDisk(r *gin.Engine, dir string) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		log.Printf("解析静态文件目录失败: %v（前端页面不可用）", err)
		return
	}

	if _, err := os.Stat(absDir); os.IsNotExist(err) {
		log.Printf("静态文件目录不存在: %s（前端页面不可用）", absDir)
		return
	}

	// 静态资源
	r.Static("/assets", filepath.Join(absDir, "assets"))
	r.StaticFile("/favicon.svg", filepath.Join(absDir, "favicon.svg"))
	r.StaticFile("/icons.svg", filepath.Join(absDir, "icons.svg"))

	// SPA fallback: 所有非 API 路由返回 index.html
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/health") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.File(filepath.Join(absDir, "index.html"))
	})
}
