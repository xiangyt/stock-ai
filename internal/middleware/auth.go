package middleware

import (
	"net/http"
	"strings"

	"stock-ai/internal/api/handler"
	"stock-ai/internal/service"

	"github.com/gin-gonic/gin"
)

// AuthRequired JWT 认证中间件
// 从 Authorization: Bearer <token> 中提取并验证 token
// 验证通过后将 user_id 和 username 注入上下文
func AuthRequired(authSvc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未提供认证令牌"})
			return
		}

		claims, err := authSvc.ParseToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "认证已过期，请重新登录"})
			return
		}

		// 将用户信息注入上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)

		c.Next()
	}
}

// OptionalAuth 可选认证中间件（有 token 就解析，没有也放行）
func OptionalAuth(authSvc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr != "" {
			claims, err := authSvc.ParseToken(tokenStr)
			if err == nil {
				c.Set("user_id", claims.UserID)
				c.Set("username", claims.Username)
			}
		}
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	bearer := c.GetHeader("Authorization")
	if strings.HasPrefix(bearer, "Bearer ") {
		return strings.TrimPrefix(bearer, "Bearer ")
	}

	// 也支持 query 参数（用于调试）
	if t := c.Query("token"); t != "" {
		return t
	}
	return handler.ExtractToken(c)
}
