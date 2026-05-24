package datacollect

import (
	"math"
	"testing"

	"stock-ai/internal/model"
)

// ============================================================================
//  测试辅助函数
// ============================================================================

// mkReport 快速构造 PerformanceReport（只填快照计算所需的字段）
func mkReport(code string, reportDate, noticeDate int, reportType string,
	parentNetProfit float64, totalRevenue float64, bvps float64) model.PerformanceReport {
	return model.PerformanceReport{
		StockCode:       code,
		ReportDate:      reportDate,
		NoticeDate:      noticeDate,
		ReportType:      reportType,
		ParentNetProfit: parentNetProfit,
		TotalRevenue:    totalRevenue,
		BVPS:            bvps,
		BasicEPS:        parentNetProfit / 1e8, // 模拟值
		ROEW:            15.0,
		ROA:             8.0,
		GrossMargin:     35.0,
		NetMargin:       12.0,
		DeductNetProfit: parentNetProfit * 0.9,
		DebtRatio:       45.0,
	}
}

// mkShare 快速构造 ShareChange
func mkShare(changeDate int, totalShares, floatShares int64) model.ShareChange {
	return model.ShareChange{
		ChangeDate:   changeDate,
		TotalShares:  totalShares,
		FloatAShares: floatShares,
	}
}

// nearlyEqual 浮点数近似比较（精度 1e-2）
func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.01
}

// ============================================================================
//  buildStockSnapshot 核心纯函数测试
// ============================================================================

func TestBuildStockSnapshot_BasicFields(t *testing.T) {
	code := "000507"
	tradeDate := 20250516
	closePrice := 12.5 // 元

	groups := reportGroups{
		All: []model.PerformanceReport{
			mkReport(code, 20240930, 20241028, "三季报", 80e8, 300e8, 8.0),
		},
	}
	share := mkShare(20250101, 10e8, 6e8)

	snap := buildStockSnapshot(code, tradeDate, closePrice, &share, &groups)

	// 基础字段
	if snap.StockCode != code {
		t.Errorf("StockCode = %s, want %s", snap.StockCode, code)
	}
	if snap.TradeDate != tradeDate {
		t.Errorf("TradeDate = %d, want %d", snap.TradeDate, tradeDate)
	}
	if snap.TotalShares != 10e8 {
		t.Errorf("TotalShares = %d, want %d", snap.TotalShares, int64(10e8))
	}
	if snap.FloatShares != 6e8 {
		t.Errorf("FloatShares = %d, want %d", snap.FloatShares, int64(6e8))
	}
}

func TestBuildStockSnapshot_MarketCap(t *testing.T) {
	code := "000507"
	tradeDate := 20250516
	closePrice := 10.0

	groups := reportGroups{
		All: []model.PerformanceReport{
			mkReport(code, 20240930, 20241028, "三季报", 100e8, 400e8, 5.0),
		},
	}
	share := mkShare(20250101, 10e8, 5e8)

	snap := buildStockSnapshot(code, tradeDate, closePrice, &share, &groups)

	wantTotalCap := float64(10e8) * 10.0 // 总股本 × 收盘价
	wantCircCap := float64(5e8) * 10.0   // 流通股 × 收盘价

	if !nearlyEqual(snap.TotalMarketCap, wantTotalCap) {
		t.Errorf("TotalMarketCap = %.2f, want %.2f", snap.TotalMarketCap, wantTotalCap)
	}
	if !nearlyEqual(snap.CirculateMarketCap, wantCircCap) {
		t.Errorf("CirculateMarketCap = %.2f, want %.2f", snap.CirculateMarketCap, wantCircCap)
	}
}

func TestBuildStockSnapshot_NilShare_ZeroMarketCap(t *testing.T) {
	code := "000568"
	tradeDate := 20250520
	closePrice := 25.0

	groups := reportGroups{
		All: []model.PerformanceReport{
			mkReport(code, 20240930, 20241028, "三季报", 50e8, 200e8, 10.0),
		},
	}

	snap := buildStockSnapshot(code, tradeDate, closePrice, nil, &groups)

	// nil 股本 → 市值为 0，估值指标不应填充
	if snap.TotalMarketCap != 0 {
		t.Errorf("TotalMarketCap should be 0 when share is nil, got %.2f", snap.TotalMarketCap)
	}
	if snap.CirculateMarketCap != 0 {
		t.Errorf("CirculateMarketCap should be 0 when share is nil, got %.2f", snap.CirculateMarketCap)
	}
	// 市值为 0 时不应进入财报分支，PE/PB 等应保持零值
	if snap.PETTM != 0 || snap.PB != 0 {
		t.Errorf("PE/PB should be 0 with zero market cap, got PETTM=%.4f PB=%.4f", snap.PETTM, snap.PB)
	}
}

