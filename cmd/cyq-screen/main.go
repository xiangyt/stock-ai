// 筹码分布(CYQ) 4个内置信号批量筛选工具
//
// 用法:
//
//	go run ./cmd/cyq-screen/
//
// 对全量 A 股分别用 CYQ 的 4 个内置信号（低位锁定/低位密集/双峰密集/高位密集）筛选，
// 输出每个信号的命中股票列表及多信号重叠情况。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/backtest/indicator/technical"
	"stock-ai/internal/config"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/utils"
)

// ============================================================================
//  调整配置
// ============================================================================

// tradeDate 交易日期 YYYYMMDD，0 = 自动取当前交易日
var tradeDate = 0

// maxConcurrency 并发数
var maxConcurrency = 50

// klineLoadLimit K 线加载条数（与 cyq-chart 一致，使用默认 210 根）
const klineLoadLimit = technical.CyqDefaultKlineCount

// maxShowPerSignal 每个信号最多展示的股票数（超出截断）
const maxShowPerSignal = 30

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime)

	date := resolveDate(tradeDate)

	reg, err := initAll("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化失败: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	stocks := db.LoadAllStockCodes()
	if len(stocks) == 0 {
		fmt.Fprintf(os.Stderr, "股票列表为空\n")
		os.Exit(1)
	}
	log.Printf("共 %d 只股票，交易日: %s\n", len(stocks), fmtDate(date))

	// CYQ 内置信号（纯筹码峰形态，不含获利比例/集中度判断）
	// 格式: 指标ID(01021) + 类型(0=BuiltIn) + 信号序号(2位)
	signalIDs := []string{
		"01021002", // 02 单峰密集 (BuiltIn) ← 原"低位密集"
		"01021003", // 03 双峰密集 (BuiltIn)
		"01021004", // 04 单峰密集 (BuiltIn) ← 原"高位密集"
	}
	signalNames := []string{"低位密集", "双峰密集", "高位密集"}

	// 为每个信号单独构建配置（每次只跑一个信号）
	signalConfigs := make([][]*indicator.SignalConfig, len(signalIDs))
	for i, sid := range signalIDs {
		signalConfigs[i] = buildConfigs(reg, []string{sid})
	}

	engine := reg.Engine()

	// 并发执行：每只股票 × 每个信号
	type stockResult struct {
		Code    string
		Name    string
		Price   float64
		Passed  [3]bool
		Message [3]string
	}

	var mu sync.Mutex
	results := make(map[string]*stockResult)

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(maxConcurrency)

	for _, stock := range stocks {
		code := stock.Code
		name := stock.Name

		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			src := newEagerSource(code, date)

			// 逐个信号评估（避免信号间依赖干扰）
			sr := &stockResult{Code: code, Name: name, Price: srcPrice(src)}

			for sigIdx := range signalIDs {
				configs := buildConfigs(reg, []string{signalIDs[sigIdx]})
				if len(configs) == 0 {
					continue
				}

				rList := engine.Execute([]indicator.StockSource{src}, configs, 1)
				for _, r := range rList {
					if r.Result == indicator.ResultPassed {
						sr.Passed[sigIdx] = true
						sr.Message[sigIdx] = r.Message
						break
					}
				}
			}

			mu.Lock()
			results[code] = sr
			mu.Unlock()

			return nil
		})
	}

	log.Println("开始执行...")
	if err := g.Wait(); err != nil {
		log.Printf("执行出错: %v\n", err)
	}
	log.Printf("完成，共评估 %d 只股票\n", len(results))

	// ====== 输出结果 ======
	fmt.Printf("\n%s  筹码分布(CYQ)信号批量筛选  %s  总计=%d只\n",
		strings.Repeat("=", 55), fmtDate(date), len(results))

	for sigIdx, sigName := range signalNames {
		var passed []string
		for _, r := range results {
			if r.Passed[sigIdx] {
				passed = append(passed, fmt.Sprintf("  %-8s %-10s %7.2f  %s",
					r.Code, r.Name, r.Price, truncate(r.Message[sigIdx], 50)))
			}
		}
		sort.Slice(passed, func(a, b int) bool { return passed[a] < passed[b] })

		fmt.Printf("\n【Signal %s — %s】 命中: %d 只\n",
			signalIDs[sigIdx][len(signalIDs[sigIdx])-2:], sigName, len(passed))

		if len(passed) == 0 {
			fmt.Println("  (无)")
		} else if len(passed) <= maxShowPerSignal {
			fmt.Println(strings.Join(passed, "\n"))
		} else {
			fmt.Println(strings.Join(passed[:maxShowPerSignal], "\n"))
			fmt.Printf("  ... 共 %d 只 (已截断显示前%d只)\n", len(passed), maxShowPerSignal)
		}
	}

	// 多信号重叠分析
	fmt.Printf("\n%s  多信号重叠分析\n", strings.Repeat("-", 45))
	multiPass := make([]string, 0)
	for _, r := range results {
		n := 0
		var names []string
		for i, p := range r.Passed {
			if p {
				n++
				names = append(names, signalNames[i])
			}
		}
		if n >= 2 {
			multiPass = append(multiPass,
				fmt.Sprintf("  %-8s %-10s %7.2f  [%s]",
					r.Code, r.Name, r.Price, strings.Join(names, "+")))
		}
	}
	sort.Strings(multiPass)
	if len(multiPass) > 0 {
		fmt.Printf("%d 只股票满足 >=2 个信号:\n", len(multiPass))
		fmt.Println(strings.Join(multiPass, "\n"))
	} else {
		fmt.Println("  (无重叠)")
	}
}

