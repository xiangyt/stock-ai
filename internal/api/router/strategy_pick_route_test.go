package router

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// TestStrategyPicksRouteConflict 校验 /strategies/:id 与 /strategies/:id/picks 可以共存注册
func TestStrategyPicksRouteConflict(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("路由注册冲突导致 panic: %v", r)
		}
	}()

	r := gin.New()
	g := r.Group("/api/v1/strategies")
	g.GET("/:id", func(c *gin.Context) {})
	g.GET("/:id/picks", func(c *gin.Context) {})

	var hasID, hasPicks bool
	for _, rt := range r.Routes() {
		if rt.Path == "/api/v1/strategies/:id" {
			hasID = true
		}
		if rt.Path == "/api/v1/strategies/:id/picks" {
			hasPicks = true
		}
	}
	if !hasID || !hasPicks {
		t.Fatalf("路由未正确注册: id=%v picks=%v", hasID, hasPicks)
	}
}
