package router

import (
	"stock-ai/internal/api/handler"

	"github.com/gin-gonic/gin"
)

// RegisterKLineRoutes 注册 K 线查询路由
func RegisterKLineRoutes(apiV1 *gin.RouterGroup) {
	h := handler.NewKLineQueryHandler()
	kline := apiV1.Group("/kline")
	{
		kline.GET("/:code", h.GetKLine)
	}
}
