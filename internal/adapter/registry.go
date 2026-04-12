package adapter

import (
	"fmt"
	"sync"
)

// Registry 数据源注册中心（单例）
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]DataSource
}

var (
	globalRegistry *Registry
	once           sync.Once
)

// GetRegistry 获取全局注册中心（单例）
func GetRegistry() *Registry {
	once.Do(func() {
		globalRegistry = &Registry{
			adapters: make(map[string]DataSource),
		}
	})
	return globalRegistry
}

// Register 注册数据源
func (r *Registry) Register(ds DataSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := ds.Name()
	if _, exists := r.adapters[name]; exists {
		return fmt.Errorf("数据源已存在: %s", name)
	}

	if err := ds.Init(nil); err != nil {
		return fmt.Errorf("初始化数据源 %s 失败: %w", name, err)
	}

	r.adapters[name] = ds
	fmt.Printf("✅ 已注册数据源: %s (%s)\n", name, ds.DisplayName())
	return nil
}

// Get 获取指定数据源
func (r *Registry) Get(name string) (DataSource, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[name]
	return a, ok
}

// List 列出所有已注册数据源
func (r *Registry) List() []DataSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]DataSource, 0, len(r.adapters))
	for _, a := range r.adapters {
		list = append(list, a)
	}
	return list
}

// Names 列出所有已注册数据源名称
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	return names
}

// Un 注销并关闭数据源
func (r *Registry) Un(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ds, exists := r.adapters[name]
	if !exists {
		return fmt.Errorf("数据源不存在: %s", name)
	}

	ds.Close()
	delete(r.adapters, name)
	return nil
}

// CloseAll 关闭所有数据源连接
func (r *Registry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, a := range r.adapters {
		if err := a.Close(); err != nil {
			fmt.Printf("⚠️ 关闭数据源 %s 失败: %v\n", name, err)
		}
	}
	r.adapters = make(map[string]DataSource)
}

// InitAll 初始化所有已注册的数据源（测试连接）
func (r *Registry) InitAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, a := range r.adapters {
		if err := a.TestConnection(nil); err != nil {
			fmt.Printf("⚠️ 数据源 %s 连接失败: %v (将继续使用)\n", name, err)
		} else {
			fmt.Printf("✅ 数据源 %s 连接正常\n", name)
		}
	}
	return nil
}

// RegisterDefaults 注册默认数据源（东方财富 + 同花顺）
// 注意：此函数需要在 cmd/server/main.go 等入口处调用
// 调用方需 import 对应的 adapter 子包
func RegisterDefaults(eastmoneyAdapter, thsAdapter DataSource) error {
	r := GetRegistry()

	if err := r.Register(eastmoneyAdapter); err != nil {
		return fmt.Errorf("注册东方财富失败: %w", err)
	}

	if err := r.Register(thsAdapter); err != nil {
		return fmt.Errorf("注册同花顺失败: %w", err)
	}

	return nil
}
