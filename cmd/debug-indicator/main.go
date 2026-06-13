// 指标/信号调试工具 — 直接编辑下方变量，运行即可测试。
//
// 用法:
//
//	go run ./cmd/debug-indicator/
//
// 修改 signalIDs 和 stockCode 后重新运行。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/eastmoney"
	"stock-ai/internal/config"
	"stock-ai/internal/db"
	"stock-ai/internal/indicator"
	"stock-ai/internal/indicator/financial"
	"stock-ai/internal/indicator/fundamental"
	"stock-ai/internal/indicator/market"
	"stock-ai/internal/indicator/technical"
	"stock-ai/internal/model"
	"stock-ai/utils"
)

// ============================================================================
//  调试配置 — 直接修改下面的变量
// ============================================================================

// stockCode 测试目标股票代码（6位数字，如 600519 = 茅台）
var stockCode = "300484"

// signalIDs 要测试的信号列表（8位完整 SignalID）
//
// 参考:
//
//	技术面: 01001001 均线多头 / 01003001 MACD水上金叉 / 01003005 MACD死叉
//	        01102101 缠论准备买入 / 01103002 红顶底见底 / 01006001 513战法
//	行情面: 02001003 成交量区间 / 02002001 成交额区间
//	基本面: 03001001 主板上市
//	财务面: 04001001 PE-TTM大于 / 04002002 PB小于 / 04004001 ROE大于
var signalIDs = []string{
	"01004001", // KDJ
}

// tradeDate 交易日期 YYYYMMDD，0 = 自动取当前日期
var tradeDate = 0

// configPath 配置文件路径
var configPath = "config.yaml"

// maxConcurrency 引擎并发度 (1 = 顺序执行，便于观察)
var maxConcurrency = 1

// klineLoadLimit K线加载条数，0 由 DAO 兜底为 250，设大值取全量
const klineLoadLimit = 100000

// emAdapter 东方财富数据源（日K从东财API获取，不复用DB）
var emAdapter *eastmoney.Adapter

func main() {
	// 初始化：加载配置 + 连接数据库 + 注册全部指标
	reg, err := initAll(configPath)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}
	defer db.Close()

	// 构建信号配置
	configs := buildConfigs(reg, signalIDs)

	date := resolveDate(tradeDate)

	fmt.Println("\n========== 调试信息 ==========")
	fmt.Printf("  股票:    %s\n", stockCode)
	fmt.Printf("  信号数:  %d\n", len(configs))
	fmt.Printf("  日期:    %d\n", date)
	for _, c := range configs {
		fmt.Printf("    → %s\n", c.SignalID)
	}
	fmt.Println("==============================\n")

	// 构造全量数据源（日K从东财获取，其他从DB）
	src := newEagerSource(stockCode, date, emAdapter)
	src.printLoadReport()
	fmt.Println()

	// 执行选股引擎
	stocks := []indicator.StockSource{src}
	engine := reg.Engine()
	results := engine.Execute(stocks, configs, maxConcurrency)

	printResults(results)

	if len(results) == 0 || results[0].Result != indicator.ResultPassed {
		os.Exit(1)
	}
}

// ============================================================================
//  eagerSource — StockSource 的全量热加载实现
//
//  与 DBStock（懒加载）不同，eagerSource 在构造时一次性加载所有数据，
//  并记录每项的加载状态，方便调试时直观看到哪些数据可用、哪些缺失。
// ============================================================================

type loadStatus int

const (
	loadOK    loadStatus = iota // 成功加载
	loadEmpty                   // 数据为空 (ErrDataEmpty)
	loadErr                     // 加载出错
)

