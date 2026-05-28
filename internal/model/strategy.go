package model

import (
	"encoding/json"
)

// StrategySignal 策略中的单个信号条件（对应前端的 Sig 结构体）
type StrategySignal struct {
	UID       int                    `json:"uid"`
	SignalID  string                 `json:"signal_id"` // 8位信号ID
	Name      string                 `json:"name"`
	Category  string                 `json:"category"` // technical/fundamental/market/financial
	Operator  string                 `json:"operator"` // < > = between cross_up 等
	OpSym     string                 `json:"opSym"`    // 运算符符号 < > ↑ ↗ ∈
	OpLbl     string                 `json:"opLbl"`    // 运算符中文：小于/大于/金叉等
	Params    map[string]interface{} `json:"params"`   // 参数键值对
	ParamText string                `json:"paramText"` // 参数文本描述
}

// StrategyDetail 策略详情响应（包含解析后的 conditions 数组）
// Strategy struct 本体定义在 indicator.go 中，此处仅定义 API 响应结构和方法
type StrategyDetail struct {
	ID                uint             `json:"id"`
	UID               uint             `json:"uid"`
	Name              string           `json:"name"`
	LogicalOp         string           `json:"logical_op"`
	Signals           []StrategySignal `json:"signals"`
	Description       string           `json:"description"`
	BacktestCount     int              `json:"backtest_count"`
	SubscriptionCount int64            `json:"subscription_count"`          // 订阅数量
	LastRunAt         *string          `json:"last_run_at,omitempty"`       // 最后运行时间
	IsPublic          bool             `json:"is_public"`                   // 是否公开
	CreatedAt         string           `json:"created_at"`
	UpdatedAt         string           `json:"updated_at"`
}

// ToDetail 将 Model 转换为 Detail 响应（解析 JSON conditions）
func (s *Strategy) ToDetail() (*StrategyDetail, error) {
	var signals []StrategySignal
	if s.Conditions != "" {
		if err := json.Unmarshal([]byte(s.Conditions), &signals); err != nil {
			return nil, err
		}
	}

	var lastRunAt *string
	if s.LastRunAt != nil {
		v := s.LastRunAt.Format("2006-01-02T15:04:05.000Z")
		lastRunAt = &v
	}

	return &StrategyDetail{
		ID:            s.ID,
		UID:           s.UID,
		Name:          s.Name,
		LogicalOp:     string(s.LogicalOp),
		Signals:       signals,
		Description:   s.Description,
		BacktestCount: s.BacktestCount,
		LastRunAt:     lastRunAt,
		IsPublic:      s.IsPublic,
		CreatedAt:     s.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:     s.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	}, nil
}

// SetConditions 设置条件 JSON（将 signals 数组序列化为字符串）
func (s *Strategy) SetConditions(signals []StrategySignal) error {
	data, err := json.Marshal(signals)
	if err != nil {
		return err
	}
	s.Conditions = string(data)
	return nil
}
