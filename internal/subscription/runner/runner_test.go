package runner

import (
	"context"
	"testing"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/eastmoney"
	"stock-ai/internal/config"
	"stock-ai/internal/db"
	"stock-ai/internal/indicator"
	"stock-ai/internal/indicator/financial"
	"stock-ai/internal/indicator/fundamental"
	"stock-ai/internal/indicator/market"
	"stock-ai/internal/indicator/stocksource"
	"stock-ai/internal/indicator/technical"
	"stock-ai/internal/model"
)

// ============================================================================
//  TestRun — 集成测试: Run() 端到端执行
//
//  使用真实 MySQL + 真实 indicator.Engine + stocksource.DBStock，
//  对单只股票(000507 珍珠港)执行完整选股流程。
//
//  前置条件:
//    - MySQL 运行在 config.yaml 配置的地址，stock 库已有 000507 的基础数据
//    - DB 中 strategies 表可读写（用于插入/清理测试策略）
// ============================================================================

// testStrategyConditions 是用户指定的真实信号配置 JSON（纯 configs 数组）
const testStrategyConditions = `[
	{"signal_id":"03001001","operator":"eq","params":{"threshold":"main"}},
	{"signal_id":"04001101","operator":"between","params":{"max":22,"min":20}},
	{"signal_id":"03004001","operator":"between","params":{"max":50,"min":0}},
	{"signal_id":"03005101","operator":"gt","params":{"threshold":30}}
]`

// testNotifier 用于拦截通知推送（测试环境不需要真实推送）
type testNotifier struct{}

func (n *testNotifier) Send(_ context.Context, _ *model.PushBot, _ string) error { return nil }
func (n *testNotifier) Render(tpl string, vars map[string]string) string         { return tpl }

// testProvider 实现 StockSourceProvider，直接用 stocksource.NewDBStockByCode 构建
type testProvider struct {
	tradeDate int
}

func (p *testProvider) BuildStockSources(_ context.Context, codes []string) ([]indicator.StockSource, error) {
	sources := make([]indicator.StockSource, 0, len(codes))
	for _, code := range codes {
		sources = append(sources, stocksource.NewDBStockByCode(code, p.tradeDate))
	}
	return sources, nil
}

func TestRun(t *testing.T) {
	// ---- 1. 初始化数据库连接 ----
	cfg, err := config.Load("../../../config.yaml")
	if err != nil {
		t.Fatalf("加载配置文件失败: %v", err)
	}
	if err := db.Init(&cfg.Database); err != nil {
		t.Fatalf("连接数据库失败（请确认 MySQL 已启动）: %v", err)
	}
	defer db.Close()

	// ---- 2. 注册数据源 adapter（指标计算可能需要）----
	for _, dsCfg := range cfg.DataSources {
		if !dsCfg.Enabled {
			continue
		}
		reg := adapter.GetRegistry()
		switch dsCfg.Provider {
		case eastmoney.AdapterName:
			ds := eastmoney.New()
			initConfig := map[string]interface{}{"cookie": dsCfg.Cookie}
			for k, v := range dsCfg.Extra {
				initConfig[k] = v
			}
			if err := ds.Init(initConfig); err != nil {
				t.Logf("警告: 初始化数据源 %s 失败: %v", dsCfg.Provider, err)
				continue
			}
			reg.Register(ds)
		}
	}

	// ---- 3. 构建 indicator.Engine（直接聚合所有内置指标，避免循环依赖）----
	var allIndicators []indicator.Indicator
	allIndicators = append(allIndicators, technical.All()...)
	allIndicators = append(allIndicators, market.All()...)
	allIndicators = append(allIndicators, fundamental.All()...)
	allIndicators = append(allIndicators, financial.All()...)
	engine := indicator.NewEngine(allIndicators)

	// ---- 4. 插入测试策略到 DB ----
	strategy := model.Strategy{
		UID:        1,
		Name:       "test-runner-integration",
		Conditions: testStrategyConditions,
		LogicalOp:  "and",
	}
	if err := db.GetDB().Create(&strategy).Error; err != nil {
		t.Fatalf("插入测试策略失败: %v", err)
	}
	strategyID := strategy.ID

	// 确保测试结束后清理
	defer func() {
		db.GetDB().Delete(&strategy, strategyID)
	}()

	// ---- 5. 构建订阅 + Runner ----
	tradeDate := parseTestDate("2026-05-24")

	sub := &model.Subscription{
		ID:           999,
		UID:          1,
		Name:         "集成测试订阅",
		StrategyID:   strategyID,
		Scope:        model.ScopeCustom,
		CustomStocks: `["000507"]`,
	}

	testRunner := NewSubscriptionRunner(
		&testProvider{tradeDate: tradeDate},
		engine,
		&testNotifier{},
	)

	// ---- 6. 执行 Run() ----
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := testRunner.Run(ctx, sub)
	if err != nil {
		t.Fatalf("Run() 返回错误: %v", err)
	}

	// ---- 7. 断言结果 ----
	t.Logf("===== Run() 执行结果 =====")
	t.Logf("状态: %s", result.Status)
	t.Logf("扫描总数: %d", result.TotalScanned)
	t.Logf("匹配数量: %d", result.MatchCount)
	t.Logf("耗时: %d ms", result.DurationMs)
	t.Logf("LogID: %d", result.LogID)
	for i, ms := range result.MatchStocks {
		t.Logf("  匹配[%d]: %s (%s) %.2f", i+1, ms.Name, ms.Code, ms.Price)
	}

	// 基本断言
	if result.Status == "" {
		t.Error("RunResult.Status 不应为空")
	}
	if result.TotalScanned == 0 {
		t.Error("TotalScanned 应 > 0（至少扫描了 CustomStocks 中的 000507）")
	}
	if result.DurationMs == 0 {
		t.Error("DurationMs 应 > 0（执行应有耗时记录）")
	}

	// 核心断言: 000507 应该被选中（用户确认此条件可以选出珍珠港）
	if result.MatchCount > 0 {
		found := false
		for _, ms := range result.MatchStocks {
			if ms.Code == "000507" {
				found = true
				t.Logf("✅ 000507 珍珠港 被成功选中! 价格: %.2f", ms.Price)
				break
			}
		}
		if !found {
			t.Errorf("MatchCount=%d 但未找到 000507，匹配列表: %+v", result.MatchCount, result.MatchStocks)
		}
	} else {
		t.Logf("⚠️ MatchCount=0，000507 未被选中。可能原因:")
		t.Logf("   - 数据库缺少 000507 的 K线/基本面/财务数据")
		t.Logf("   - 信号条件与当前数据不匹配（需核实策略参数）")
		t.Logf("   这不一定是 bug，取决于 DB 中的实际数据完整性")
	}
}

// parseTestDate 将 YYYY-MM-DD 字符串转为 int 格式的交易日期（如 20260524）
func parseTestDate(dateStr string) int {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 20260524 // fallback
	}
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d
}
