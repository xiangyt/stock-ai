package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	applog "stock-ai/internal/log"
)

// TraceIDHeader 是 trace_id 在 HTTP 头中的字段名
const TraceIDHeader = "X-Trace-ID"

// TraceID 生成并透传 trace_id 的中间件。
//
// 优先从请求头 X-Trace-ID 获取（便于上游服务串联），否则生成新的 UUID。
// trace_id 会写入 request context（供 GORM Logger 等下游读取）、
// gin context（供 handler 取用）以及响应头（便于客户端关联）。
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(TraceIDHeader)
		if traceID == "" {
			traceID = uuid.NewString()
		}

		// 写入 request context，供 GORM Logger 等下游读取
		c.Request = c.Request.WithContext(applog.WithTraceID(c.Request.Context(), traceID))
		// 存入 gin context，方便 handler 取用
		c.Set("trace_id", traceID)
		// 回写响应头，便于客户端关联
		c.Header(TraceIDHeader, traceID)

		c.Next()
	}
}
