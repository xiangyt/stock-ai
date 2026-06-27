package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
)

// ============================================================================
//  defaultEngine — Engine 的默认实现
// ============================================================================

type defaultEngine struct {
	screener        Screener
	feeCalculator   FeeCalculator
	positionManager PositionManager
	exitExecutor    ExitExecutor
	tradeRecorder   TradeRecorder
	dataProvider    DataProvider
	concurrency     int
}

// NewEngine 创建回测引擎。
func NewEngine(opts ...EngineOption) (Engine, error) {
	e := &defaultEngine{concurrency: 10}
	for _, opt := range opts {
		opt(e)
	}
	if err := e.validate(); err != nil {
		return nil, err
	}
	return e, nil
}

// validate 校验必需组件已注入。
func (e *defaultEngine) validate() error {
	if e.screener == nil {
		return fmt.Errorf("screening: Screener is required, use WithScreener()")
	}
	if e.feeCalculator == nil {
		return fmt.Errorf("screening: FeeCalculator is required, use WithFeeCalculator()")
	}
	return nil
}

// Initiate 实现 Engine，创建运行记录并异步启动回测。
func (e *defaultEngine) Initiate(ctx context.Context, req RunRequest) (uint64, error) {
	recorder, ok := e.tradeRecorder.(*DBTradeRecorder)
	if !ok {
		return 0, fmt.Errorf("Initiate requires DBTradeRecorder to create run record")
	}
	runID, err := recorder.CreateRun(ctx, req)
	if err != nil {
		return 0, err
	}
	// 使用 Background context，避免 HTTP 请求结束后 ctx 被取消
	// 导致异步 goroutine 中数据库写入失败（context canceled）。
	go e.runAsync(context.Background(), runID, req)
	return runID, nil
}

// ============================================================================
//  runContext — 运行上下文
// ============================================================================

type runContext struct {
	initialCapital float64
	trades         []Trade
	dailySnapshots []EquitySnapshot
}

// ============================================================================
//  bar — 当日行情
// ============================================================================

type bar struct {
	open, high, low, close float64 // 价格均以元为单位
}

// barPrices 将 bar 映射转换为价格映射。
func barPrices(bars map[string]bar) map[string]float64 {
	prices := make(map[string]float64, len(bars))
	for code, b := range bars {
		prices[code] = b.close
	}
	return prices
}

// ============================================================================
//  loadDay — 加载当日行情
// ============================================================================

func (e *defaultEngine) loadDay(
	ctx context.Context, date string,
	pool []string, holdings map[string]*HoldingPosition,
) ([]*StockSnapshot, map[string]bar) {
	codeSet := make(map[string]bool)
	for _, c := range pool {
		codeSet[c] = true
	}
	for c := range holdings {
		codeSet[c] = true
	}
	codes := make([]string, 0, len(codeSet))
	for c := range codeSet {
		codes = append(codes, c)
	}
	sort.Strings(codes)

	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, nil
	}
	tradeDate := t.Year()*10000 + int(t.Month())*100 + t.Day()

	if e.dataProvider != nil {
		snaps, err := e.dataProvider.FetchSnapshots(ctx, codes, tradeDate)
		if err == nil && len(snaps) > 0 {
			bars := make(map[string]bar, len(snaps))
			for _, s := range snaps {
				s.TradeDate = tradeDate
				bars[s.Code] = bar{close: s.Price}
			}
			return snaps, bars
		}
	}

	var klines []model.DailyKline
	db.GetDB().WithContext(ctx).
		Where("stock_code IN ? AND trade_date = ?", codes, tradeDate).
		Find(&klines)

	bars := make(map[string]bar, len(klines))
	for _, k := range klines {
		bars[k.StockCode] = bar{
			open:  float64(k.Open) / 100,
			high:  float64(k.High) / 100,
			low:   float64(k.Low) / 100,
			close: float64(k.Close) / 100,
		}
	}

	snapshots := make([]*StockSnapshot, 0, len(bars))
	for code, b := range bars {
		snapshots = append(snapshots, &StockSnapshot{Code: code, Price: b.close, TradeDate: tradeDate})
	}
	return snapshots, bars
}

// ============================================================================
//  calcMetrics — 计算最终指标
// ============================================================================

func (e *defaultEngine) calcMetrics(rc *runContext, initial float64, tradingDays int) *RunResult {
	if len(rc.dailySnapshots) == 0 {
		return &RunResult{}
	}
	final := rc.dailySnapshots[len(rc.dailySnapshots)-1].TotalEquity
	totalReturn := (final - initial) / initial * 100

	years := float64(tradingDays) / 250.0
	annualReturn := 0.0
	if years > 0.01 && final > 0 {
		annualReturn = (math.Pow(final/initial, 1.0/years) - 1) * 100
	}

	maxDD := 0.0
	peak := initial
	for _, s := range rc.dailySnapshots {
		if s.TotalEquity > peak {
			peak = s.TotalEquity
		}
		dd := (peak - s.TotalEquity) / peak * 100
		if dd > maxDD {
			maxDD = dd
		}
	}

	wins := 0
	totalProfit := 0.0
	totalLoss := 0.0
	for _, t := range rc.trades {
		if t.Type == 2 {
			if t.Profit > 0 {
				wins++
				totalProfit += t.Profit
			} else {
				totalLoss += -t.Profit
			}
		}
	}
	winRate := 0.0
	if len(rc.trades) > 0 {
		winRate = float64(wins) / float64(len(rc.trades)) * 100
	}
	profitFactor := 0.0
	if totalLoss > 0 {
		profitFactor = totalProfit / totalLoss
	}

	return &RunResult{
		TotalReturn: totalReturn, AnnualReturn: annualReturn,
		MaxDrawdown: maxDD, WinRate: winRate,
		ProfitFactor: profitFactor, TradeCount: len(rc.trades),
	}
}

// ============================================================================
//  convertSignalConfigs — screening.SignalConfig → indicator.SignalConfig
// ============================================================================

func convertSignalConfigs(configs []SignalConfig) []*indicator.SignalConfig {
	result := make([]*indicator.SignalConfig, 0, len(configs))
	for _, cfg := range configs {
		result = append(result, &indicator.SignalConfig{
			SignalID: cfg.SignalID,
			Operator: indicator.CompareOperator(cfg.Operator),
			Params:   cfg.Params,
		})
	}
	return result
}
