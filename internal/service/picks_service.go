package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/utils"
)

// ========== 复盘参数限制 ==========

const (
	// DefaultPickHoldDays 默认观察窗口（交易日）
	DefaultPickHoldDays = 5
	// MaxPickHoldDays 允许的最大观察窗口（交易日）
	MaxPickHoldDays = 60
	// PickQuoteConcurrency 并发拉取选中股票未来行情的协程数
	PickQuoteConcurrency = 20
)

// ========== 结果结构 ==========

// StrategyPicksQuery 选股复盘查询条件
type StrategyPicksQuery struct {
	Strategy       *model.Strategy // 目标策略（含 conditions JSON）
	StartDate      string          // 复盘起始日期 YYYY-MM-DD
	EndDate        string          // 复盘结束日期 YYYY-MM-DD
	HoldDays       int             // 观察窗口（交易日）
	MaxConcurrency int             // 选股计算的并发数
}

// ForwardDay 选股日之后某个交易日的行情表现。
// 四个价格维度的收益均以「选股日收盘价」为基准，单位为 %(百分比)。
type ForwardDay struct {
	Offset   int     `json:"offset"`    // 相对选股日的交易日序号，从 1 开始
	Date     string  `json:"date"`      // 交易日 YYYY-MM-DD
	OpenPct  float64 `json:"open_pct"`  // 开盘价相对基准的收益率(%)
	HighPct  float64 `json:"high_pct"`  // 最高价相对基准的收益率(%)
	LowPct   float64 `json:"low_pct"`   // 最低价相对基准的收益率(%)
	ClosePct float64 `json:"close_pct"` // 收盘价相对基准的收益率(%)
}

// StockPick 单条选股记录（一只股票在某天被策略选中）
type StockPick struct {
	Date      string       `json:"date"`       // 选股日 YYYY-MM-DD
	Code      string       `json:"code"`       // 股票代码
	Name      string       `json:"name"`       // 股票名称
	SignalID  string       `json:"signal_id"`  // 命中的信号ID
	Message   string       `json:"message"`    // 信号描述
	BaseClose float64      `json:"base_close"` // 基准价（元）：选股日收盘价
	MaxPct    float64      `json:"max_pct"`    // 窗口内最高价收益率(%)
	MinPct    float64      `json:"min_pct"`    // 窗口内最低价收益率(%)
	Returns   []ForwardDay `json:"returns"`    // 选股日之后的逐日表现
}

// PickDayResult 单个交易日的选股结果
type PickDayResult struct {
	Date      string      `json:"date"`       // 交易日 YYYY-MM-DD
	Scanned   int         `json:"scanned"`    // 当日参与筛选的股票数
	PickCount int         `json:"pick_count"` // 当日命中数量
	Picks     []StockPick `json:"picks"`      // 当日命中的股票明细
}

// StrategyPicksService 负责策略选股复盘：
// 按交易日逐个回放策略的选股结果，并计算选中股票之后若干交易日的 OHLC 收益。
type StrategyPicksService struct {
	screenSvc *ScreenService
	registry  *indicator.Registry
}

// NewStrategyPicksService 创建选股复盘服务。
//
// 参数:
//   - screenSvc: 用于构建每个交易日的股票池
//   - registry: 指标注册表，提供选股引擎
func NewStrategyPicksService(screenSvc *ScreenService, registry *indicator.Registry) *StrategyPicksService {
	return &StrategyPicksService{screenSvc: screenSvc, registry: registry}
}

// prepare 校验策略与参数，返回归一化后的查询条件与信号配置。
func (s *StrategyPicksService) prepare(q StrategyPicksQuery) (StrategyPicksQuery, []*indicator.SignalConfig, error) {
	if q.Strategy == nil {
		return q, nil, errors.New("策略为空")
	}
	configs, err := parseStrategyConditions(q.Strategy.Conditions)
	if err != nil {
		return q, nil, err
	}
	holdDays := q.HoldDays
	if holdDays <= 0 {
		holdDays = DefaultPickHoldDays
	}
	if holdDays > MaxPickHoldDays {
		holdDays = MaxPickHoldDays
	}
	concurrency := q.MaxConcurrency
	if concurrency <= 0 {
		concurrency = 10
	}
	out := StrategyPicksQuery{
		Strategy:       q.Strategy,
		StartDate:      q.StartDate,
		EndDate:        q.EndDate,
		HoldDays:       holdDays,
		MaxConcurrency: concurrency,
	}
	return out, configs, nil
}

