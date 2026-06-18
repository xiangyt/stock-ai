package model

import "encoding/json"

// ============================================================================
//  回测 — 卖出规则与仓位管理类型 (v1.1 可插拔架构)
//  持久化: strategies.exit_rules / strategies.position_rules (JSON)
// ============================================================================

// ExitRules 卖出规则集 (v1.1: 改为 rules[] 数组格式，支持可插拔)
type ExitRules struct {
	Rules       []ExitRuleConfig `json:"rules"`
	SlippagePct float64          `json:"slippage_pct"` // 滑点百分比，默认 0.3
}

// ExitRuleConfig 单条规则配置 (可插拔架构)
type ExitRuleConfig struct {
	Type     string          `json:"type"`                // "stop_loss" | "take_profit" | "time_exit" | ...
	Enabled  bool            `json:"enabled"`
	Params   json.RawMessage `json:"params"`              // 类型专属参数，反序列化时由工厂函数处理
	Priority *int            `json:"priority,omitempty"` // 可选，覆盖默认优先级
}

// UnmarshalJSON 兼容旧格式 (v1.0: stop_loss/take_profit/time_exit 独立字段)
func (e *ExitRules) UnmarshalJSON(data []byte) error {
	// 尝试新格式
	type newFormat ExitRules
	var nf newFormat
	if err := json.Unmarshal(data, &nf); err == nil && len(nf.Rules) > 0 {
		*e = ExitRules(nf)
		return nil
	}

	// 兼容旧格式: {"stop_loss": {...}, "take_profit": {...}, "time_exit": {...}, "slippage_pct": 0.3}
	var old struct {
		StopLoss    *oldStopLossRule  `json:"stop_loss"`
		TakeProfit  *oldTakeProfitRule `json:"take_profit"`
		TimeExit    *oldTimeExitRule  `json:"time_exit"`
		SlippagePct float64           `json:"slippage_pct"`
	}
	if err := json.Unmarshal(data, &old); err != nil {
		return err
	}

	e.SlippagePct = old.SlippagePct
	if e.SlippagePct == 0 {
		e.SlippagePct = 0.3
	}
	e.Rules = []ExitRuleConfig{}

	if old.StopLoss != nil && old.StopLoss.Enabled {
		p, _ := json.Marshal(map[string]float64{"threshold_pct": old.StopLoss.ThresholdPct})
		e.Rules = append(e.Rules, ExitRuleConfig{Type: "stop_loss", Enabled: true, Params: p})
	}
	if old.TakeProfit != nil && old.TakeProfit.Enabled {
		p, _ := json.Marshal(map[string]float64{"threshold_pct": old.TakeProfit.ThresholdPct})
		e.Rules = append(e.Rules, ExitRuleConfig{Type: "take_profit", Enabled: true, Params: p})
	}
	if old.TimeExit != nil && old.TimeExit.Enabled {
		p, _ := json.Marshal(map[string]int{"hold_days": old.TimeExit.HoldDays})
		e.Rules = append(e.Rules, ExitRuleConfig{Type: "time_exit", Enabled: true, Params: p})
	}
	return nil
}

// 旧格式结构体 (仅用于 UnmarshalJSON 兼容)
type oldStopLossRule struct {
	Enabled      bool    `json:"enabled"`
	ThresholdPct float64 `json:"threshold_pct"`
}
type oldTakeProfitRule struct {
	Enabled      bool    `json:"enabled"`
	ThresholdPct float64 `json:"threshold_pct"`
}
type oldTimeExitRule struct {
	Enabled  bool `json:"enabled"`
	HoldDays int  `json:"hold_days"`
}

// PositionRules 仓位管理规则 (v1.1: allocation 改为对象格式)
type PositionRules struct {
	MaxPositions  int              `json:"max_positions"`  // 0=不限制
	MaxSinglePct  float64          `json:"max_single_pct"` // 0=不限制
	Allocation    AllocationConfig `json:"allocation"`      // v1.1: 改为对象格式
	CashBufferPct float64          `json:"cash_buffer_pct"` // 现金缓冲比例, 默认 5
}

// AllocationConfig 仓位分配配置 (v1.1 可插拔架构)
type AllocationConfig struct {
	Type   string          `json:"type"`             // "equal" | "signal_weighted" | "volatility_weighted" | "risk_parity" | "custom_weight"
	Params json.RawMessage `json:"params,omitempty"` // 类型专属参数
}

// UnmarshalJSON 兼容旧格式 (v1.0: "allocation": "equal")
func (p *AllocationConfig) UnmarshalJSON(data []byte) error {
	// 尝试字符串格式 (旧格式)
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		p.Type = s
		p.Params = nil
		return nil
	}
	// 新格式: {"type": "equal", "params": {}}
	type newFormat AllocationConfig
	var nf newFormat
	return json.Unmarshal(data, &nf)
}

// MarshalJSON 序列化时，若 Params 为空则输出字符串格式 (保持 JSON 简洁)
func (p AllocationConfig) MarshalJSON() ([]byte, error) {
	if len(p.Params) == 0 {
		return json.Marshal(p.Type)
	}
	type alias AllocationConfig
	return json.Marshal((alias)(p))
}
