package quotecache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/db"
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
//  - 其余方法（PerformanceReport、ShareholderCount、Detail 等）委托 DBStock
// ============================================================================

var _ indicator.StockSource = (*CachedStock)(nil)

// CachedStock 缓存增强版股票数据源
type CachedStock struct {
	*stocksource.DBStock // 嵌入：历史数据从 DB 加载
	cache     QuoteCache  // 行情缓存引用
	code      string
	tradeDate int
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
//  MarketSource 覆盖 — 当日行情
// ============================================================================

// GetDailySnapshot 覆盖 DBStock，优先从缓存获取当日行情快照
func (c *CachedStock) GetDailySnapshot() (*model.StockDailySnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := c.cache.Get(ctx, c.code)
	if err != nil || data.Daily == nil {
		return c.DBStock.GetDailySnapshot()
	}

	return convertToSnapshot(data.Daily), nil
}

// ============================================================================
//  TechnicalSource 覆盖 — K线拼接
// ============================================================================

// GetDailyKline 覆盖 DBStock，拼接缓存当日行情到日K线头部
func (c *CachedStock) GetDailyKline() ([]*model.DailyKline, error) {
	klines, err := c.DBStock.GetDailyKline()
	if err != nil {
		return nil, err
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

// GetWeeklyKline 覆盖 DBStock，拼接缓存本周行情到周K线头部
func (c *CachedStock) GetWeeklyKline() ([]*model.WeeklyKline, error) {
	klines, err := c.DBStock.GetWeeklyKline()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := c.cache.Get(ctx, c.code)
	if err != nil || data.Weekly == nil {
		return klines, nil
	}

	return appendKlineHead(klines, data.Weekly, func(p *adapter.StockPriceDaily) *model.WeeklyKline {
		return convertToWeeklyKline(p)
	}), nil
}

// GetMonthlyKline 覆盖 DBStock，拼接缓存本月行情到月K线头部
func (c *CachedStock) GetMonthlyKline() ([]*model.MonthlyKline, error) {
	klines, err := c.DBStock.GetMonthlyKline()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := c.cache.Get(ctx, c.code)
	if err != nil || data.Monthly == nil {
		return klines, nil
	}

	return appendKlineHead(klines, data.Monthly, func(p *adapter.StockPriceDaily) *model.MonthlyKline {
		return convertToMonthlyKline(p)
	}), nil
}

// GetYearlyKline 覆盖 DBStock，拼接缓存本年行情到年K线头部
func (c *CachedStock) GetYearlyKline() ([]*model.YearlyKline, error) {
	klines, err := c.DBStock.GetYearlyKline()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := c.cache.Get(ctx, c.code)
	if err != nil || data.Yearly == nil {
		return klines, nil
	}

	return appendKlineHead(klines, data.Yearly, func(p *adapter.StockPriceDaily) *model.YearlyKline {
		return convertToYearlyKline(p)
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

	utils.ConcurrentExec(codes, 20, func(i int, code string) error {
		stockDetail, err := db.FindStockByCode(code)
		if err != nil {
			return nil // 股票不存在则跳过
		}
		base := stocksource.NewDBStock(&stockDetail, td)
		cached := NewCachedStock(base, p.cache)
		cached.SetTradeDate(td)
		stocks = append(stocks, cached)
		return nil
	})

	return stocks, nil
}

// ============================================================================
//  转换函数
// ============================================================================

// convertToSnapshot 将 adapter.StockPriceDaily 转换为 model.StockDailySnapshot
func convertToSnapshot(price *adapter.StockPriceDaily) *model.StockDailySnapshot {
	return &model.StockDailySnapshot{
		StockCode:     price.Code,
		TradeDate:     parseDateToInt(price.Date),
		PETTM:         price.Pe,
		PB:            price.Pb,
		TotalMarketCap: price.MarketCap * 1e8,
	}
}

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

// appendKlineHead 将 adapter 缓存数据拼接到 K线头部
// 泛型函数：T 为任意 K线类型（DailyKline/WeeklyKline/MonthlyKline/YearlyKline）
//
//	拼接条件：缓存 date > 历史 K 线最新 date
func appendKlineHead[T any](klines []*T, price *adapter.StockPriceDaily, convert func(*adapter.StockPriceDaily) *T) []*T {
	if len(klines) == 0 || price == nil || price.Date == "" {
		return klines
	}

	// 通过 reflect 获取 TradeDate 字段的 int 值进行比较
	latestDate := getTradeDateFromKline(klines[0])
	priceDate := parseDateToInt(price.Date)

	if price.Date > latestDate && priceDate > 0 {
		item := convert(price)
		result := make([]*T, 0, len(klines)+1)
		result = append(result, item)
		result = append(result, klines...)
		return result
	}

	return klines
}

// getTradeDateFromKline 通过反射获取 K 线结构体的 TradeDate 字段
func getTradeDateFromKline(kline interface{}) string {
	switch v := kline.(type) {
	case *model.DailyKline:
		return fmt.Sprintf("%08d", v.TradeDate)
	case *model.WeeklyKline:
		return fmt.Sprintf("%08d", v.TradeDate)
	case *model.MonthlyKline:
		return fmt.Sprintf("%08d", v.TradeDate)
	case *model.YearlyKline:
		return fmt.Sprintf("%08d", v.TradeDate)
	}
	return ""
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
