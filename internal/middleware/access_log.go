package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	applog "stock-ai/internal/log"
)

// GinLogger 用 slog 记录 HTTP 访问日志的结构化中间件，替代 gin.Logger()。
//
// 输出字段包含 trace_id（依赖 TraceID 中间件先注册）、method、path、status、
// 耗时、客户端 IP 与响应大小；按状态码自动分级（5xx Error / 4xx Warn / 其余 Info）。
func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		if raw := c.Request.URL.RawQuery; raw != "" {
			path = path + "?" + raw
		}

		c.Next()

		status := c.Writer.Status()
		elapsed := time.Since(start)
		slog.Log(c.Request.Context(), levelForStatus(status), "HTTP",
			"trace_id", applog.TraceIDFromContext(c.Request.Context()),
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"elapsed_ms", elapsed.Milliseconds(),
			"client", c.ClientIP(),
			"size", c.Writer.Size(),
		)
	}
}

// levelForStatus 根据响应状态码选择日志级别。
func levelForStatus(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}
