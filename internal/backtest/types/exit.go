package types

import (
	"encoding/json"
	"fmt"

	"stock-ai/internal/model"
)

// ============================================================================
//  卖出规则可插拔架构 — 核心接口定义 (v1.1)
// ============================================================================

// DayBar 日K线数据（回测引擎使用的简化结构）
type DayBar struct {
	Date  string
	Open  float64
	High  float64
	Low   float64
	Close float64
}

// ExitChecker 卖出规则核心接口 — 只需实现 Check + Name 即可接入引擎
type ExitChecker interface {
	Check(pos *HoldingPosition, bar DayBar) *ExitDecision
	Name() string
}

// PriorityChecker 可选接口 — 需要控制执行顺序的 checker 实现
type PriorityChecker interface {
	Priority() int
}

// EntryHook 可选接口 — 需要建仓时初始化状态的 checker 实现
type EntryHook interface {
	OnEntry(pos *HoldingPosition, entryPrice float64, entryDate string)
}

// DailyUpdateHook 可选接口 — 需要每日更新状态的 checker 实现
type DailyUpdateHook interface {
	OnDailyUpdate(pos *HoldingPosition, bar DayBar)
}

// ExitDecision 卖出判定结果
type ExitDecision struct {
	Reason   string
	Price    float64
	Quantity int // 0 = 整仓卖出; >0 = 部分卖出
}

// ============================================================================
//  HoldingPosition 回测持仓（内存对象）
// ============================================================================

type HoldingPosition struct {
	StockCode  string
	Quantity   int
	EntryPrice float64
	EntryDate  string
	HoldDays   int
	State      map[string]any
}

func (p *HoldingPosition) SetState(key string, val any) {
	if p.State == nil {
		p.State = make(map[string]any)
	}
	p.State[key] = val
}

func (p *HoldingPosition) GetState(key string) any {
	if p.State == nil {
		return nil
	}
	return p.State[key]
}

// ============================================================================
//  卖出规则注册表 + 工厂模式
// ============================================================================

// ExitCheckerFactory 从 JSON params 创建 checker
type ExitCheckerFactory func(params json.RawMessage) (ExitChecker, error)

var exitCheckerFactories = map[string]ExitCheckerFactory{}

// RegisterExitChecker 注册卖出规则工厂（通常在 init() 中调用）
func RegisterExitChecker(ruleType string, factory ExitCheckerFactory) {
	exitCheckerFactories[ruleType] = factory
}

// SlippageInjector 可选接口 — 需要滑点配置的 checker 实现
type SlippageInjector interface {
	SetSlippage(slippagePct float64)
}

// Prioritizer 可选接口 — 用于子包 checker 实现动态设置优先级
type Prioritizer interface {
	SetPriority(p int)
}

// BuildExitChain 从配置构建执行链（按优先级排序）
func BuildExitChain(configs []model.ExitRuleConfig, slippagePct float64) ([]ExitChecker, error) {
	chain := []ExitChecker{}
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		factory, ok := exitCheckerFactories[cfg.Type]
		if !ok {
			return nil, fmt.Errorf("unknown exit rule type: %s", cfg.Type)
		}
		checker, err := factory(cfg.Params)
		if err != nil {
			return nil, fmt.Errorf("failed to create checker %s: %w", cfg.Type, err)
		}
		if si, ok := checker.(SlippageInjector); ok && slippagePct > 0 {
			si.SetSlippage(slippagePct)
		}
		if cfg.Priority != nil {
			if pc, ok := checker.(Prioritizer); ok {
				pc.SetPriority(*cfg.Priority)
			}
		}
		chain = append(chain, checker)
	}
	return GetSortedChain(chain), nil
}

// GetSortedChain 按优先级排序（PriorityChecker 未实现 → 默认 100）
func GetSortedChain(chain []ExitChecker) []ExitChecker {
	sorted := make([]ExitChecker, len(chain))
	copy(sorted, chain)
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			pj := GetPriority(sorted[j])
			pk := GetPriority(sorted[j+1])
			if pj > pk {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	return sorted
}

// GetPriority 获取 checker 优先级
func GetPriority(c ExitChecker) int {
	if pc, ok := c.(PriorityChecker); ok {
		return pc.Priority()
	}
	return 100
}
