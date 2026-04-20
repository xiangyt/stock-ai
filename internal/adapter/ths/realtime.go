package ths

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"stock-ai/internal/adapter"
)

// ========== 实时/当日数据 ==========

func (a *Adapter) GetTodayData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	return a.getDataByType(ctx, code, KLineTypeDaily)
}

// GetThisWeekData 本周数据
func (a *Adapter) GetThisWeekData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	return a.getDataByType(ctx, code, KLineTypeWeekly)
}

// GetThisMonthData 本月数据
func (a *Adapter) GetThisMonthData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	return a.getDataByType(ctx, code, KLineTypeMonthly)
}

// GetThisQuarterData 本季数据
func (a *Adapter) GetThisQuarterData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	return a.getDataByType(ctx, code, KLineTypeQuarterly)
}

// GetThisYearData 本年数据
func (a *Adapter) GetThisYearData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	return a.getDataByType(ctx, code, KLineTypeYearly)
}

func (a *Adapter) getDataByType(ctx context.Context, code, klineType string) (*adapter.StockPriceDaily, error) {
	symbol, _, err := a.parseCode(code)
	if err != nil {
		return nil, fmt.Errorf("invalid tsCode format: %s", code)
	}
	thsCode := buildTHSCode(symbol)
	if thsCode == "" {
		return nil, fmt.Errorf("unsupported market for code: %s", code)
	}
	url := fmt.Sprintf("https://d.10jqka.com.cn/v6/line/%s/%s%s/defer/today.js",
		thsCode, klineType, adapter.AdjQFQ)
	body, err := a.makeTodayDataRequest(url)
	if err != nil {
		return nil, err
	}
	return a.parseTodayDataResponse(code, thsCode, body)
}

// parseTodayDataResponse 解析今日/本周/本月等数据响应
func (a *Adapter) parseTodayDataResponse(tsCode, thsCode, body string) (*adapter.StockPriceDaily, error) {
	callbackPrefix := fmt.Sprintf("quotebridge_v6_line_%s_", thsCode)
	startIdx := strings.Index(body, callbackPrefix)
	if startIdx == -1 {
		return nil, fmt.Errorf("callback not found")
	}
	parenIdx := strings.Index(body[startIdx:], "(")
	jsonStart := startIdx + parenIdx + 1
	jsonEnd := strings.LastIndex(body, ")")
	if jsonEnd <= jsonStart {
		return nil, fmt.Errorf("invalid format")
	}
	jsonStr := body[jsonStart:jsonEnd]

	var response map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		return nil, err
	}

	dataMap, ok := response[thsCode].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("data not found for %s", thsCode)
	}

	tradeDateStr := getString(dataMap, "1")
	openStr := getString(dataMap, "7")
	highStr := getString(dataMap, "8")
	lowStr := getString(dataMap, "9")
	closeStr := getString(dataMap, "11")
	volStr := getString(dataMap, "13")
	amountStr := getString(dataMap, "19")

	td, _ := strconv.Atoi(tradeDateStr)

	return &adapter.StockPriceDaily{
		Code:   tsCode,
		Date:   formatTradeDate(td),
		Open:   yuanToCents(parseFloat(openStr)),
		High:   yuanToCents(parseFloat(highStr)),
		Low:    yuanToCents(parseFloat(lowStr)),
		Close:  yuanToCents(parseFloat(closeStr)),
		Volume: parseInt64(volStr),
		Amount: yuanToCents(parseFloat(amountStr)),
	}, nil
}
