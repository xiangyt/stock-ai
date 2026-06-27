package data

import (
	"context"
	"fmt"

	"stock-ai/internal/db"
	"stock-ai/internal/model"

	"stock-ai/internal/backtest"
)

// DBProvider DataProvider 的 DB 实现。
type DBProvider struct{}

// NewDBProvider 创建 DB 数据提供者。
func NewDBProvider() *DBProvider { return &DBProvider{} }

// FetchSnapshots 批量获取股票行情快照。
func (p *DBProvider) FetchSnapshots(ctx context.Context, codes []string, tradeDate int) ([]*backtest.StockSnapshot, error) {
	if len(codes) == 0 { return nil, nil }
	var klines []model.DailyKline
	if err := db.GetDB().WithContext(ctx).
		Where("stock_code IN ? AND trade_date = ?", codes, tradeDate).
		Find(&klines).Error; err != nil {
		return nil, fmt.Errorf("fetch kline: %w", err)
	}
	codeKline := make(map[string]*model.DailyKline, len(klines))
	for i := range klines { codeKline[klines[i].StockCode] = &klines[i] }
	results := make([]*backtest.StockSnapshot, 0, len(codes))
	for _, code := range codes {
		k, ok := codeKline[code]; if !ok { continue }
		results = append(results, &backtest.StockSnapshot{
			Code: code, Name: code,
			Price: float64(k.Close) / 100.0,
		})
	}
	return results, nil
}