type eagerSource struct {
	code      string
	tradeDate int

	// 基本面（必定加载）
	detail *model.Stock
	name   string

	// K 线数据 + 加载状态
	dailyKlines   []*model.DailyKline
	dailyStatus   loadStatus
	weeklyKlines  []*model.WeeklyKline
	weeklyStatus  loadStatus
	monthlyKlines []*model.MonthlyKline
	monthlyStatus loadStatus
	yearlyKlines  []*model.YearlyKline
	yearlyStatus  loadStatus

	// 财务数据 + 加载状态
	snapshot          *model.StockDailySnapshot
	snapshotStatus    loadStatus
	perfReports       []*model.PerformanceReport
	perfStatus        loadStatus
	shareholderVal    *model.ShareholderCount
	shareholderStatus loadStatus
	shareChanges      []*model.ShareChange
	shareChangeStatus loadStatus
}

func newEagerSource(code string, tradeDate int, adapter *eastmoney.Adapter) *eagerSource {
	s := &eagerSource{code: code, tradeDate: tradeDate}

	// 1. 股票基本信息
	detail, err := db.FindStockByCode(code)
	if err != nil {
		log.Printf("[EAGER] 股票详情加载失败: %v", err)
	} else {
		s.detail = &detail
		s.name = detail.Name
	}

	// 2. K 线 — 日K从东财API获取，周/月/年从DB
	s.dailyKlines, s.dailyStatus = loadDailyKlinesFromEastMoney(adapter, code)
	s.weeklyKlines, s.weeklyStatus = loadWeeklyKlines(code, tradeDate)
	s.monthlyKlines, s.monthlyStatus = loadMonthlyKlines(code, tradeDate)
	s.yearlyKlines, s.yearlyStatus = loadYearlyKlines(code, tradeDate)

	// 3. 快照
	s.snapshot, s.snapshotStatus = loadSnapshot(code, tradeDate)

	// 4. 财报
	s.perfReports, s.perfStatus = loadPerfReports(code, tradeDate)

	// 5. 股东人数
	s.shareholderVal, s.shareholderStatus = loadShareholderCount(code, tradeDate)

	// 6. 股本变动
	s.shareChanges, s.shareChangeStatus = loadShareChanges(code)

	return s
}

// printLoadReport 打印热加载数据摘要
func (s *eagerSource) printLoadReport() {
	fmt.Println("┌──────────────── 数据加载报告 ────────────────┐")
	fmt.Printf("│  %-6s  %-36s │\n", "", statusLabel(loadOK))
	fmt.Printf("│  股票: %-8s  %-30s │\n", s.code, s.name)
	fmt.Println("├──────────────────────────────────────────────┤")
	s.printRow("日K", lenOrDash(s.dailyKlines), s.dailyStatus)
	s.printRow("周K", lenOrDash(s.weeklyKlines), s.weeklyStatus)
	s.printRow("月K", lenOrDash(s.monthlyKlines), s.monthlyStatus)
	s.printRow("年K", lenOrDash(s.yearlyKlines), s.yearlyStatus)
	s.printRow("快照", snapshotStr(s.snapshot), s.snapshotStatus)
	s.printRow("财报", lenOrDash(s.perfReports), s.perfStatus)
	s.printRow("股东人数", shareholderCountStr(s.shareholderVal), s.shareholderStatus)
	s.printRow("股本变动", lenOrDash(s.shareChanges), s.shareChangeStatus)
	fmt.Println("└──────────────────────────────────────────────┘")
}

func (s *eagerSource) printRow(label string, detail string, status loadStatus) {
	fmt.Printf("│  %-8s %-12s %-24s │\n", label, detail, statusLabel(status))
}

func lenOrDash(slice interface{}) string {
	switch v := slice.(type) {
	case []*model.DailyKline:
		if v == nil {
			return "-"
		}
		return fmt.Sprintf("%d条", len(v))
	case []*model.WeeklyKline:
		if v == nil {
			return "-"
		}
		return fmt.Sprintf("%d条", len(v))
	case []*model.MonthlyKline:
		if v == nil {
			return "-"
		}
		return fmt.Sprintf("%d条", len(v))
	case []*model.YearlyKline:
		if v == nil {
			return "-"
		}
		return fmt.Sprintf("%d条", len(v))
	case []*model.PerformanceReport:
		if v == nil {
			return "-"
		}
		return fmt.Sprintf("%d期", len(v))
	case []*model.ShareChange:
		if v == nil {
			return "-"
		}
		return fmt.Sprintf("%d条", len(v))
	default:
		return "-"
	}
}

