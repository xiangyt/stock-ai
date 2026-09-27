package mcp

import (
	"testing"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStrategy 构造测试用策略
func newTestStrategy(id uint, name string) model.Strategy {
	return model.Strategy{
		ID:   id,
		Name: name,
	}
}

func TestBuildStrategyBriefs(t *testing.T) {
	list := []model.Strategy{
		newTestStrategy(1, "均线多头"),
		newTestStrategy(2, "放量突破"),
	}

	items := buildStrategyBriefs(list, map[uint]bool{1: true})

	assert.Len(t, items, 2)
	assert.Equal(t, strategyBrief{ID: 1, Name: "均线多头", Using: true}, items[0],
		"命中 usingSet 的策略应标记为使用中")
	assert.Equal(t, strategyBrief{ID: 2, Name: "放量突破", Using: false}, items[1],
		"未命中的策略 Using 应为 false")
}

func TestBuildStrategyBriefs_Empty(t *testing.T) {
	items := buildStrategyBriefs(nil, nil)

	assert.NotNil(t, items, "空列表应返回非 nil 切片，序列化为 []")
	assert.Empty(t, items)
}

func TestToScreenStocks(t *testing.T) {
	evaluated := []*indicator.EvaluatedStock{
		{Code: "600519", Name: "贵州茅台", Price: 1688.005, Result: indicator.ResultPassed},
		{Code: "000001", Name: "平安银行", Price: 11.5, Result: indicator.ResultRejected},
		{Code: "300750", Name: "宁德时代", Price: 180.0, Result: indicator.ResultPassed},
	}

	items := toScreenStocks(evaluated)

	require.Len(t, items, 2, "仅保留通过评估的股票")
	assert.Equal(t, "300750", items[0].Code, "结果应按股票代码升序排列")
	assert.Equal(t, "600519", items[1].Code)
	assert.Equal(t, 1688.01, items[1].Price, "价格应四舍五入保留2位")
}

func TestToScreenStocks_Empty(t *testing.T) {
	items := toScreenStocks(nil)

	assert.NotNil(t, items)
	assert.Empty(t, items)
}

func TestParseSignalConfigs(t *testing.T) {
	conditions := `[{"signal_id":"0000101","operator":"cross_up","params":{"period":5}}]`

	configs, err := parseSignalConfigs(conditions)
	require.NoError(t, err)
	require.Len(t, configs, 1)
	assert.Equal(t, "0000101", configs[0].SignalID)
}

func TestParseSignalConfigs_Invalid(t *testing.T) {
	tests := []struct {
		name      string
		condition string
	}{
		{"空条件", ""},
		{"JSON 非法", `{bad json`},
		{"无信号", `[]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs, err := parseSignalConfigs(tt.condition)
			assert.Error(t, err)
			assert.Nil(t, configs)
		})
	}
}

func TestNormalizeScreenDate(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"缺省取最近交易日", "", "", false},
		{"横线格式", "2026-09-27", "2026-09-27", false},
		{"纯数字格式", "20260927", "2026-09-27", false},
		{"日期非法", "2026-13-45", "", true},
		{"长度不足", "2026092", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeScreenDate(tt.raw)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
