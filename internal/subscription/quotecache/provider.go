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
)

// ============================================================================
//  CachedStock 缓存增强版股票数据源
//
//  嵌入 DBStock，覆盖 GetDailySnapshot 返回缓存的今日行情，
//  其余方法（K线、财报等）委托给 DBStock 从数据库读取。
// ============================================================================

// 确保 CachedStock 实现 indicator.StockSource
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
		tradeDate: 0, // 由 Provider 在构建时设置
	}
}

// SetTradeDate 设置交易日期（由 Provider 在构建时调用）
func (c *CachedStock) SetTradeDate(tradeDate int) {
	c.tradeDate = tradeDate
}

// GetDailySnapshot 覆盖 DBStock 的方法，优先从 QuoteCache 获取今日行情
// 转换 adapter.StockPriceDaily 为 model.StockDailySnapshot 格式返回
func (c *CachedStock) GetDailySnapshot() (*model.StockDailySnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	price, err := c.cache.Get(ctx, c.code)
	if err != nil {
		// 缓存获取失败，fallback 到 DBStock 的原始实现
		return c.DBStock.GetDailySnapshot()
	}

	return convertToSnapshot(price), nil
}

// GetDailyKline 覆盖 GetDailyKline，拼接今日行情到 K线头部
// 使技术指标 MA/MACD 等能取到最新价格
func (c *CachedStock) GetDailyKline() ([]*model.DailyKline, error) {
	// 先获取 DB 历史K线
	klines, err := c.DBStock.GetDailyKline()
	if err != nil {
		return nil, err
	}

	// 从缓存获取今日行情
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	price, err := c.cache.Get(ctx, c.code)
	if err != nil {
		// 缓存获取失败，直接返回历史K线
		return klines, nil
	}

	// 检查是否需要拼接（最新K线日期 < 今日日期）
	if len(klines) > 0 && price.Date != "" {
		latestDateStr := fmt.Sprintf("%08d", klines[0].TradeDate)
		if price.Date > latestDateStr {
			// 将今日行情转换为 DailyKline 并拼接到头部
			todayKline := convertToDailyKline(price)
			result := make([]*model.DailyKline, 0, len(klines)+1)
			result = append(result, todayKline)
			result = append(result, klines...)
			return result, nil
		}
	}

	return klines, nil
}

// convertToSnapshot 将 adapter.StockPriceDaily 转换为 model.StockDailySnapshot
func convertToSnapshot(price *adapter.StockPriceDaily) *model.StockDailySnapshot {
	return &model.StockDailySnapshot{
		StockCode:    price.Code,
		TradeDate:    parseDateToInt(price.Date),
		PETTM:        price.Pe,
		PB:           price.Pb,
		TotalMarketCap: price.MarketCap * 1e8, // 亿 → 元（近似转换）
	}
}

// convertToDailyKline 将 adapter.StockPriceDaily 转换为 model.DailyKline
func convertToDailyKline(price *adapter.StockPriceDaily) *model.DailyKline {
	return &model.DailyKline{
		StockCode:    price.Code,
		TradeDate:    parseDateToInt(price.Date),
		Open:         int(price.Open),
		High:         int(price.High),
		Low:          int(price.Low),
		Close:        int(price.Close),
		Volume:       price.Volume,
		Amount:       price.Amount,
		TurnoverRate: price.Turnover,
	}
}

// parseDateToInt 将日期字符串 "2025-07-09" 转换为整数 20250709
func parseDateToInt(dateStr string) int {
	if len(dateStr) < 10 {
		return 0
	}
	// 去掉连字符: "2025-07-09" → "20250709"
	clean := dateStr[0:4] + dateStr[5:7] + dateStr[8:10]
	val, err := strconv.Atoi(clean)
	if err != nil {
		return 0
	}
	return val
}
