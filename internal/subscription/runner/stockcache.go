package runner

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"stock-ai/internal/indicator"
	stocksource "stock-ai/internal/indicator/stocksource"
	"stock-ai/internal/model"
	"stock-ai/internal/subscription/quotecache"
	"stock-ai/utils"
)

// ============================================================================
//  CachedStock 缓存增强版股票数据源
//
//  嵌入 DBStock，覆盖 K线/Snapshot 方法实现缓存优先策略：
//  - DB 历史K线 + quotecache 缓存当期数据（拼接至头部）
//  - GetDailySnapshot: quotecache 当日行情 + DB 财报/股本
//  - 其余方法（PerformanceReport、ShareholderCount、Detail 等）委托 DBStock
// ============================================================================

var _ indicator.StockSource = (*CachedStock)(nil)

// CachedStock 缓存增强版股票数据源
type CachedStock struct {
	*stocksource.DBStock
	cache     quotecache.QuoteCache
	code      string
	tradeDate int
}

// NewCachedStock 创建 CachedStock 实例
func NewCachedStock(base *stocksource.DBStock, cache quotecache.QuoteCache) *CachedStock {
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

// GetDailySnapshot 覆盖 DBStock，使用缓存元数据中的快照。
//
// 快照由上层 OnQuoteReady hook 预计算并存入 data.Meta["snapshot"]。
// 无快照时降级到 DBStock。
func (c *CachedStock) GetDailySnapshot() (*model.StockDailySnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := c.cache.Get(ctx, c.code)
	if err == nil && data.Meta != nil {
		if snap, ok := data.Meta["snapshot"].(*model.StockDailySnapshot); ok && snap != nil {
			return snap, nil
		}
	}

	return c.DBStock.GetDailySnapshot()
}

// GetDailyKline 覆盖 DBStock，拼接缓存当日行情到日K线头部。
//
// 交易时段(9:00-19:00) → 从 cache 取当日行情拼接到 K 线头部。
// 非交易时段 → 直接读 DB（不拼接实时行情）。
func (c *CachedStock) GetDailyKline() ([]*model.DailyKline, error) {
	klines, err := c.DBStock.GetDailyKline()
	if err != nil {
		return nil, err
	}

	if !isCacheWindow() {
		return klines, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := c.cache.Get(ctx, c.code)
	if err != nil {
		return klines, nil
	}
	daily := data.Daily()
	if daily == nil {
		return klines, nil
	}

	return appendKlineHead(klines, daily, convertToDailyKline), nil
}

// ============================================================================
//  CachedQuoteProvider 实现 StockSourceProvider
// ============================================================================

var _ StockSourceProvider = (*CachedQuoteProvider)(nil)

// CachedQuoteProvider 带行情缓存的 StockSource 构建器
type CachedQuoteProvider struct {
	cache quotecache.QuoteCache
}

// NewCachedQuoteProvider 创建 CachedQuoteProvider 实例
func NewCachedQuoteProvider(cache quotecache.QuoteCache) StockSourceProvider {
	return &CachedQuoteProvider{cache: cache}
}

// BuildStockSources 实现 StockSourceProvider 接口
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
//  转换函数
// ============================================================================

func convertToDailyKline(p *quotecache.StockPriceDaily) *model.DailyKline {
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

// appendKlineHead 将缓存数据拼接到 K 线头部。
func appendKlineHead[T any](klines []*T, price *quotecache.StockPriceDaily, convert func(*quotecache.StockPriceDaily) *T) []*T {
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
		result := make([]*T, 0, len(klines)+1)
		result = append(result, item)
		result = append(result, klines...)
		return result
	}

	// priceDate == latestDate：覆盖第一条
	result := make([]*T, len(klines))
	result[0] = item
	copy(result[1:], klines[1:])
	return result
}

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

func isCacheWindow() bool {
	now := time.Now()
	hour := now.Hour()
	if hour < 9 || hour >= 19 {
		return false
	}
	return utils.IsTradingDay()
}
