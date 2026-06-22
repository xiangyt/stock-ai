package ths2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"stock-ai/internal/adapter"
)

// API请求体结构
type klineRequest struct {
	CodeList   []codeListItem `json:"code_list"`
	TradeClass string         `json:"trade_class"`
	TimePeriod string         `json:"time_period"`
	TradeDate  int            `json:"trade_date"`
	BeginTime  int            `json:"begin_time"`
	EndTime    int            `json:"end_time"`
	AdjustType string         `json:"adjust_type"`
	GPID       int            `json:"gpid"`
}

type codeListItem struct {
	Codes  []string `json:"codes"`
	Market string   `json:"market"`
}

// API响应体结构
type klineResponse struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
	Data       struct {
		QuoteData  []quoteDataItem `json:"quote_data"`
		FailParams json.RawMessage `json:"fail_params"`
	} `json:"data"`
}

type quoteDataItem struct {
	Market     string      `json:"market"`
	Code       string      `json:"code"`
	Delay      bool        `json:"delay"`
	DataFields []string    `json:"data_fields"`
	Value      [][]float64 `json:"value"`
}

// GetDailyKLine 获取日K线
func (a *Adapter) GetDailyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.getKLines(ctx, code, adjType, TimePeriodDaily)
}

// GetWeeklyKLine 获取周K线
func (a *Adapter) GetWeeklyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.getKLines(ctx, code, adjType, TimePeriodWeekly)
}

// GetMonthlyKLine 获取月K线
func (a *Adapter) GetMonthlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.getKLines(ctx, code, adjType, TimePeriodMonthly)
}

// getKLines 通用K线获取方法
func (a *Adapter) getKLines(ctx context.Context, code, adjType, timePeriod string) ([]adapter.StockPriceDaily, error) {
	market := codeToMarket(code)
	if market == "" {
		return nil, fmt.Errorf("unsupported code: %s", code)
	}

	adjustType := adjTypeToAdjustType(adjType)

	reqBody := klineRequest{
		CodeList: []codeListItem{
			{
				Codes:  []string{code},
				Market: market,
			},
		},
		TradeClass: "intraday",
		TimePeriod: timePeriod,
		TradeDate:  -1,
		BeginTime:  -400,
		EndTime:    0,
		AdjustType: adjustType,
		GPID:       1,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://quota-h.10jqka.com.cn/fuyao/common_hq_aggr/quote/v1/single_kline",
		bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	a.setRequestHeaders(req)
	resp, err := a.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result klineResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.StatusCode != 0 {
		return nil, fmt.Errorf("API error: status_code=%d, msg=%s", result.StatusCode, result.StatusMsg)
	}

	if len(result.Data.QuoteData) == 0 {
		return nil, fmt.Errorf("empty quote_data for code=%s", code)
	}

	return parseQuoteData(code, result.Data.QuoteData[0])
}

// setRequestHeaders 设置请求头
func (a *Adapter) setRequestHeaders(req *http.Request) {
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("origin", "https://stockpage.10jqka.com.cn")
	req.Header.Set("platform", "hxkline")
	req.Header.Set("pragma", "no-cache")
	req.Header.Set("referer", "https://stockpage.10jqka.com.cn/")
	req.Header.Set("sec-ch-ua", `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-site")
	req.Header.Set("source-id", "hxkline-NEWS_appNewsFlowHome_Page")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
	req.Header.Set("x-auth-appname", "AINVEST")
	req.Header.Set("x-auth-progid", "7047")
	req.Header.Set("x-auth-type", "ths")
	req.Header.Set("x-auth-version", "1.0")

	if a.fuyaoAuth != "" {
		req.Header.Set("x-fuyao-auth", a.fuyaoAuth)
	}
	if a.cookie != "" {
		req.Header.Set("cookie", a.cookie)
	}
}

// doRequest 发送请求并检查限流
func (a *Adapter) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := a.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return resp, nil
}

// parseQuoteData 解析 quote_data 返回的K线数据
func parseQuoteData(code string, qd quoteDataItem) ([]adapter.StockPriceDaily, error) {
	// 同花顺 quota-h API 字段定义（与 ths realtime 相同编号体系）:
	//   "1"  — 时间戳(ms)
	//   "7"  — 开盘价(元)    → yuanToCents → 分
	//   "8"  — 最高价(元)    → yuanToCents → 分
	//   "9"  — 最低价(元)    → yuanToCents → 分
	//   "11" — 收盘价(元)    → yuanToCents → 分
	//   "13" — 成交量(手)    → ×100 → 股
	//   "19" — 成交额(万元)  → yuanToCents → 分  (注: yuanToCents 仅 ×100，万元→分 需 PNV)
	// 构建字段名→索引映射，兼容字段顺序变化
	fieldIndex := make(map[string]int)
	for i, f := range qd.DataFields {
		fieldIndex[f] = i
	}

	timeIdx, ok1 := fieldIndex["1"]
	openIdx, ok2 := fieldIndex["7"]
	highIdx, ok3 := fieldIndex["8"]
	lowIdx, ok4 := fieldIndex["9"]
	closeIdx, ok5 := fieldIndex["11"]
	volIdx, ok6 := fieldIndex["13"]
	amountIdx, ok7 := fieldIndex["19"]

	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 || !ok7 {
		return nil, fmt.Errorf("missing required data_fields, got: %v", qd.DataFields)
	}

	result := make([]adapter.StockPriceDaily, 0, len(qd.Value))
	for _, row := range qd.Value {
		if len(row) <= maxIndex(timeIdx, openIdx, highIdx, lowIdx, closeIdx, volIdx, amountIdx) {
			continue
		}

		tsMs := int64(row[timeIdx])
		open := yuanToCents(row[openIdx])
		high := yuanToCents(row[highIdx])
		low := yuanToCents(row[lowIdx])
		closePrice := yuanToCents(row[closeIdx])
		volume := int64(row[volIdx]) // API已返回股
		amount := yuanToCents(row[amountIdx])

		date := time.UnixMilli(tsMs).Format(time.DateOnly)

		result = append(result, adapter.StockPriceDaily{
			Code:   code,
			Date:   date,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closePrice,
			Volume: volume,
			Amount: amount,
		})
	}

	return result, nil
}

// maxIndex 返回最大值
func maxIndex(indices ...int) int {
	max := indices[0]
	for _, v := range indices[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

// yuanToCents 将价格(元)转为分(int64)，四舍五入
// 支持 1~3 位小数的价格（如 5.26, 5.3, 2.841）
func yuanToCents(yuan float64) int64 {
	return int64(math.Round(yuan * 100))
}
