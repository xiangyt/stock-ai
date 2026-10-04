package service

import (
	"testing"

	"stock-ai/internal/backtest/indicator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReturnPct(t *testing.T) {
	tests := []struct {
		name       string
		priceCents int
		baseCents  int
		want       float64
	}{
		{"上涨10%", 1100, 1000, 10},
		{"下跌5%", 950, 1000, -5},
		{"持平", 1000, 1000, 0},
		{"基准为0返回0", 1000, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.InDelta(t, tt.want, returnPct(tt.priceCents, tt.baseCents), 1e-9)
		})
	}
}

func TestCentsToYuan(t *testing.T) {
	assert.InDelta(t, 12.34, centsToYuan(1234), 1e-9)
	assert.InDelta(t, 0, centsToYuan(0), 1e-9)
}

func TestExtremes(t *testing.T) {
	maxPct, minPct := extremes(nil)
	assert.Equal(t, 0.0, maxPct)
	assert.Equal(t, 0.0, minPct)

	items := []ForwardDay{
		{Offset: 1, HighPct: 2.5, LowPct: -1.0},
		{Offset: 2, HighPct: 6.1, LowPct: -3.2},
		{Offset: 3, HighPct: 1.2, LowPct: 0.5},
	}
	maxPct, minPct = extremes(items)
	assert.InDelta(t, 6.1, maxPct, 1e-9)
	assert.InDelta(t, -3.2, minPct, 1e-9)
}

func TestFilterPassedStocks(t *testing.T) {
	results := []*indicator.EvaluatedStock{
		{Code: "000002", Result: indicator.ResultPassed},
		{Code: "000001", Result: indicator.ResultRejected},
		nil,
		{Code: "600000", Result: indicator.ResultPassed},
	}

	passed := filterPassedStocks(results)
	require.Len(t, passed, 2)
	assert.Equal(t, "000002", passed[0].Code)
	assert.Equal(t, "600000", passed[1].Code)
}

func TestFilterTradingDates(t *testing.T) {
	// 2026-01-03 周六、2026-01-04 周日 → 跳过；2026-01-05 周一 → 保留
	got := filterTradingDates([]int{20260103, 20260104, 20260105})
	assert.Equal(t, []int{20260105}, got)

	assert.Empty(t, filterTradingDates(nil))
	assert.Empty(t, filterTradingDates([]int{}))
}

func TestParseStrategyConditions(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantCount int
		wantErr   bool
	}{
		{"正常解析", `[{"signal_id":"0100101","operator":"lt","params":{"threshold":30}}]`, 1, false},
		{"跳过空信号", `[{"signal_id":"","operator":"lt"},{"signal_id":"0100102"}]`, 1, false},
		{"JSON非法", `{bad`, 0, true},
		{"空条件", ``, 0, true},
		{"全部无效", `[{"signal_id":""}]`, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs, err := parseStrategyConditions(tt.raw)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, configs, tt.wantCount)
		})
	}
}