func TestBuildStockSnapshot_NoReports_Fallback(t *testing.T) {
	code := "000001"
	tradeDate := 20250516
	closePrice := 15.0
	share := mkShare(20250101, 100e8, 80e8)

	groups := reportGroups{} // 空！

	snap := buildStockSnapshot(code, tradeDate, closePrice, &share, &groups)

	// 有股本但无财报 → 只有市值，无 PE/PB/ROE 等
	if snap.TotalMarketCap == 0 {
		t.Error("TotalMarketCap should be non-zero")
	}
	// 无财报 → 所有估值指标为 0
	if snap.PETTM != 0 || snap.PB != 0 || snap.ROE != 0 {
		t.Errorf("with no reports all indicators should be 0: PETTM=%.4f PB=%.4f ROE=%.4f",
			snap.PETTM, snap.PB, snap.ROE)
	}
}

func TestBuildStockSnapshot_ReportAfterTradeDate_Ignored(t *testing.T) {
	code := "000507"
	tradeDate := 20250516 // 交易日期
	closePrice := 10.0

	// 公告日在交易日期之后 → 不应被使用
	groups := reportGroups{
		All: []model.PerformanceReport{
			mkReport(code, 20250630, 20250815, "中报", 200e8, 500e8, 12.0), // notice > tradeDate
		},
	}
	share := mkShare(20250101, 10e8, 6e8)

	snap := buildStockSnapshot(code, tradeDate, closePrice, &share, &groups)

	// 公告日之后的财报被跳过 → 估值为 0
	if snap.PETTM != 0 || snap.PEDynamic != 0 {
		t.Errorf("report after tradeDate should be ignored, got PETTM=%.4f PEDynamic=%.4f",
			snap.PETTM, snap.PEDynamic)
	}
}

func TestBuildStockSnapshot_PickLatestValidReport(t *testing.T) {
	code := "000507"
	tradeDate := 20250516
	closePrice := 20.0

	groups := reportGroups{
		All: []model.PerformanceReport{
			mkReport(code, 20240331, 20240428, "一季报", 30e8, 100e8, 6.0), // notice < tradeDate ✓
			mkReport(code, 20240630, 20240825, "中报", 70e8, 250e8, 7.0),  // notice < tradeDate ✓ (最新)
			mkReport(code, 20250331, 20250428, "一季报", 35e8, 120e8, 6.5), // notice < tradeDate ✓ (最新！但 ReportDate 更晚)
			mkReport(code, 20250630, 20250720, "中报", 90e8, 350e8, 8.0),  // notice > tradeDate ✗
		},
	}
	share := mkShare(20250101, 8e8, 5e8)

	snap := buildStockSnapshot(code, tradeDate, closePrice, &share, &groups)

	// 应选用 notice_date <= 20250516 中最新的：20250428 的 2025 一季报
	if !nearlyEqual(snap.ParentNetProfit, 35e8) {
		t.Errorf("ParentNetProfit = %.2e, want 35e8 (latest valid report)", snap.ParentNetProfit)
	}
	if !nearlyEqual(snap.BVPS, 6.5) {
		t.Errorf("BVPS = %.2f, want 6.5", snap.BVPS)
	}
}

func TestBuildStockSnapshot_PB_Calculation(t *testing.T) {
	code := "000507"
	tradeDate := 20250516
	closePrice := 10.0 // 收盘价 10 元

	groups := reportGroups{
		All: []model.PerformanceReport{
			mkReport(code, 20240930, 20241028, "三季报", 100e8, 300e8, 5.0), // BVPS=5 元
		},
	}
	share := mkShare(20250101, 10e8, 6e8)

	snap := buildStockSnapshot(code, tradeDate, closePrice, &share, &groups)

	// PB = 收盘价 / BVPS = 10.0 / 5.0 = 2.0
	wantPB := 10.0 / 5.0
	if !nearlyEqual(snap.PB, wantPB) {
		t.Errorf("PB = %.4f, want %.4f (close/BVPS)", snap.PB, wantPB)
	}
}

// ============================================================================
//  calcDynamicPE 测试 — 动态市盈率年化逻辑
// ============================================================================