func snapshotStr(s *model.StockDailySnapshot) string {
	if s == nil {
		return "-"
	}
	return fmt.Sprintf("%d", s.TradeDate)
}

func shareholderCountStr(s *model.ShareholderCount) string {
	if s == nil {
		return "-"
	}
	return fmt.Sprintf("%d户 (%d)", s.HolderNum, s.EndDate)
}

func statusLabel(s loadStatus) string {
	switch s {
	case loadOK:
		return "✅ 已加载"
	case loadEmpty:
		return "⚠ 无数据"
	case loadErr:
		return "❌ 加载失败"
	default:
		return "?"
	}
}

// --- StockSource 接口实现 ---

func (s *eagerSource) GetCode() string { return s.code }
func (s *eagerSource) GetName() string { return s.name }
func (s *eagerSource) GetDetail() (*model.Stock, error) {
	if s.detail == nil {
		return nil, indicator.ErrDataEmpty
	}
	return s.detail, nil
}

// --- TechnicalSource ---

func (s *eagerSource) GetDailyKline() ([]*model.DailyKline, error) {
	return s.dailyKlines, toErr(s.dailyStatus)
}
func (s *eagerSource) GetWeeklyKline() ([]*model.WeeklyKline, error) {
	return s.weeklyKlines, toErr(s.weeklyStatus)
}
func (s *eagerSource) GetMonthlyKline() ([]*model.MonthlyKline, error) {
	return s.monthlyKlines, toErr(s.monthlyStatus)
}
func (s *eagerSource) GetYearlyKline() ([]*model.YearlyKline, error) {
	return s.yearlyKlines, toErr(s.yearlyStatus)
}

func toErr(status loadStatus) error {
	switch status {
	case loadOK:
		return nil
	case loadEmpty:
		return indicator.ErrDataEmpty
	default:
		return indicator.ErrDatabase
	}
}

// --- MarketSource ---

func (s *eagerSource) GetDailySnapshot() (*model.StockDailySnapshot, error) {
	switch s.snapshotStatus {
	case loadOK:
		return s.snapshot, nil
	case loadEmpty:
		return nil, indicator.ErrDataEmpty
	default:
		return nil, indicator.ErrDatabase
	}
}

// --- FinancialSource ---

func (s *eagerSource) GetPerformanceReport() ([]*model.PerformanceReport, error) {
	switch s.perfStatus {
	case loadOK:
		return s.perfReports, nil
	case loadEmpty:
		return nil, indicator.ErrDataEmpty
	default:
		return nil, indicator.ErrDatabase
	}
}

func (s *eagerSource) GetShareholderCount() (*model.ShareholderCount, error) {
	switch s.shareholderStatus {
	case loadOK:
		return s.shareholderVal, nil
	case loadEmpty:
		return nil, indicator.ErrDataEmpty
	default:
		return nil, indicator.ErrDatabase
	}
}

// --- 加载函数 ---

// --- 日K: 从东方财富 API 获取 ---

func loadDailyKlinesFromEastMoney(a *eastmoney.Adapter, code string) ([]*model.DailyKline, loadStatus) {
	if a == nil {
		log.Println("[EAGER] 东方财富适配器未初始化，跳过日K")
		return nil, loadErr
	}

	ctx := context.Background()
	data, err := a.GetDailyKLine(ctx, code, adapter.AdjQFQ)
	if err != nil {
		log.Printf("[EAGER] 东财日K加载失败: %v", err)
		return nil, loadErr
	}
	if len(data) == 0 {
		return nil, loadEmpty
	}

	klines, err := convertStockPriceDaily(data)
	if err != nil {
		log.Printf("[EAGER] 东财日K转换失败: %v", err)
		return nil, loadErr
	}

	log.Printf("[EAGER] 东财日K加载成功: %d条", len(klines))
	return klines, loadOK
}

