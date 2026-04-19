package ths

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"stock-ai/internal/adapter"
)

// ========== K线数据 ==========

// GetDailyKLine 获取日K线
func (a *Adapter) GetDailyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.getKLines(ctx, code, adjType, KLineTypeDaily)
}

// GetWeeklyKLine 获取周K线
func (a *Adapter) GetWeeklyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.getKLines(ctx, code, adjType, KLineTypeWeekly)
}

// GetMonthlyKLine 获取月K线
func (a *Adapter) GetMonthlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.getKLines(ctx, code, adjType, KLineTypeMonthly)
}

// GetQuarterlyKLine 获取季K线
func (a *Adapter) GetQuarterlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.getKLines(ctx, code, adjType, KLineTypeQuarterly)
}

// GetYearlyKLine 获取年K线
func (a *Adapter) GetYearlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.getKLines(ctx, code, adjType, KLineTypeYearly)
}

// getKLines 通用K线获取方法 — 同花顺全量数据接口
//
// 核心逻辑：
// 1. 拼接 kline 参数 = 周期 + 复权bit（如 "01"=日前复权, "02"=日后复权, "00"=日不复权）
// 2. 请求同花顺全量 K 线接口，解析 JSONP 响应
func (a *Adapter) getKLines(ctx context.Context, code, adjType, klineType string) ([]adapter.StockPriceDaily, error) {
	symbol, _, err := a.parseCode(code)
	if err != nil {
		return nil, fmt.Errorf("invalid tsCode format: %s", code)
	}
	thsCode := buildTHSCode(symbol)
	if thsCode == "" {
		return nil, fmt.Errorf("unsupported market for code: %s", code)
	}

	klineType += adjType
	requestURL := fmt.Sprintf("https://d.10jqka.com.cn/v6/line/%s/%s/all.js", thsCode, klineType)

	body, err := a.makeTHSRequest(requestURL)
	if err != nil {
		return nil, err
	}

	out, err := a.parseKLineResponse(code, thsCode, klineType, body)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// parseKLineResponse 解析同花顺K线响应数据
func (a *Adapter) parseKLineResponse(code, thsCode, klineType, res string) ([]adapter.StockPriceDaily, error) {
	// 同花顺返回的是JavaScript格式，需要提取数据部分
	// 示例格式: quotebridge_v6_line_hs_001208_01_all({"data":"20240101,10.5,10.8,10.2,10.6,1000000;..."})

	// 构建回调函数名
	callbackName := fmt.Sprintf("quotebridge_v6_line_%s_%s_all(", thsCode, klineType)

	res = strings.TrimPrefix(res, callbackName)
	res = strings.TrimSuffix(res, ")")

	// 解析JSON
	var response struct {
		Start    string  `json:"start"`
		SortYear [][]int `json:"sortYear"`
		Price    string  `json:"price"`
		Volume   string  `json:"volumn"`
		Dates    string  `json:"dates"`
	}

	if err := json.Unmarshal([]byte(res), &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	prices := strings.Split(response.Price, ",")
	volumes := strings.Split(response.Volume, ",")
	dates := strings.Split(response.Dates, ",")

	// 解析K线数据
	var klineData = make([]adapter.StockPriceDaily, 0, len(dates))
	var index int
	for _, arr := range response.SortYear {
		year, num := arr[0], arr[1]
		for num > 0 && len(dates) > index && len(prices) > index*4 && len(volumes) > index {
			var data = adapter.StockPriceDaily{
				Code:   code,
				Volume: 0,
				Amount: 0,
			}

			td, _ := time.ParseInLocation("20060102", fmt.Sprintf("%d%s", year, dates[index]), time.Local)
			data.Date = td.Format(time.DateOnly)
			low, _ := strconv.Atoi(prices[index*4])
			open, _ := strconv.Atoi(prices[index*4+1])
			high, _ := strconv.Atoi(prices[index*4+2])
			over, _ := strconv.Atoi(prices[index*4+3])
			volume, _ := strconv.ParseInt(volumes[index], 10, 64)

			data.Low = int64(low)
			data.Open = int64(low + open)
			data.High = int64(low + high)
			data.Close = int64(low + over)
			data.Volume = volume

			klineData = append(klineData, data)

			num--
			index++
		}
	}

	return klineData, nil
}