func TestCalcDynamicPE_Q1_Annualized4x(t *testing.T) {
	r := mkReport("", 0, 0, "一季报", 25e8, 0, 0) // 季度利润 25 亿
	marketCap := 200e8                         // 总市值 200 亿

	pe := calcDynamicPE(marketCap, &r)
	want := 200e8 / (25e8 * 4) // 年化: 25*4=100亿 → 200/100=2.0
	if !nearlyEqual(pe, want) {
		t.Errorf("Q1 dynamic PE = %.4f, want %.4f", pe, want)
	}
}

func TestCalcDynamicPE_HalfYear_2x(t *testing.T) {
	r := mkReport("", 0, 0, "中报", 60e8, 0, 0) // 半年利润 60 亿
	marketCap := 240e8

	pe := calcDynamicPE(marketCap, &r)
	want := 240e8 / (60e8 * 2) // 年化: 60*2=120亿 → 240/120=2.0
	if !nearlyEqual(pe, want) {
		t.Errorf("HalfYear dynamic PE = %.4f, want %.4f", pe, want)
	}
}

func TestCalcDynamicPE_Q3_Div3Times4(t *testing.T) {
	r := mkReport("", 0, 0, "三季报", 90e8, 0, 0) // 三季度累计 90 亿
	marketCap := 240e8

	pe := calcDynamicPE(marketCap, &r)
	annualized := 90e8 / 3.0 * 4 // 120 亿
	want := 240e8 / annualized   // 2.0
	if !nearlyEqual(pe, want) {
		t.Errorf("Q3 dynamic PE = %.4f, want %.4f", pe, want)
	}
}

func TestCalcDynamicPE_Annual_NoAnnualize(t *testing.T) {
	r := mkReport("", 0, 0, "年报", 120e8, 0, 0) // 全年利润 120 亿
	marketCap := 360e8

	pe := calcDynamicPE(marketCap, &r)
	want := 360e8 / 120e8 // 不年化 → 直接 3.0
	if !nearlyEqual(pe, want) {
		t.Errorf("Annual dynamic PE = %.4f, want %.4f", pe, want)
	}
}

func TestCalcDynamicPE_ZeroProfit_ReturnsZero(t *testing.T) {
	r := mkReport("", 0, 0, "一季报", 0, 0, 0)
	pe := calcDynamicPE(100e8, &r)
	if pe != 0 {
		t.Errorf("zero profit should return 0, got %.4f", pe)
	}
}

func TestCalcDynamicPE_NegativeProfit_AllowsNegativePE(t *testing.T) {
	r := mkReport("", 0, 0, "年报", -50e8, 0, 0) // 亏损
	marketCap := 200e8

	pe := calcDynamicPE(marketCap, &r)
	want := 200e8 / (-50e8) // -4.0
	if !nearlyEqual(pe, want) {
		t.Errorf("negative profit PE = %.4f, want %.4f", pe, want)
	}
}

// ============================================================================
//  calcStaticPE 测试 — 静态市盈率（用最新年报）
// ============================================================================

func makeGroupsWithAnnualAndQ(_ int, annualProfit, qProfit float64) reportGroups {
	return reportGroups{
		Annual: []model.PerformanceReport{
			mkReport("", 20231231, 20240320, "年报", annualProfit, 0, 0),
		},
		Quarterly: []model.PerformanceReport{
			mkReport("", 20240331, 20240428, "一季报", qProfit, 0, 0),
		},
		All: []model.PerformanceReport{
			mkReport("", 20231231, 20240320, "年报", annualProfit, 0, 0),
			mkReport("", 20240331, 20240428, "一季报", qProfit, 0, 0),
		},
	}
}

func TestCalcStaticPE_UsesLatestAnnual(t *testing.T) {
	groups := makeGroupsWithAnnualAndQ(20250516, 80e8, 25e8)
	marketCap := 160e8

	pe := calcStaticPE(marketCap, &groups, 20250516)
	want := 160e8 / 80e8 // 用年报 80 亿 → 2.0
	if !nearlyEqual(pe, want) {
		t.Errorf("static PE = %.4f, want %.4f", pe, want)
	}
}

func TestCalcStaticPE_NoAnnual_ReturnsZero(t *testing.T) {
	groups := reportGroups{
		Annual: []model.PerformanceReport{}, // 空
		All: []model.PerformanceReport{
			mkReport("", 20240331, 20240428, "一季报", 30e8, 0, 0),
		},
	}
	pe := calcStaticPE(100e8, &groups, 20250428)
	if pe != 0 {
		t.Errorf("no annual report should return 0, got %.4f", pe)
	}
}

