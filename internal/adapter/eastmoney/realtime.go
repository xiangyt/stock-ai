package eastmoney

import (
	"context"
	"time"

	"stock-ai/internal/adapter"
)

// ========== 实时数据（复用K线接口） ==========

// GetTodayData 当日数据 - 拉取近7天日K线，取最后一条(当日)
func (a *Adapter) GetTodayData(ctx context.Context, code string) (*adapter.StockPriceDaily, string, error) {
	beg := time.Now().AddDate(0, 0, -7).Format("20060102")
	klines, err := a.fetchKLines(ctx, code, adapter.AdjQFQ, KLineTypeDaily, beg)
	if err != nil || len(klines) == 0 {
		return nil, "", err
	}
	last := &klines[len(klines)-1]
	detail, _ := a.GetStockDetail(ctx, code)
	name := ""
	if detail != nil {
		name = detail.Name
	}
	return last, name, nil
}

// GetThisWeekData 本周数据 - beg设为本周一
func (a *Adapter) GetThisWeekData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	now := time.Now()
	weekday := now.Weekday()
	var monday time.Time
	if weekday == time.Sunday {
		monday = now.AddDate(0, 0, -6)
	} else {
		monday = now.AddDate(0, 0, -(int(weekday) - 1))
	}
	beg := monday.Format("20060102")
	klines, err := a.fetchKLines(ctx, code, adapter.AdjQFQ, KLineTypeWeekly, beg)
	if err != nil || len(klines) == 0 {
		return nil, err
	}
	return &klines[len(klines)-1], nil
}

// GetThisMonthData 本月数据 - beg设为当月1号
func (a *Adapter) GetThisMonthData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	beg := time.Now().Format("200601") + "01"
	klines, err := a.fetchKLines(ctx, code, adapter.AdjQFQ, KLineTypeMonthly, beg)
	if err != nil || len(klines) == 0 {
		return nil, err
	}
	return &klines[len(klines)-1], nil
}

// GetThisQuarterData 本季数据 - beg设为当季第一天
func (a *Adapter) GetThisQuarterData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	now := time.Now()
	quarterBeg := time.Date(now.Year(), time.Month((int(now.Month()-1)/3)*3+1), 1, 0, 0, 0, 0, now.Location())
	beg := quarterBeg.Format("20060102")
	klines, err := a.fetchKLines(ctx, code, adapter.AdjQFQ, KLineTypeQuarterly, beg)
	if err != nil || len(klines) == 0 {
		return nil, err
	}
	return &klines[len(klines)-1], nil
}

// GetThisYearData 本年数据 - beg设为当年1月1日
func (a *Adapter) GetThisYearData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	beg := time.Now().Format("2006") + "0101"
	klines, err := a.fetchKLines(ctx, code, adapter.AdjQFQ, KLineTypeYearly, beg)
	if err != nil || len(klines) == 0 {
		return nil, err
	}
	return &klines[len(klines)-1], nil
}
