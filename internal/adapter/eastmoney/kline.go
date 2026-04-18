package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"stock-ai/internal/adapter"
)

// ========== K线数据 ==========

// KLineResponse 东方财富K线API响应
type KLineResponse struct {
	RC   int `json:"rc"`
	RT   int `json:"rt"`
	Data struct {
		Code   string   `json:"code"`
		Market int      `json:"market"`
		Name   string   `json:"name"`
		Klines []string `json:"klines"`
	} `json:"data"`
}

// GetDailyKLine 获取日K线
func (a *Adapter) GetDailyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.fetchKLines(ctx, code, adjType, KLineTypeDaily, "")
}

// GetWeeklyKLine 获取周K线
func (a *Adapter) GetWeeklyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.fetchKLines(ctx, code, adjType, KLineTypeWeekly, "")
}

// GetMonthlyKLine 获取月K线
func (a *Adapter) GetMonthlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.fetchKLines(ctx, code, adjType, KLineTypeMonthly, "")
}

// GetQuarterlyKLine 获取季K线
func (a *Adapter) GetQuarterlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.fetchKLines(ctx, code, adjType, KLineTypeQuarterly, "")
}

// GetYearlyKLine 获取年K线
func (a *Adapter) GetYearlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.fetchKLines(ctx, code, adjType, KLineTypeYearly, "")
}

// fetchKLines 通用K线获取方法
// beg: 起始日期，格式 YYYYMMDD，空字符串则从上市起("0")
func (a *Adapter) fetchKLines(ctx context.Context, code, adjType, klineType, beg string) ([]adapter.StockPriceDaily, error) {
	symbol, market := parseCode(code)
	secid := buildSecID(symbol, market)
	refer := "https://quote.eastmoney.com"

	if beg == "" {
		beg = "0"
	}

	params := url.Values{
		"fields1": {"f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13"},
		"fields2": {"f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61"},
		"beg":     {beg},
		"ut":      {"fa5fd1943c7b386f172d6893dbfba10b"},
		"rtntype": {"6"},
		"secid":   {secid},
		"klt":     {klineType},
		"fqt":     {adjType},
		"cb":      {fmt.Sprintf("jsonp%d", time.Now().UnixMilli())},
	}

	url := "https://push2his.eastmoney.com/api/qt/stock/kline/get?" + params.Encode()
	body, err := a.makeGetRequest(url, refer)
	if err != nil {
		return nil, err
	}

	jsonStr := extractJSONP(body)
	var response KLineResponse
	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	result := make([]adapter.StockPriceDaily, 0, len(response.Data.Klines))
	for _, kline := range response.Data.Klines {
		parsed, err := a.parser.ParseDailyKline(code, kline)
		if err != nil {
			continue
		}
		result = append(result, *parsed)
	}

	return result, nil
}