// TradingDays 列出 [startDate, endDate] 区间内需要回放的 A 股交易日（YYYY-MM-DD 升序）。
//
// 周末与法定节假日一律跳过，供前端按天逐个请求后自行汇总。
func (s *StrategyPicksService) TradingDays(startDate, endDate string) ([]string, error) {
	start, err := utils.ParseDateToTradeDate(startDate)
	if err != nil {
		return nil, fmt.Errorf("起始日期无效: %w", err)
	}
	end, err := utils.ParseDateToTradeDate(endDate)
	if err != nil {
		return nil, fmt.Errorf("结束日期无效: %w", err)
	}
	if start > end {
		return nil, errors.New("起始日期不能晚于结束日期")
	}

	dates, err := db.FindDailyTradeDatesInRange(start, end, 0)
	if err != nil {
		return nil, fmt.Errorf("查询交易日失败: %w", err)
	}
	dates = filterTradingDates(dates)

	out := make([]string, 0, len(dates))
	for _, d := range dates {
		out = append(out, db.FormatTradeDate(d))
	}
	return out, nil
}

// AnalyzeDay 只回放单个交易日的选股结果，用于前端按天轮询、逐步汇总。
//
// date 必须是 A 股交易日（YYYY-MM-DD），否则返回错误。
func (s *StrategyPicksService) AnalyzeDay(ctx context.Context, q StrategyPicksQuery, date string) (*PickDayResult, error) {
	query, configs, err := s.prepare(q)
	if err != nil {
		return nil, err
	}
	tradeDate, err := utils.ParseDateToTradeDate(date)
	if err != nil {
		return nil, fmt.Errorf("日期无效: %w", err)
	}
	if !isTradingDate(tradeDate) {
		return nil, fmt.Errorf("%s 不是交易日", db.FormatTradeDate(tradeDate))
	}
	return s.analyzeDay(query, tradeDate, configs)
}

// analyzeDay 回放单个交易日的选股，并计算入选股票的后续走势。
func (s *StrategyPicksService) analyzeDay(q StrategyPicksQuery, tradeDate int, configs []*indicator.SignalConfig) (*PickDayResult, error) {
	dateStr := db.FormatTradeDate(tradeDate)
	day := &PickDayResult{Date: dateStr}

	stocks, err := s.screenSvc.BuildAll(q.MaxConcurrency, dateStr)
	if err != nil {
		return nil, fmt.Errorf("构建股票池失败: %w", err)
	}
	day.Scanned = len(stocks)
	if len(stocks) == 0 {
		return day, nil
	}

	passed := filterPassedStocks(s.registry.Engine().Execute(stocks, configs, q.MaxConcurrency))
	day.PickCount = len(passed)
	if len(passed) == 0 {
		return day, nil
	}

	picks := make([]StockPick, len(passed))
	if err := utils.ConcurrentExec(passed, PickQuoteConcurrency, func(i int, stock indicator.EvaluatedStock) error {
		pick := StockPick{
			Date:     dateStr,
			Code:     stock.Code,
			Name:     stock.Name,
			SignalID: stock.SignalID,
			Message:  stock.Message,
		}
		forwards, baseClose := BuildForwardReturns(stock.Code, tradeDate, q.HoldDays)
		pick.BaseClose = baseClose
		pick.Returns = forwards
		pick.MaxPct, pick.MinPct = extremes(forwards)
		picks[i] = pick
		return nil
	}); err != nil {
		return nil, fmt.Errorf("拉取入选股票后续走势失败: %w", err)
	}
	day.Picks = picks

	return day, nil
}

