//go:build integration
// +build integration

package backtest

import (
	"context"
	"os"
	"testing"

	"stock-ai/internal/config"
	"stock-ai/internal/db"
	"stock-ai/internal/backtest/indicator"
	fundamental "stock-ai/internal/backtest/indicator/fundamental"
	stocksource "stock-ai/internal/backtest/indicator/stocksource"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
//  集成测试：真实 DB + 简单指标验证引擎链路
//
//  运行方式:
//    go test -tags=integration ./internal/screening/ -v -run Integration -count=1
//  要求:
//    - 项目根目录存在 config.yaml
//    - MySQL 可连接且有 stock_daily_snapshot 数据
// ============================================================================

// loadTestConfig 从项目根目录 config.yaml 加载配置。
func loadTestConfig(t *testing.T) *config.Config {
	t.Helper()

	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "../../config.yaml"
	}

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err, "加载配置文件失败: %s", cfgPath)
	return cfg
}

// newIntegrationScreener 创建带总市值指标的 Screener。
func newIntegrationScreener(t *testing.T) Screener {
	t.Helper()
	return NewScreener([]indicator.Indicator{
		fundamental.NewTotalMarketCap(),
	}, 10)
}

// fetchRealStocks 从 DB 加载股票，并包装为 indicator.StockSource。
func fetchRealStocks(t *testing.T, limit int) []indicator.StockSource {
	t.Helper()

	stocks := db.LoadAllStockCodes()
	require.NotEmpty(t, stocks, "数据库中无股票数据")
	if len(stocks) > limit {
		stocks = stocks[:limit]
	}

	// 用当前日期（或最近交易日）作为评估日期
	tradeDate := 20260101 // 简单固定日期，实际应根据最近交易日决定

	sources := make([]indicator.StockSource, 0, len(stocks))
	for i := range stocks {
		src := stocksource.NewDBStock(&stocks[i], tradeDate)
		sources = append(sources, src)
	}
	return sources
}

// ============================================================================
//  TestIntegration_TotalMarketCap — 总市值 > 1000 亿筛选
// ============================================================================

func TestIntegration_TotalMarketCap(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（-short）")
	}

	cfg := loadTestConfig(t)
	require.NoError(t, db.Init(&cfg.Database))

	// 用真实 StockSource（DB 懒加载）直接调 indicator.Engine
	engine := indicator.NewEngine([]indicator.Indicator{
		fundamental.NewTotalMarketCap(),
	})
	sources := fetchRealStocks(t, 10)

	configs := []*indicator.SignalConfig{
		{
			SignalID: "03004004",        // 总市值 > 1000 亿
			Operator: indicator.OpGT,
			Params:   map[string]any{"min": float64(1000)},
		},
	}

	results := engine.Execute(sources, configs, 1)

	// 验证：至少返回了结果
	assert.Len(t, results, len(sources))

	passed := 0
	for _, r := range results {
		if r.Result == indicator.ResultPassed {
			passed++
			t.Logf("  ✅ %s: 总市值 > 1000 亿", r.Code)
		} else {
			t.Logf("  ❌ %s: %s", r.Code, r.Message)
		}
	}
	t.Logf("总市值>1000亿: %d/%d 通过", passed, len(results))
}

// ============================================================================
//  TestIntegration_TotalMarketCap_SmallCap — 小市值筛选
// ============================================================================

func TestIntegration_TotalMarketCap_SmallCap(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（-short）")
	}

	cfg := loadTestConfig(t)
	require.NoError(t, db.Init(&cfg.Database))

	engine := indicator.NewEngine([]indicator.Indicator{
		fundamental.NewTotalMarketCap(),
	})
	sources := fetchRealStocks(t, 20)

	// 筛选 50~200 亿（小盘）
	configs := []*indicator.SignalConfig{
		{
			SignalID: "03004002",
			Operator: indicator.OpBetween,
			Params:   map[string]any{"min": 50.0, "max": 200.0},
		},
	}

	results := engine.Execute(sources, configs, 1)

	passed := 0
	for _, r := range results {
		if r.Result == indicator.ResultPassed {
			passed++
			t.Logf("  ✅ %s: 50~200亿", r.Code)
		} else {
			t.Logf("  ❌ %s: %s", r.Code, r.Message)
		}
	}
	t.Logf("50~200亿: %d/%d 通过", passed, len(results))
}

// ============================================================================
//  TestIntegration_Screener_WithRealData — Screener 链路验证
//
//  通过 Screener 接口 + StockSnapshot 验证整体链路。
//  注意：当前 Screener 适配器不提供 StockDailySnapshot，
//  含快照数据的指标（如总市值）会返回 Rejected。
//  后续扩展 snapshotSource 后可打开此测试。
// ============================================================================

func TestIntegration_Screener_WithRealData(t *testing.T) {
	t.Skip("待 snapshotSource 支持 StockDailySnapshot 后启用")

	cfg := loadTestConfig(t)
	require.NoError(t, db.Init(&cfg.Database))

	stocks := db.LoadAllStockCodes()
	require.NotEmpty(t, stocks)

	snapshots := make([]*StockSnapshot, 0, 5)
	for i := 0; i < min(5, len(stocks)); i++ {
		snapshots = append(snapshots, &StockSnapshot{
			Code: stocks[i].Code,
			Name: stocks[i].Name,
		})
	}

	screener := newIntegrationScreener(t)
	configs := []SignalConfig{
		{SignalID: "03004004", Operator: "gt"},
	}

	result, err := screener.Screen(context.Background(), snapshots, configs)
	require.NoError(t, err)
	t.Logf("Screener 链路: passed=%d/%d", result.Passed, result.Total)
}

// ============================================================================
//  TestIntegration_Engine_Run — 完整回测链路
// ============================================================================

func TestIntegration_Engine_Run(t *testing.T) {
	t.Skip("Engine.Run 需要完整的 Screener + ExitExecutor + DataProvider 注入后启用")

	cfg := loadTestConfig(t)
	require.NoError(t, db.Init(&cfg.Database))

	engine := NewEngine(
		WithScreener(newIntegrationScreener(t)),
		WithFeeCalculator(NewAShareFeeCalculator(2.5, true)),
	)

	result, err := engine.Run(context.Background(), RunRequest{
		StockPool:      []string{"000001", "600519"},
		StartDate:      "2025-01-02",
		EndDate:        "2025-01-10",
		InitialCapital: 100000,
		EntryConfigs: []SignalConfig{
			{SignalID: "03004004"},
		},
		PositionRules: PositionRules{MaxPositions: 5},
	})
	require.NoError(t, err)
	t.Logf("回测: 收益率=%.2f%% 交易数=%d", result.TotalReturn, result.TradeCount)
}
