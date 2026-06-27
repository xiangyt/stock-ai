package indicator

import (
	"sync"
)

// ============================================================================
//  全局单例 — 指标引擎和注册中心
// ============================================================================

var (
	globalRegistry     *Registry
	globalRegistryOnce sync.Once
)

// InitGlobal 初始化全局唯一的指标注册中心和引擎。
//
// 在 main.go 或服务启动时调用一次。
// 注册所有内置指标（技术面/行情面/基本/财务面）。
//
//   indicator.InitGlobal(allIndicators)
//   reg := indicator.GlobalRegistry()
//   engine := reg.Engine()
func InitGlobal(indicators []Indicator) {
	globalRegistryOnce.Do(func() {
		globalRegistry = NewRegistry(indicators)
	})
}

// GlobalRegistry 返回全局唯一的注册中心。
//
// 若未调用 InitGlobal 则返回 nil。
// 使用方需自行判空或确保在服务启动时已初始化。
func GlobalRegistry() *Registry {
	return globalRegistry
}

// GlobalEngine 返回全局唯一的选股引擎。
//
// 快捷方法，等同于 GlobalRegistry().Engine()。
func GlobalEngine() *Engine {
	if globalRegistry == nil {
		return nil
	}
	return globalRegistry.Engine()
}