// ============================================================================
//  eagerSource — 复用 debug-indicator 的热加载数据源实现
// ============================================================================

type loadStatus int

const (
	loadOK loadStatus = iota
	loadEmpty
	loadErr
)

type eagerSource struct {
	code      string
	tradeDate int
	detail    *model.Stock
	name      string
	dailyKlines []*model.DailyKline
	dailyStatus   loadStatus
}

func newEagerSource(code string, tradeDate int) *eagerSource {
	s := &eagerSource{code: code, tradeDate: tradeDate}

	detail, err := db.FindStockByCode(code)
	if err == nil {
		s.detail = &detail
		s.name = detail.Name
	}

	s.dailyKlines, s.dailyStatus = loadDailyKlines(code, tradeDate)
	return s
}

func (s *eagerSource) GetCode() string       { return s.code }
func (s *eagerSource) GetName() string       { return s.name }
func (s *eagerSource) GetDailyKline() ([]*model.DailyKline, error) {
	return s.dailyKlines, toErr(s.dailyStatus)
}
// --- TechnicalSource 其余方法 ---
func (s *eagerSource) GetWeeklyKline() ([]*model.WeeklyKline, error)  { return nil, indicator.ErrDataEmpty }
func (s *eagerSource) GetMonthlyKline() ([]*model.MonthlyKline, error) { return nil, indicator.ErrDataEmpty }
func (s *eagerSource) GetYearlyKline() ([]*model.YearlyKline, error)  { return nil, indicator.ErrDataEmpty }
// --- MarketSource ---
func (s *eagerSource) GetDailySnapshot() (*model.StockDailySnapshot, error) {
	return nil, indicator.ErrDataEmpty
}
// --- FundamentalSource ---
func (s *eagerSource) GetDetail() (*model.Stock, error) {
	if s.detail == nil || s.detail.Code == "" {
		return nil, indicator.ErrDataEmpty
	}
	return s.detail, nil
}
// --- FinancialSource ---
func (s *eagerSource) GetPerformanceReport() ([]*model.PerformanceReport, error) {
	return nil, indicator.ErrDataEmpty
}
func (s *eagerSource) GetShareholderCount() (*model.ShareholderCount, error) {
	return nil, indicator.ErrDataEmpty
}

// --- 加载函数 ---

func loadDailyKlines(code string, date int) ([]*model.DailyKline, loadStatus) {
	data, err := db.FindDailyKlines(code, date, klineLoadLimit)
	if err != nil {
		return nil, loadErr
	}
	if len(data) == 0 {
		return nil, loadEmpty
	}
	return data, loadOK
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

func srcPrice(s *eagerSource) float64 {
	if len(s.dailyKlines) > 0 {
		return float64(s.dailyKlines[0].Close) / 100.0
	}
	return 0
}

// ============================================================================
//  初始化 / 配置构建 — 复用 debug-indicator 的逻辑
// ============================================================================

func initAll(cfgPath string) (*indicator.Registry, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}
	if err := db.Init(&cfg.Database); err != nil {
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}

	allIndicators := append(
		append(technical.All(), technical.All()...),
	)
	reg := indicator.NewRegistry(allIndicators)
	return reg, nil
}

// buildConfigs 从 SignalID 列表构建 SignalConfig。
func buildConfigs(reg *indicator.Registry, ids []string) []*indicator.SignalConfig {
	configs := make([]*indicator.SignalConfig, 0, len(ids))
	for _, sid := range ids {
		indID := indicator.IndicatorID(sid)
		ind, ok := reg.GetIndicatorByID(indID)
		if !ok {
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
			cfg = &indicator.SignalConfig{SignalID: sid}
		}
		configs = append(configs, cfg)
	}
	return configs
}

// ============================================================================
//  辅助函数
// ============================================================================

func resolveDate(d int) int {
	if d != 0 {
		return d
	}
	return utils.TodayTradeDate()
}

func fmtDate(d int) string {
	return fmt.Sprintf("%d", d)
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