// convertStockPriceDaily 将适配器返回的 StockPriceDaily 转为 model.DailyKline。
// 返回 newest-first（[0]=最新），与 DB 查询的排序一致。
func convertStockPriceDaily(data []adapter.StockPriceDaily) ([]*model.DailyKline, error) {
	n := len(data)
	result := make([]*model.DailyKline, n)
	for i, d := range data {
		date, err := strconv.Atoi(strings.ReplaceAll(d.Date, "-", ""))
		if err != nil {
			return nil, err
		}
		// 反转: data[0]=最旧 → result[n-1-i]=最新
		result[n-1-i] = &model.DailyKline{
			StockCode:    d.Code,
			TradeDate:    date,
			Open:         int(d.Open),
			High:         int(d.High),
			Low:          int(d.Low),
			Close:        int(d.Close),
			Volume:       d.Volume,
			Amount:       d.Amount,
			TurnoverRate: d.Turnover,
		}
	}
	return result, nil
}

func loadWeeklyKlines(code string, date int) ([]*model.WeeklyKline, loadStatus) {
	data, err := db.FindWeeklyKlines(code, date, klineLoadLimit)
	if err != nil {
		log.Printf("[EAGER] 周K加载失败: %v", err)
		return nil, loadErr
	}
	if len(data) == 0 {
		return nil, loadEmpty
	}
	return data, loadOK
}

func loadMonthlyKlines(code string, date int) ([]*model.MonthlyKline, loadStatus) {
	data, err := db.FindMonthlyKlines(code, date, klineLoadLimit)
	if err != nil {
		log.Printf("[EAGER] 月K加载失败: %v", err)
		return nil, loadErr
	}
	if len(data) == 0 {
		return nil, loadEmpty
	}
	return data, loadOK
}

func loadYearlyKlines(code string, date int) ([]*model.YearlyKline, loadStatus) {
	data, err := db.FindYearlyKlines(code, date, klineLoadLimit)
	if err != nil {
		log.Printf("[EAGER] 年K加载失败: %v", err)
		return nil, loadErr
	}
	if len(data) == 0 {
		return nil, loadEmpty
	}
	return data, loadOK
}

func loadSnapshot(code string, date int) (*model.StockDailySnapshot, loadStatus) {
	// 快照需要知道最新交易日的日期，先取日K
	klines, err := db.FindDailyKlines(code, date, 1)
	if err != nil {
		log.Printf("[EAGER] 快照-取日K失败: %v", err)
		return nil, loadErr
	}
	if len(klines) == 0 {
		return nil, loadEmpty
	}
	snapshot, err := db.FindSnapshotByStockAndDate(code, klines[0].TradeDate)
	if err != nil {
		log.Printf("[EAGER] 快照加载失败: %v", err)
		return nil, loadErr
	}
	if snapshot == nil {
		return nil, loadEmpty
	}
	return snapshot, loadOK
}

func loadPerfReports(code string, date int) ([]*model.PerformanceReport, loadStatus) {
	data, err := db.GetPerformanceReports(code, date, 8)
	if err != nil {
		log.Printf("[EAGER] 财报加载失败: %v", err)
		return nil, loadErr
	}
	if len(data) == 0 {
		return nil, loadEmpty
	}
	return data, loadOK
}

func loadShareholderCount(code string, date int) (*model.ShareholderCount, loadStatus) {
	data, err := db.FindLatestShareholderCount(code, date)
	if err != nil {
		log.Printf("[EAGER] 股东人数加载失败: %v", err)
		return nil, loadErr
	}
	if data == nil {
		return nil, loadEmpty
	}
	return data, loadOK
}

func loadShareChanges(code string) ([]*model.ShareChange, loadStatus) {
	changes, err := db.FindShareChanges(code, 0)
	if err != nil {
		log.Printf("[EAGER] 股本变动加载失败: %v", err)
		return nil, loadErr
	}
	result := make([]*model.ShareChange, len(changes))
	for i := range changes {
		result[i] = &changes[i]
	}
	return result, loadOK
}