// ============================================================================
//  calcTTMValues 测试 — TTM 滚动四季
// ============================================================================

func TestTTM_Annual_SelfOnly(t *testing.T) {
	r := mkReport("", 20241231, 20250320, "年报", 120e8, 500e8, 0)
	groups := reportGroups{
		All: []model.PerformanceReport{r},
	}
	ttm := calcTTMValues(&r, &groups)

	// 年报就是全年数据，TTM = 自身
	if !nearlyEqual(ttm.Profit, 120e8) {
		t.Errorf("annual TTM profit = %.2e, want 120e8", ttm.Profit)
	}
	if !nearlyEqual(ttm.Revenue, 500e8) {
		t.Errorf("annual TTM revenue = %.2e, want 500e8", ttm.Revenue)
	}
}

func TestTTM_Q1_Formula(t *testing.T) {
	// 最新 = 2025 一季报，上年年报 = 2024 年报，上年同期 = 2024 一季报
	q1_2025 := mkReport("", 20250331, 20250428, "一季报", 30e8, 100e8, 0)
	annual_2024 := mkReport("", 20241231, 20250320, "年报", 100e8, 400e8, 0)
	q1_2024 := mkReport("", 20240331, 20240428, "一季报", 22e8, 80e8, 0)

	groups := reportGroups{
		All: []model.PerformanceReport{q1_2025, annual_2024, q1_2024},
	}
	ttm := calcTTMValues(&q1_2025, &groups)

	// TTM 利润 = Q1_2025 + (Annual_2024 - Q1_2024) = 30 + (100-22) = 108
	wantProfit := 30e8 + (100e8 - 22e8)
	// TTM 营收 = 100 + (400-80) = 420
	wantRevenue := 100e8 + (400e8 - 80e8)

	if !nearlyEqual(ttm.Profit, wantProfit) {
		t.Errorf("Q1 TTM profit = %.2e, want %.2e", ttm.Profit, wantProfit)
	}
	if !nearlyEqual(ttm.Revenue, wantRevenue) {
		t.Errorf("Q1 TTM revenue = %.2e, want %.2e", ttm.Revenue, wantRevenue)
	}
}

func TestTTM_MissingPrevData_ReturnsZero(t *testing.T) {
	// 缺少上年年报或上年同期 → 返回零值
	q1_2025 := mkReport("", 20250331, 20250428, "一季报", 30e8, 100e8, 0)
	// 没有 2024 年报和 2024 一季报
	groups := reportGroups{
		All: []model.PerformanceReport{q1_2025},
	}
	ttm := calcTTMValues(&q1_2025, &groups)

	if ttm.Profit != 0 || ttm.Revenue != 0 {
		t.Errorf("missing prev data should return zero, got Profit=%.2e Revenue=%.2e",
			ttm.Profit, ttm.Revenue)
	}
}

// ============================================================================
//  preprocessReports 分组测试
// ============================================================================

func TestPreprocessReports_Grouping(t *testing.T) {
	reports := []model.PerformanceReport{
		mkReport("A", 20240331, 20240428, "一季报", 10, 0, 0),
		mkReport("A", 20240630, 20240825, "中报", 20, 0, 0),
		mkReport("A", 20240930, 20241028, "三季报", 30, 0, 0),
		mkReport("A", 20241231, 20250320, "年报", 40, 0, 0),
		mkReport("A", 20250331, 20250428, "一季报", 15, 0, 0),
	}

	g := preprocessReports(reports)

	if len(g.Quarterly) != 4 {
		t.Fatalf("Quarterly count = %d, want 4 (一季报+中报+三季报+2025Q1)", len(g.Quarterly))
	}
	if len(g.Annual) != 1 {
		t.Fatalf("Annual count = %d, want 1", len(g.Annual))
	}
	if len(g.All) != 5 {
		t.Fatalf("All count = %d, want 5", len(g.All))
	}

	// 验证 Annual 内容
	if g.Annual[0].ReportType != "年报" {
		t.Errorf("Annual[0].ReportType = %s, want 年报", g.Annual[0].ReportType)
	}

	// 验证排序：All 按 NoticeDate 升序
	for i := 1; i < len(g.All); i++ {
		if g.All[i].NoticeDate < g.All[i-1].NoticeDate {
			t.Error("All group not sorted by NoticeDate ascending")
		}
	}
}

