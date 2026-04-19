package ths

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"stock-ai/internal/adapter"
)

// ========== K线数据 ==========

func (a *Adapter) GetDailyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.getKLines(ctx, code, KLineTypeDaily)
}

func (a *Adapter) GetWeeklyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.getKLines(ctx, code, KLineTypeWeekly)
}

func (a *Adapter) GetMonthlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.getKLines(ctx, code, KLineTypeMonthly)
}

func (a *Adapter) GetQuarterlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.getKLines(ctx, code, KLineTypeQuarterly)
}

func (a *Adapter) GetYearlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.getKLines(ctx, code, KLineTypeYearly)
}

// getKLines 通用K线获取方法 - 同花顺全量数据接口
func (a *Adapter) getKLines(ctx context.Context, code, klineType string) ([]adapter.StockPriceDaily, error) {
	symbol, _, err := a.parseCode(code)
	if err != nil {
		return nil, fmt.Errorf("invalid tsCode format: %s", code)
	}
	thsCode := buildTHSCode(symbol)
	if thsCode == "" {
		return nil, fmt.Errorf("unsupported market for code: %s", code)
	}

	requestURL := fmt.Sprintf("https://d.10jqka.com.cn/v6/line/%s/%s/all.js", thsCode, klineType)

	body, err := a.makeTHSRequest(requestURL)
	if err != nil {
		return nil, err
	}

	dates, prices, volumes := a.parseKLineResponse(body, klineType)
	result, err := a.parser.ParseTHSDailyKline(code, dates, prices, volumes)
	if err != nil {
		return nil, err
	}

	var resultData []adapter.StockPriceDaily
	for _, p := range result {
		resultData = append(resultData, adapter.StockPriceDaily{
			Code:   p.Code,
			Date:   p.Date,
			Open:   p.Open,
			High:   p.High,
			Low:    p.Low,
			Close:  p.Close,
			Volume: p.Volume,
			Amount: p.Amount,
		})
	}
	return resultData, nil
}

// parseKLineResponse 解析同花顺K线全量数据响应
func (a *Adapter) parseKLineResponse(body, klineType string) ([]string, []string, []string) {
	callbackName := fmt.Sprintf("quotebridge_v6_line_%s_%s_all(", "placeholder", klineType)
	res := body
	if strings.Contains(res, callbackName) {
		res = strings.TrimPrefix(res, callbackName)
		res = strings.TrimSuffix(res, ")")
	}

	var response struct {
		Start    string  `json:"start"`
		SortYear [][]int `json:"sortYear"`
		Price    string  `json:"price"`
		Volume   string  `json:"volumn"`
		Dates    string  `json:"dates"`
	}

	if err := json.Unmarshal([]byte(res), &response); err != nil {
		return nil, nil, nil
	}

	prices := strings.Split(response.Price, ",")
	volumes := strings.Split(response.Volume, ",")
	dates := strings.Split(response.Dates, ",")
	return dates, prices, volumes
}