// ============================================================================
//  初始化
// ============================================================================

func initAll(cfgPath string) (*indicator.Registry, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}
	log.Printf("[INFO] 配置加载成功: %s", cfgPath)

	if err := db.Init(&cfg.Database); err != nil {
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}
	log.Println("[INFO] 数据库连接成功")

	allIndicators := append(
		append(technical.All(), market.All()...),
		append(fundamental.All(), financial.All()...)...,
	)
	reg := indicator.NewRegistry(allIndicators)
	log.Printf("[INFO] 已注册 %d 个指标", len(allIndicators))

	// 初始化东方财富适配器（日K从API获取）
	emAdapter = eastmoney.New()
	for _, ds := range cfg.DataSources {
		if ds.Provider == eastmoney.AdapterName && ds.Enabled {
			initCfg := map[string]interface{}{"cookie": ds.Cookie}
			for k, v := range ds.Extra {
				initCfg[k] = v
			}
			if err := emAdapter.Init(initCfg); err != nil {
				log.Printf("[WARN] 东方财富适配器初始化失败: %v", err)
			} else {
				log.Println("[INFO] 东方财富适配器已就绪")
			}
			break
		}
	}

	return reg, nil
}

// ============================================================================
//  信号配置构建
// ============================================================================

// buildConfigs 从 SignalID 列表构建 SignalConfig，自动匹配指标默认配置。
func buildConfigs(reg *indicator.Registry, ids []string) []*indicator.SignalConfig {
	configs := make([]*indicator.SignalConfig, 0, len(ids))

	for _, sid := range ids {
		indID := indicator.IndicatorID(sid)
		ind, ok := reg.GetIndicatorByID(indID)
		if !ok {
			log.Printf("[WARN] 指标不存在: %s (来自信号 %s)，跳过", indID, sid)
			continue
		}

		var cfg *indicator.SignalConfig
		for _, sig := range ind.AllSignals() {
			for _, src := range []string{"0", "1"} {
				fullID := ind.ID() + src + sig.Seq()
				if fullID == sid && sig.DefaultConfig() != nil {
					cp := *sig.DefaultConfig()
					cp.SignalID = sid
					cfg = &cp
					break
				}
			}
			if cfg != nil {
				break
			}
		}

		if cfg == nil {
			log.Printf("[WARN] 信号 %s 无默认配置，使用空配置", sid)
			cfg = &indicator.SignalConfig{SignalID: sid}
		}

		configs = append(configs, cfg)
	}

	return configs
}

// ============================================================================
//  辅助
// ============================================================================

func resolveDate(d int) int {
	if d != 0 {
		return d
	}
	return utils.TodayTradeDate()
}

func printResults(results []*indicator.EvaluatedStock) {
	if len(results) == 0 {
		fmt.Println("⚠ 无结果（请检查股票代码是否正确）")
		return
	}

	r := results[0]
	resultLabel := func(res indicator.EvaluatedResult) string {
		switch res {
		case indicator.ResultPassed:
			return "✅ 通过(Passed)"
		case indicator.ResultPending:
			return "⏳ 待定(Pending)"
		case indicator.ResultRejected:
			return "❌ 拒绝(Rejected)"
		default:
			return fmt.Sprintf("未知(%d)", res)
		}
	}

	fmt.Println("┌──────────────────────────────────────────────┐")
	fmt.Printf("│  股票: %-12s (%-8s)              │\n", r.Code, r.Name)
	fmt.Printf("│  价格: %.2f                                  │\n", r.Price)
	fmt.Printf("│  结果: %-36s │\n", resultLabel(r.Result))
	if r.SignalID != "" {
		fmt.Printf("│  触发信号: %-32s       │\n", r.SignalID)
	}
	fmt.Printf("│  详情: %-36s │\n", r.Message)
	fmt.Println("└──────────────────────────────────────────────┘")
}