func TestPreprocessReports_IgnoresUnknownType(t *testing.T) {
	reports := []model.PerformanceReport{
		mkReport("A", 20240331, 20240428, "一季报", 10, 0, 0),
		mkReport("A", 20240630, 20240825, "业绩预告", 99, 0, 0), // 未知类型
		mkReport("A", 20241231, 20250320, "年报", 40, 0, 0),
	}

	g := preprocessReports(reports)

	// 未知类型不进入任何组
	if len(g.All) != 2 {
		t.Errorf("All count = %d, want 2 (unknown type filtered)", len(g.All))
	}
	if len(g.Quarterly) != 1 {
		t.Errorf("Quarterly count = %d, want 1", len(g.Quarterly))
	}
}

// ============================================================================
//  findLatestFromList 边界测试
// ============================================================================

func TestFindLatestFromList_BeforeTradeDate(t *testing.T) {
	list := []model.PerformanceReport{
		mkReport("A", 0, 20240320, "", 0, 0, 0),
		mkReport("A", 0, 20240615, "", 0, 0, 0),
		mkReport("A", 0, 20240910, "", 0, 0, 0),
		mkReport("A", 0, 20250301, "", 0, 0, 0), // > tradeDate
	}

	r := findLatestFromList(list, 20250115)
	if r == nil {
		t.Fatal("should find a report")
	}
	if r.NoticeDate != 20240910 {
		t.Errorf("got NoticeDate = %d, want 20240910", r.NoticeDate)
	}
}

func TestFindLatestFromList_AllAfterTradeDate(t *testing.T) {
	list := []model.PerformanceReport{
		mkReport("A", 0, 20250201, "", 0, 0, 0),
		mkReport("A", 0, 20250301, "", 0, 0, 0),
	}

	r := findLatestFromList(list, 20250115)
	if r != nil {
		t.Errorf("all reports after tradeDate, should return nil, got NoticeDate=%d", r.NoticeDate)
	}
}

func TestFindLatestFromList_EmptyList(t *testing.T) {
	r := findLatestFromList([]model.PerformanceReport{}, 20250115)
	if r != nil {
		t.Error("empty list should return nil")
	}
}

// ============================================================================
//  findReportByYearAndType 测试
// ============================================================================

func TestFindReportByYearAndType_Found(t *testing.T) {
	groups := reportGroups{
		All: []model.PerformanceReport{
			mkReport("", 20241231, 20250320, "年报", 0, 0, 0),
			mkReport("", 20250331, 20250428, "一季报", 0, 0, 0),
			mkReport("", 20231231, 20240320, "年报", 0, 0, 0),
		},
	}

	r := findReportByYearAndType(&groups, 2024, "年报")
	if r == nil {
		t.Fatal("should find 2024 年报")
	}
	if r.ReportDate != 20241231 {
		t.Errorf("ReportDate = %d, want 20241231", r.ReportDate)
	}
}

func TestFindReportByYearAndType_NotFound(t *testing.T) {
	groups := reportGroups{
		All: []model.PerformanceReport{
			mkReport("", 20241231, 20250320, "年报", 0, 0, 0),
		},
	}

	r := findReportByYearAndType(&groups, 2024, "一季报") // 不存在
	if r != nil {
		t.Errorf("should return nil for missing type, got ReportDate=%d", r.ReportDate)
	}
}

// ============================================================================
//  端到端集成测试：完整场景验证
// ============================================================================

