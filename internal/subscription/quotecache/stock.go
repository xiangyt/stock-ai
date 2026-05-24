package quotecache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/indicator"
	stocksource "stock-ai/internal/indicator/stocksource"
	"stock-ai/internal/model"
	"stock-ai/internal/subscription/runner"
	"stock-ai/utils"
)

// ============================================================================
//  CachedStock 缓存增强版股票数据源
//
//  嵌入 DBStock，覆盖 K线/Snapshot 方法实现缓存优先策略：
//  - DB 历史K线 + adapter 缓存当期数据（拼接至头部）
//  - GetDailySnapshot: cache 当日行情 + DB 财报/股本 → 完整实时快照
//  - 其余方法（PerformanceReport、ShareholderCount、Detail 等）委托 DBStock
// ============================================================================

var _ indicator.StockSource = (*CachedStock)(nil)

// CachedStock 缓存增强版股票数据源
type CachedStock struct {
	*stocksource.DBStock            // 嵌入：历史数据从 DB 加载
	cache                QuoteCache // 行情缓存引用
	code                 string
	tradeDate            int
}

// NewCachedStock 创建 CachedStock 实例
func NewCachedStock(base *stocksource.DBStock, cache QuoteCache) *CachedStock {
	return &CachedStock{
		DBStock:   base,
		cache:     cache,
		code:      base.GetCode(),
		tradeDate: 0,
	}
}

// SetTradeDate 设置交易日期（由 Provider 在构建时调用）
func (c *CachedStock) SetTradeDate(tradeDate int) {
	c.tradeDate = tradeDate
}

// ============================================================================
//  MarketSource 覆盖 — 实时行情快照（完整计算）
//
//  流程：cache.Get(当日行情) → DBStock.GetPerformanceReport(财报)
//       → DBStock.GetShareChanges(股本) → BuildRealtimeSnapshot
// ============================================================================

// GetDailySnapshot 覆盖 DBStock，使用缓存快照（预计算于 fetchAndCache / 19:00 盘后刷新）
//
// 流程：优先从 cache 读 data.Snapshot；读取失败或无快照时降级到 DBStock。
// 原因：快照在第二天凌晨3点才入库，非交易时段 cache 中可能仍有昨日的有效快照。
func (c *CachedStock) GetDailySnapshot() (*model.StockDailySnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := c.cache.Get(ctx, c.code)
	if err == nil && data.Snapshot != nil {
		return data.Snapshot, nil
	}

	// 缓存未命中或无快照数据，降级到 DBStock
	return c.DBStock.GetDailySnapshot()
}

// ============================================================================
//  TechnicalSource 覆盖 — K线拼接（不变）
// ============================================================================

