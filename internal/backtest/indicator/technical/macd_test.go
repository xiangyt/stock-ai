package technical

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"stock-ai/internal/backtest/indicator"
)

// macdTrendTestCfg 构造 MACD 趋势信号的配置。
// custom=true 时 SignalID 第 6 位为 '1'（自定义信号标记），否则走默认配置分支。
func macdTrendTestCfg(seq string, params map[string]any) *indicator.SignalConfig {
	return &indicator.SignalConfig{
		SignalID: "010031" + seq,
		Params:   params,
	}
}

func TestSignalMacdTrend(t *testing.T) {
	// oldest-first: 索引 0 最旧，索引 4 最新
	risingData := []float64{0.10, 0.20, 0.30, 0.40, 0.50}
	fallingData := []float64{0.50, 0.40, 0.30, 0.20, 0.10}
	brokenData := []float64{0.10, 0.20, 0.15, 0.40, 0.50}     // 索引2 回落
	flatData := []float64{0.10, 0.20, 0.20, 0.40, 0.50}       // 索引2 与前一日持平
	brokenDownData := []float64{0.50, 0.40, 0.45, 0.20, 0.10} // 索引2 反弹

	tests := []struct {
		name   string
		signal *SignalMacdTrend
		data   []float64
		start  float64
		end    float64
		want   indicator.EvaluatedResult
	}{
		{"上升-区间连续递增", newSignalMacdTrend("06", "区间上升", "", true, indicator.OpRising),
			risingData, 4, 0, indicator.ResultPassed},
		{"上升-区间内出现回落", newSignalMacdTrend("06", "区间上升", "", true, indicator.OpRising),
			brokenData, 4, 0, indicator.ResultRejected},
		{"上升-区间内出现持平", newSignalMacdTrend("06", "区间上升", "", true, indicator.OpRising),
			flatData, 4, 0, indicator.ResultRejected},
		{"上升-仅取最新两天且递增", newSignalMacdTrend("06", "区间上升", "", true, indicator.OpRising),
			risingData, 1, 0, indicator.ResultPassed},
		{"上升-窗口外的回落不影响判定", newSignalMacdTrend("06", "区间上升", "", true, indicator.OpRising),
			brokenData, 2, 0, indicator.ResultPassed},
		{"上升-窗口仅一天无法判断", newSignalMacdTrend("06", "区间上升", "", true, indicator.OpRising),
			risingData, 0, 0, indicator.ResultRejected},
		{"下降-区间连续递减", newSignalMacdTrend("07", "区间下降", "", false, indicator.OpFalling),
			fallingData, 4, 0, indicator.ResultPassed},
		{"下降-区间内出现反弹", newSignalMacdTrend("07", "区间下降", "", false, indicator.OpFalling),
			brokenDownData, 4, 0, indicator.ResultRejected},
		{"下降-对上升序列判定失败", newSignalMacdTrend("07", "区间下降", "", false, indicator.OpFalling),
			risingData, 4, 0, indicator.ResultRejected},
		{"start与end反序自动纠正", newSignalMacdTrend("06", "区间上升", "", true, indicator.OpRising),
			risingData, 0, 4, indicator.ResultPassed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := macdTrendTestCfg(tt.signal.Seq(), map[string]any{
				indicator.ParamKeyLookbackStart: tt.start,
				indicator.ParamKeyLookbackEnd:   tt.end,
			})
			got := tt.signal.Evaluate(MACDResult{MACD: tt.data}, cfg)
			assert.Equal(t, tt.want, got.Result)
			assert.NotEmpty(t, got.Message)
			assert.Equal(t, cfg.SignalID, got.SignalID)
		})
	}
}

// TestSignalMacdTrendDefaultConfig 校验非自定义配置走默认参数（5天前~今天）。
func TestSignalMacdTrendDefaultConfig(t *testing.T) {
	sig := newSignalMacdTrend("06", "区间上升", "", true, indicator.OpRising)
	cfg := &indicator.SignalConfig{Operator: indicator.OpRising}

	rising := []float64{0.10, 0.20, 0.30, 0.40, 0.50, 0.60}
	assert.Equal(t, indicator.ResultPassed, sig.Evaluate(MACDResult{MACD: rising}, cfg).Result)

	broken := []float64{0.10, 0.20, 0.30, 0.25, 0.40, 0.60}
	assert.Equal(t, indicator.ResultRejected, sig.Evaluate(MACDResult{MACD: broken}, cfg).Result)
}