func TestBuildStockSnapshot_FullScenario_EarningsStock(t *testing.T) {
	// 场景：盈利股票的完整快照构建
	// 2025-05-16 交易日，股价 12.5 元
	// 财报：2024年报(公告0320)、2025一季报(公告0428)
	// 股本：总 10亿股，流通 6亿股
	code := "000507"
	tradeDate := 20250516
	closePrice := 12.5

	groups := reportGroups{
		Annual: []model.PerformanceReport{
			mkReport(code, 20241231, 20250320, "年报", 120e8, 500e8, 10.0),
		},
		Quarterly: []model.PerformanceReport{
			mkReport(code, 20250331, 20250428, "一季报", 35e8, 130e8, 10.5),
		},
		All: []model.PerformanceReport{
			mkReport(code, 20241231, 20250320, "年报", 120e8, 500e8, 10.0),
			mkReport(code, 20250331, 20250428, "一季报", 35e8, 130e8, 10.5),
		},
	}
	share := mkShare(20250101, 10e8, 6e8)

	snap := buildStockSnapshot(code, tradeDate, closePrice, &share, &groups)

	// --- 验证市值 ---
	expectedTotalCap := float64(10e8) * 12.5 // 125 亿
	expectedCircCap := float64(6e8) * 12.5   // 75 亿
	if !nearlyEqual(snap.TotalMarketCap, expectedTotalCap) {
		t.Errorf("TotalMarketCap = %.2e, want %.2e", snap.TotalMarketCap, expectedTotalCap)
	}
	if !nearlyEqual(snap.CirculateMarketCap, expectedCircCap) {
		t.Errorf("CirculateMarketCap = %.2e, want %.2e", snap.CirculateMarketCap, expectedCircCap)
	}

	// --- 应选用最新有效财报：2025 一季报 (notice 0428 <= 0516) ---
	if !nearlyEqual(snap.ParentNetProfit, 35e8) {
		t.Errorf("ParentNetProfit = %.2e, want 35e8", snap.ParentNetProfit)
	}
	if !nearlyEqual(snap.TotalRevenue, 130e8) {
		t.Errorf("TotalRevenue = %.2e, want 130e8", snap.TotalRevenue)
	}
	if !nearlyEqual(snap.BVPS, 10.5) {
		t.Errorf("BVPS = %.2f, want 10.5", snap.BVPS)
	}

	// --- PB = 12.5 / 10.5 ---
	wantPB := 12.5 / 10.5
	if !nearlyEqual(snap.PB, wantPB) {
		t.Errorf("PB = %.4f, want %.4f", snap.PB, wantPB)
	}

	// --- 动态 PE：市值 / (一季报利润×4) = 125e8 / (35e8×4) = 125/140 ≈ 0.8929 ---
	wantDynPE := expectedTotalCap / (35e8 * 4)
	if !nearlyEqual(snap.PEDynamic, wantDynPE) {
		t.Errorf("PEDynamic = %.4f, want %.4f", snap.PEDynamic, wantDynPE)
	}

	// --- 静态 PE：市值 / 年报利润 = 125e8 / 120e8 ≈ 1.0417 ---
	wantStaticPE := expectedTotalCap / 120e8
	if !nearlyEqual(snap.PEStatic, wantStaticPE) {
		t.Errorf("PEStatic = %.4f, want %.4f", snap.PEStatic, wantStaticPE)
	}

	// --- TTM PE：TTM利润 = Q1_2025 + (2024年报 - 2024Q1) ---
	// 注意：此场景没有 2024Q1 数据，TTM 会因缺上年同期而返回 0 → PETTM 保持 0
	// 这是正确行为：TTM 计算需要上年同期才能补齐
}

func TestBuildStockSnapshot_FullScenario_WithTTM(t *testing.T) {
	// 完整 TTM 计算：提供上年年报+上年同期数据
	code := "600519"
	tradeDate := 20250520
	closePrice := 1650.0

	// 构造完整 4 期财报链
	annual2024 := mkReport(code, 20241231, 20250320, "年报", 800e8, 1500e8, 70.0)
	q12024 := mkReport(code, 20240331, 20240428, "一季报", 200e8, 300e8, 68.0)
	q12025 := mkReport(code, 20250331, 20250428, "一季报", 240e8, 350e8, 72.0)

	groups := reportGroups{
		All: []model.PerformanceReport{annual2024, q12024, q12025},
	}
	share := mkShare(20250101, 12.56e8, 12.56e8) // 全流通

	snap := buildStockSnapshot(code, tradeDate, closePrice, &share, &groups)
	marketCap := float64(12.56e8) * 1650.0

	// TTM 利润 = 240 + (800-200) = 840 亿
	ttmProfit := 240e8 + (800e8 - 200e8)
	// TTM 营收 = 350 + (1500-300) = 1550 亿
	ttmRevenue := 350e8 + (1500e8 - 300e8)

	if !nearlyEqual(snap.PETTM, marketCap/ttmProfit) {
		t.Errorf("PETTM = %.4f, want %.4f (marketCap/TTMProfit)",
			snap.PETTM, marketCap/ttmProfit)
	}
	if !nearlyEqual(snap.PSTTM, marketCap/ttmRevenue) {
		t.Errorf("PSTTM = %.4f, want %.4f (marketCap/TTMRevenue)",
			snap.PSTTM, marketCap/ttmRevenue)
	}
}