// GetDailyKline 覆盖 DBStock，拼接缓存当日行情到日K线头部
//
// 交易时段(9:00-19:00) → 从 cache 取当日行情拼接到 K 线头部（18:00 后开始有当天缓存数据）
//  非交易时段 → 直接读 DB（不拼接实时行情）
func (c *CachedStock) GetDailyKline() ([]*model.DailyKline, error) {
	klines, err := c.DBStock.GetDailyKline()
	if err != nil {
		return nil, err
	}

	// 非缓存窗口（非交易日 或 19:00 之后）直接返回历史K线
	if !isCacheWindow() {
		return klines, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := c.cache.Get(ctx, c.code)
	if err != nil || data.Daily == nil {
		return klines, nil
	}

	return appendKlineHead(klines, data.Daily, func(p *adapter.StockPriceDaily) *model.DailyKline {
		return convertToDailyKline(p)
	}), nil
}

// ============================================================================
//  CachedQuoteProvider 实现 runner.StockSourceProvider
//
//  组合 QuoteCache，为 Runner 提供构建 []indicator.StockSource 的能力。
// ============================================================================

var _ runner.StockSourceProvider = (*CachedQuoteProvider)(nil)

// CachedQuoteProvider 带行情缓存的 StockSource 构建器
type CachedQuoteProvider struct {
	cache QuoteCache
}

// NewCachedQuoteProvider 创建 CachedQuoteProvider 实例
func NewCachedQuoteProvider(cache QuoteCache) runner.StockSourceProvider {
	return &CachedQuoteProvider{cache: cache}
}

// BuildStockSources 实现 runner.StockSourceProvider 接口
func (p *CachedQuoteProvider) BuildStockSources(ctx context.Context, codes []string) ([]indicator.StockSource, error) {
	stocks := make([]indicator.StockSource, 0, len(codes))

	tradeDate := time.Now().Format("20060102")
	tradeDateInt := 0
	fmt.Sscanf(tradeDate, "%d", &tradeDateInt)
	td := tradeDateInt

	for _, code := range codes {
		base := stocksource.NewDBStockByCode(code, td)
		cached := NewCachedStock(base, p.cache)
		cached.SetTradeDate(td)
		stocks = append(stocks, cached)
	}
	return stocks, nil
}

// ============================================================================
//  转换函数（轻量转换，无财报计算）
// ============================================================================

// convertToDailyKline 将 adapter.StockPriceDaily 转换为 model.DailyKline
func convertToDailyKline(p *adapter.StockPriceDaily) *model.DailyKline {
	return &model.DailyKline{
		StockCode:    p.Code,
		TradeDate:    parseDateToInt(p.Date),
		Open:         int(p.Open),
		High:         int(p.High),
		Low:          int(p.Low),
		Close:        int(p.Close),
		Volume:       p.Volume,
		Amount:       p.Amount,
		TurnoverRate: p.Turnover,
	}
}

// convertToWeeklyKline 将 adapter.StockPriceDaily 转换为 model.WeeklyKline
func convertToWeeklyKline(p *adapter.StockPriceDaily) *model.WeeklyKline {
	return &model.WeeklyKline{
		StockCode:    p.Code,
		TradeDate:    parseDateToInt(p.Date),
		Open:         int(p.Open),
		High:         int(p.High),
		Low:          int(p.Low),
		Close:        int(p.Close),
		Volume:       p.Volume,
		Amount:       p.Amount,
		TurnoverRate: p.Turnover,
	}
}

// convertToMonthlyKline 将 adapter.StockPriceDaily 转换为 model.MonthlyKline
func convertToMonthlyKline(p *adapter.StockPriceDaily) *model.MonthlyKline {
	return &model.MonthlyKline{
		StockCode:    p.Code,
		TradeDate:    parseDateToInt(p.Date),
		Open:         int(p.Open),
		High:         int(p.High),
		Low:          int(p.Low),
		Close:        int(p.Close),
		Volume:       p.Volume,
		Amount:       p.Amount,
		TurnoverRate: p.Turnover,
	}
}

// convertToYearlyKline 将 adapter.StockPriceDaily 转换为 model.YearlyKline
func convertToYearlyKline(p *adapter.StockPriceDaily) *model.YearlyKline {
	return &model.YearlyKline{
		StockCode:    p.Code,
		TradeDate:    parseDateToInt(p.Date),
		Open:         int(p.Open),
		High:         int(p.High),
		Low:          int(p.Low),
		Close:        int(p.Close),
		Volume:       p.Volume,
		Amount:       p.Amount,
		TurnoverRate: p.Turnover,
	}
}

// appendKlineHead 将 adapter 缓存数据拼接到 K 线头部
// 泛型函数：T 为任意 K 线类型（DailyKline/WeeklyKline/MonthlyKline/YearlyKline）
//
//	拼接条件：
//	  - priceDate > latestDate: 缓存数据更新，插入到头部
//	  - priceDate == latestDate: 日期相同，用缓存数据覆盖历史 K 线第一条（盘后 DB 刷新场景）
//	  - priceDate < latestDate: 缓存数据更旧，不处理
func appendKlineHead[T any](klines []*T, price *adapter.StockPriceDaily, convert func(*adapter.StockPriceDaily) *T) []*T {
	if len(klines) == 0 || price == nil || price.Date == "" {
		return klines
	}

	latestDate := getTradeDateFromKline(klines[0])
	priceDate := parseDateToInt(price.Date)

	if priceDate <= 0 || priceDate < latestDate {
		return klines
	}

	item := convert(price)
	if priceDate > latestDate {
		// 缓存更新：插入到头部
		result := make([]*T, 0, len(klines)+1)
		result = append(result, item)
		result = append(result, klines...)
		return result
	}

	// priceDate == latestDate：覆盖第一条（19:00 盘后刷新场景）
	result := make([]*T, len(klines))
	result[0] = item
	copy(result[1:], klines[1:])
	return result
}

// getTradeDateFromKline 通过反射获取 K 线结构体的 TradeDate 字段的整数值
func getTradeDateFromKline(kline interface{}) int {
	switch v := kline.(type) {
	case *model.DailyKline:
		return v.TradeDate
	case *model.WeeklyKline:
		return v.TradeDate
	case *model.MonthlyKline:
		return v.TradeDate
	case *model.YearlyKline:
		return v.TradeDate
	}
	return 0
}

// parseDateToInt 将日期字符串 "2025-07-09" 转换为整数 20250709
func parseDateToInt(dateStr string) int {
	if len(dateStr) < 10 {
		return 0
	}
	clean := dateStr[0:4] + dateStr[5:7] + dateStr[8:10]
	val, err := strconv.Atoi(clean)
	if err != nil {
		return 0
	}
	return val
}

// isCacheWindow 判断当前是否在缓存有效窗口内
//
// 条件：必须是交易日（排除周末+节假日）且时间在 9:00-19:00 之间。
// 满足条件时使用行情缓存，否则直读 DB 避免读到过期数据。
func isCacheWindow() bool {
	now := time.Now()
	hour := now.Hour()

	// 时间必须在 9:00 ~ 19:00（19:00 之前）
	if hour < 9 || hour >= 19 {
		return false
	}

	// 必须是交易日
	return utils.IsTradingDay()
}
