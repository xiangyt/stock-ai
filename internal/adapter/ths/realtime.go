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

func (a *Adapter) GetTodayData(ctx context.Context, code string) (*adapter.StockPriceDaily, string, error) {
	symbol, _, err := a.parseCode(code)
	if err != nil {
		return nil, "", fmt.Errorf("invalid tsCode format: %s", code)
	}
	thsCode := buildTHSCode(symbol)
	if thsCode == "" {
		return nil, "", fmt.Errorf("unsupported market for code: %s", code)
	}

	requestURL := fmt.Sprintf("https://d.10jqka.com.cn/v6/line/%s/%s/defer/today.js", thsCode, KLineTypeDaily)
	body, err := a.makeTodayDataRequest(requestURL)
	if err != nil {
		return nil, "", err
	}

	data, name, err := a.parseTodayDataResponse(code, thsCode, body)
	if err != nil {
		return nil, "", err
	}
	return data, name, nil
}

// GetThisWeekData / GetThisMonthData / GetThisQuarterData / GetThisYearData
func (a *Adapter) GetThisWeekData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	data, _, err := a.getDataByType(code, KLineTypeWeekly)
	return data, err
}

func (a *Adapter) GetThisMonthData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	data, _, err := a.getDataByType(code, KLineTypeMonthly)
	return data, err
}

func (a *Adapter) GetThisQuarterData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	data, _, err := a.getDataByType(code, KLineTypeQuarterly)
	return data, err
}

func (a *Adapter) GetThisYearData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	data, _, err := a.getDataByType(code, KLineTypeYearly)
	return data, err
}

func (a *Adapter) getDataByType(code, klineType string) (*adapter.StockPriceDaily, string, error) {
	symbol, _, err := a.parseCode(code)
	if err != nil {
		return nil, "", err
	}
	thsCode := buildTHSCode(symbol)
	url := fmt.Sprintf("https://d.10jqka.com.cn/v6/line/%s/%s/defer/today.js", thsCode, klineType)
	body, err := a.makeTodayDataRequest(url)
	if err != nil {
		return nil, "", err
	}
	return a.parseTodayDataResponse(code, thsCode, body)
}

// parseTodayDataResponse 解析今日/本周/本月等数据响应
func (a *Adapter) parseTodayDataResponse(tsCode, thsCode, body string) (*adapter.StockPriceDaily, string, error) {
	callbackPrefix := fmt.Sprintf("quotebridge_v6_line_%s_", thsCode)
	startIdx := strings.Index(body, callbackPrefix)
	if startIdx == -1 {
		return nil, "", fmt.Errorf("callback not found")
	}
	parenIdx := strings.Index(body[startIdx:], "(")
	jsonStart := startIdx + parenIdx + 1
	jsonEnd := strings.LastIndex(body, ")")
	if jsonEnd <= jsonStart {
		return nil, "", fmt.Errorf("invalid format")
	}
	jsonStr := body[jsonStart:jsonEnd]

	var response map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		return nil, "", err
	}

	dataMap, ok := response[thsCode].(map[string]interface{})
	if !ok {
		return nil, "", fmt.Errorf("data not found for %s", thsCode)
	}

	tradeDateStr := getString(dataMap, "1")
	openStr := getString(dataMap, "7")
	highStr := getString(dataMap, "8")
	lowStr := getString(dataMap, "9")
	closeStr := getString(dataMap, "11")
	volStr := getString(dataMap, "13")
	amountStr := getString(dataMap, "19")
	name := getString(dataMap, "name")

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
	}, name, nil
}
