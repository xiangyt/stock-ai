package backtest

import (
	"context"
	"fmt"
	"testing"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/backtest/indicator/technical"
	"stock-ai/internal/model"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
//  测试原则：
//    1. 单信号单股票 — 手写 StockSnapshot，验证 MA 指标
//    2. 覆盖通过/不通过/边界情况
// ============================================================================

// makeBullishKL 构造多头排列 K 线：
// KLine[0] 最新，Close 从 10元 逐步上涨到 20元
func makeBullishKL(n int) []*model.DailyKline {
	kl := make([]*model.DailyKline, n)
	for i := 0; i < n; i++ {
		// 越新越高：i=0 是最新一天 = 最高价
		price := 2000 - i*5 // 分，最新≈2000分(20元)
		if price < 100 {
			price = 100
		}
		kl[i] = &model.DailyKline{Close: int(price)}
	}
	return kl
}

// newTestScreener 创建测试用 Screener（只含 MA 指标）
func newTestScreener() Screener {
	indicators := []indicator.Indicator{
		technical.NewMa(),
	}
	return NewScreener(indicators, 1)
}

// ============================================================================
//  单信号测试：多头排列
// ============================================================================

func TestScreener_MABullish_Pass(t *testing.T) {
	screener := newTestScreener()
	stock := &StockSnapshot{
		Code:  "000001",
		Name:  "平安银行",
		KLine: makeBullishKL(72), // MA 需要 69 根，+lookback_start=2
	}

	configs := []SignalConfig{
		{SignalID: "01001001", // 多头排列
			Params: map[string]any{
				"lookback_start": float64(2),
				"lookback_end":   float64(0),
			}},
	}

	result, err := screener.Screen(context.Background(), []*StockSnapshot{stock}, configs)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 1, result.Passed, "多头排列应通过: %s", result.Results[0].Reason)
}

func TestScreener_MABullish_Reject(t *testing.T) {
	screener := newTestScreener()
	// 构造空头排列：越新越低
	kl := make([]*model.DailyKline, 72)
	for i := 0; i < 72; i++ {
		price := 500 + i*5 // 越早越高，越新越低 → 空头
		kl[i] = &model.DailyKline{Close: int(price)}
	}
	stock := &StockSnapshot{Code: "000001", KLine: kl}

	configs := []SignalConfig{
		{SignalID: "01001001", // 多头排列
			Params: map[string]any{
				"lookback_start": float64(0),
				"lookback_end":   float64(0),
			}},
	}

	result, err := screener.Screen(context.Background(), []*StockSnapshot{stock}, configs)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Rejected, "空头排列应拒绝")
}

func TestScreener_MABullish_InsufficientData(t *testing.T) {
	screener := newTestScreener()
	stock := &StockSnapshot{
		Code:  "000001",
		KLine: makeBullishKL(5), // 不足 65 根
	}

	configs := []SignalConfig{
		{SignalID: "01001001"},
	}

	result, err := screener.Screen(context.Background(), []*StockSnapshot{stock}, configs)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Rejected, "数据不足应拒绝")
}

// ============================================================================
//  单信号测试：均线粘合
// ============================================================================

func TestScreener_MASticky(t *testing.T) {
	screener := newTestScreener()

	// 构造恒定价格：所有 K 线相同 → 均线完全粘合
	kl := make([]*model.DailyKline, 72)
	for i := 0; i < 72; i++ {
		kl[i] = &model.DailyKline{Close: int(1000)} // 10元
	}

	stock := &StockSnapshot{Code: "000001", KLine: kl}
	configs := []SignalConfig{
		{SignalID: "01001002", // 均线粘合
			Params: map[string]any{
				"lookback_start": float64(0),
				"lookback_end":   float64(0),
				"threshold":      float64(2.0),
			}},
	}

	result, err := screener.Screen(context.Background(), []*StockSnapshot{stock}, configs)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Passed, "恒定价格应粘合: %s", result.Results[0].Reason)
}

// ============================================================================
//  单信号测试：5日线金叉10日线
// ============================================================================

func TestScreener_MAGoldenCross(t *testing.T) {
	screener := newTestScreener()

	// 构造金叉：前一日 MA5<=MA10，今日 MA5>MA10
	// MA 指标需要至少 69 根
	kl := make([]*model.DailyKline, 72)
	for i := 0; i < 72; i++ {
		p := 1000
		if i <= 1 { // 最近 2 天大涨 → MA5 被拉起上穿 MA10
			p = 2000
		}
		kl[i] = &model.DailyKline{Close: int(p)}
	}

	stock := &StockSnapshot{Code: "000001", KLine: kl}
	configs := []SignalConfig{
		{SignalID: "01001004"}, // 5日线金叉10日线
	}

	result, err := screener.Screen(context.Background(), []*StockSnapshot{stock}, configs)
	assert.NoError(t, err)
	fmt.Printf("金叉测试: passed=%v results=%+v\n", result.Results[0].Passed, result.Results[0].Reason)
}

// ============================================================================
//  集成测试：多股票 + 多信号
// ============================================================================

func TestScreener_MultiStock(t *testing.T) {
	screener := newTestScreener()

	bullKL := makeBullishKL(72)
	stocks := []*StockSnapshot{
		{Code: "000001", Name: "多头股", KLine: bullKL},
		{Code: "000002", Name: "无数据股", KLine: make([]*model.DailyKline, 0)},
	}

	configs := []SignalConfig{
		{SignalID: "01001001"}, // 多头排列
	}

	result, err := screener.Screen(context.Background(), stocks, configs)
	assert.NoError(t, err)
	assert.Equal(t, 2, result.Total)
	assert.Equal(t, 1, result.Passed)   // 多头股通过
	assert.Equal(t, 1, result.Rejected) // 无数据股拒绝
}

// ============================================================================
//  基准测试
// ============================================================================

func BenchmarkScreener_MABullish(b *testing.B) {
	screener := newTestScreener()
	stock := &StockSnapshot{Code: "000001", KLine: makeBullishKL(72)}
	configs := []SignalConfig{{SignalID: "01001001"}}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		screener.Screen(ctx, []*StockSnapshot{stock}, configs)
	}
}