// BuildForwardReturns 计算某只股票自 signalDate（含）起之后 holdDays 个交易日的 OHLC 收益率。
//
// 窗口以该股票自己的选股日为起点独立计算：[signalDate, signalDate + holdDays 个交易日]，
// 中间遇到周末/节假日直接跳过，不占用观察天数（offset 只按交易日递增）。
// 返回逐日表现（不含信号日，单位 %）与基准价（元，信号日收盘价）。
// 无数据时返回空切片，baseClose 为 0。
func BuildForwardReturns(code string, signalDate, holdDays int) ([]ForwardDay, float64) {
	if holdDays <= 0 {
		holdDays = DefaultPickHoldDays
	}
	klines, err := db.FindDailyKlinesFromDate(code, signalDate, holdDays+1)
	if err != nil || len(klines) == 0 || klines[0].TradeDate != signalDate || klines[0].Close <= 0 {
		return nil, 0
	}

	base := klines[0].Close // 单位：分
	items := make([]ForwardDay, 0, len(klines)-1)
	offset := 0
	for _, k := range klines[1:] {
		if !isTradingDate(k.TradeDate) {
			continue // 节假日/周末不占用观察天数
		}
		offset++
		items = append(items, ForwardDay{
			Offset:   offset,
			Date:     db.FormatTradeDate(k.TradeDate),
			OpenPct:  returnPct(k.Open, base),
			HighPct:  returnPct(k.High, base),
			LowPct:   returnPct(k.Low, base),
			ClosePct: returnPct(k.Close, base),
		})
	}
	return items, centsToYuan(base)
}

// isTradingDate 判断 YYYYMMDD 日期是否为 A 股交易日（排除周末与法定节假日）。
func isTradingDate(tradeDate int) bool {
	ok, err := utils.IsTradingDayForDate(db.FormatTradeDate(tradeDate))
	if err != nil {
		return false
	}
	return ok
}

// filterTradingDates 过滤掉非 A 股交易日（周末 + 法定节假日）。
//
// daily_kline 本身只在交易日有数据，这里再做一次防御性过滤，
// 避免脏数据导致复盘把节假日当成交易日回放。
func filterTradingDates(dates []int) []int {
	if len(dates) == 0 {
		return dates
	}
	kept := make([]int, 0, len(dates))
	for _, d := range dates {
		if isTradingDate(d) {
			kept = append(kept, d)
		}
	}
	return kept
}

// returnPct 计算相对基准价的收益率(%)，入参均为「分」计的价格。
func returnPct(priceCents, baseCents int) float64 {
	if baseCents <= 0 {
		return 0
	}
	return (float64(priceCents)/float64(baseCents) - 1) * 100
}

// centsToYuan 将「分」计的价格转为「元」。
func centsToYuan(cents int) float64 {
	return float64(cents) / 100
}

// extremes 取窗口内最高价收益率与最低价收益率。
func extremes(items []ForwardDay) (maxPct, minPct float64) {
	if len(items) == 0 {
		return 0, 0
	}
	maxPct, minPct = items[0].HighPct, items[0].LowPct
	for _, it := range items[1:] {
		if it.HighPct > maxPct {
			maxPct = it.HighPct
		}
		if it.LowPct < minPct {
			minPct = it.LowPct
		}
	}
	return maxPct, minPct
}

// filterPassedStocks 提取评估结果为 Passed 的股票，按代码升序排列。
func filterPassedStocks(results []*indicator.EvaluatedStock) []indicator.EvaluatedStock {
	passed := make([]indicator.EvaluatedStock, 0, len(results))
	for _, r := range results {
		if r != nil && r.Result == indicator.ResultPassed {
			passed = append(passed, *r)
		}
	}
	sort.Slice(passed, func(i, j int) bool { return passed[i].Code < passed[j].Code })
	return passed
}

// parseStrategyConditions 将策略 conditions JSON 解析为引擎所需的信号配置列表。
func parseStrategyConditions(conditionsJSON string) ([]*indicator.SignalConfig, error) {
	if conditionsJSON == "" {
		return nil, errors.New("策略未配置任何信号")
	}
	var configs []*indicator.SignalConfig
	if err := json.Unmarshal([]byte(conditionsJSON), &configs); err != nil {
		return nil, fmt.Errorf("解析策略条件失败: %w", err)
	}
	valid := make([]*indicator.SignalConfig, 0, len(configs))
	for _, cfg := range configs {
		if cfg != nil && cfg.SignalID != "" {
			valid = append(valid, cfg)
		}
	}
	if len(valid) == 0 {
		return nil, errors.New("策略未配置有效信号")
	}
	return valid, nil
}
