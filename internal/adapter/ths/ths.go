package ths

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/helpers"
	"stock-ai/internal/model"

	"golang.org/x/time/rate"
)

// K线类型常量
const (
	KLineTypeDaily     = "01"
	KLineTypeWeekly    = "11"
	KLineTypeMonthly   = "21"
	KLineTypeQuarterly = "91"
	KLineTypeYearly    = "81"
)

// Adapter 同花顺数据源适配器
type Adapter struct {
	config       map[string]interface{}
	client       *http.Client
	parser       *helpers.KLineParser
	limiter      *rate.Limiter
	quota        adapter.QuotaInfo
	userAgentGen *helpers.UserAgentGenerator
	cookieGen    *helpers.CookieGenerator
	currentUA    string
	lastUpdateTime time.Time
}

// New 创建同花顺数据源适配器
func New() *Adapter {
	q := adapter.QuotaInfo{
		DailyLimit: -1,
		RateLimit:  5, // 5rps，同花顺限制较严
		Burst:      5,
	}
	r, burst := q.LimiterConfig()
	return &Adapter{
		config:       make(map[string]interface{}),
		client:       &http.Client{Timeout: 10 * time.Second},
		parser:       helpers.NewKLineParser(),
		limiter:      rate.NewLimiter(rate.Limit(r), burst),
		quota:        q,
		userAgentGen: helpers.NewUserAgentGenerator(),
		cookieGen:    helpers.NewCookieGenerator(),
	}
}

func (a *Adapter) Name() string                { return "tonghuashun" }
func (a *Adapter) DisplayName() string         { return "同花顺" }
func (a *Adapter) Type() string                { return "web_crawl" }

func (a *Adapter) Init(config map[string]interface{}) error {
	a.config = config
	a.currentUA = a.userAgentGen.GenerateUserAgent()
	a.lastUpdateTime = time.Now()
	return nil
}

func (a *Adapter) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://d.10jqka.com.cn", nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	log.Println("✅ 同花顺数据源连接正常")
	return nil
}

func (a *Adapter) Close() error {
	return nil
}

func (a *Adapter) GetQuotaInfo() adapter.QuotaInfo {
	return a.quota
}

// ========== 股票列表 ==========

func (a *Adapter) GetStockList(_ context.Context) ([]adapter.StockBasic, error) {
	var allStocks []adapter.StockBasic
	maxPages := 103

	for page := 1; page <= maxPages; page++ {
		stocks, hasMore, err := a.fetchStockListPage(page)
		if err != nil {
			if page == 1 {
				return nil, fmt.Errorf("获取股票列表失败: %w", err)
			}
			break
		}

		allStocks = append(allStocks, stocks...)

		if !hasMore || len(stocks) == 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	return allStocks, nil
}

func (a *Adapter) fetchStockListPage(page int) ([]adapter.StockBasic, bool, error) {
	url := fmt.Sprintf("https://data.10jqka.com.cn/funds/ggzjl/field/zdf/order/desc/page/%d/ajax/1/free/1/", page)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, false, err
	}

	a.setTHSHeaders(req, "https://data.10jqka.com.cn/funds/ggzjl/")
	hexinV := generateWencaiToken()
	cookieValue := a.cookieGen.GenerateTHSCookie(hexinV)
	req.Header.Set("Cookie", cookieValue)
	req.Header.Set("Hexin-V", hexinV)

	// 限流
	if err := a.limiter.Wait(context.Background()); err != nil {
		return nil, false, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}

	stocks, hasMore := a.parseStockListHTML(string(body))
	return stocks, hasMore, nil
}

// parseStockListHTML 解析股票列表HTML
func (a *Adapter) parseStockListHTML(html string) ([]adapter.StockBasic, bool) {
	lines := strings.Split(html, "\n")
	var stocks []adapter.StockBasic

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "stockCode") || !strings.Contains(line, "linkToGghq") {
			continue
		}

		code := extractBetween(line, "stockCode\">", "</a>")
		name := a.extractNameFromLines(lines, i)
		if code == "" {
			continue
		}

		_, exchange, board := detectTHSExchange(code)

		stocks = append(stocks, adapter.StockBasic{
			Code:        code,
			Name:        name,
			Exchange:    exchange,
			ListingBoard: board,
		})
	}

	return stocks, len(stocks) > 0
}

// extractNameFromNames 从多行中提取股票名称
func (a *Adapter) extractNameFromLines(lines []string, idx int) string {
	// 方法1：title属性
	for j := idx; j < len(lines) && j < idx+5; j++ {
		if name := extractBetween(lines[j], `title="`, `"`); name != "" && isValidStockName(name) {
			return name
		}
	}
	// 方法2：alt属性
	for j := idx; j < len(lines) && j < idx+5; j++ {
		if name := extractBetween(lines[j], `alt="`, `"`); name != "" && isValidStockName(name) {
			return name
		}
	}
	// 方法3：纯中文行
	for j := idx + 1; j < len(lines) && j < idx+10; j++ {
		nextLine := strings.TrimSpace(lines[j])
		if isValidStockName(nextLine) {
			return nextLine
		}
	}
	return ""
}

// GetStockDetail 获取股票详情
func (a *Adapter) GetStockDetail(ctx context.Context, code string) (*adapter.StockBasic, error) {
	symbol, _ := parseCode(code)
	_, exchange, board := detectTHSExchange(symbol)
	return &adapter.StockBasic{
		Code:         symbol,
		Name:         "", // 需要额外查询
		Exchange:     exchange,
		ListingBoard: board,
	}, nil
}

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
	symbol, _ := parseCode(code)
	thsCode := buildTHSCode(symbol)
	if thsCode == "" {
		return nil, fmt.Errorf("unsupported market for code: %s", code)
	}

	requestURL := fmt.Sprintf("https://d.10jqka.com.cn/v6/line/%s/%s/all.js", thsCode, klineType)

	body, err := a.makeTHSRequest(requestURL)
	if err != nil {
		return nil, err
	}

	dates, prices, volumes := parseTHSKLineResponse(body, klineType)
	result, err := a.parser.ParseTHSDailyKline(code, dates, prices, volumes)
	if err != nil {
		return nil, err
	}

	var pricesResult []adapter.StockPriceDaily
	for _, p := range result {
		pricesResult = append(pricesResult, adapter.StockPriceDaily{
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

	return pricesResult, nil
}

// ========== 实时/当日数据 ==========

// GetTodayData 获取当日数据
func (a *Adapter) GetTodayData(ctx context.Context, code string) (*adapter.StockPriceDaily, string, error) {
	symbol, _ := parseCode(code)
	thsCode := buildTHSCode(symbol)
	if thsCode == "" {
		return nil, "", fmt.Errorf("unsupported market: %s", code)
	}

	requestURL := fmt.Sprintf("https://d.10jqka.com.cn/v6/line/%s/%s/defer/today.js", thsCode, KLineTypeDaily)
	body, err := a.makeTHSRequest(requestURL)
	if err != nil {
		return nil, "", err
	}

	data, name, err := a.parseTodayDataResponse(code, thsCode, body)
	if err != nil {
		return nil, "", err
	}

	pd := adapter.StockPriceDaily{
		Code:      data.Code,
		Date:      data.Date,
		Open:      data.Open,
		High:      data.High,
		Low:       data.Low,
		Close:     data.Close,
		Volume:    data.Volume,
		Amount:    data.Amount,
		ChangePct: data.ChangePct,
		Turnover:  data.Turnover,
		Pe:        data.Pe,
		Pb:        data.Pb,
		MarketCap: data.MarketCap,
	}
	return &pd, name, nil
}

// GetThisWeekData / GetThisMonthData / GetThisQuarterData / GetThisYearData
// 使用对应的K线类型调用 today.js 接口
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
	symbol, _ := parseCode(code)
	thsCode := buildTHSCode(symbol)
	url := fmt.Sprintf("https://d.10jqka.com.cn/v6/line/%s/%s/defer/today.js", thsCode, klineType)
	body, err := a.makeTHSRequest(url)
	if err != nil {
		return nil, "", err
	}
	return a.parseTodayDataResponse(code, thsCode, body)
}

// ========== 财务数据 ==========

func (a *Adapter) GetPerformanceReports(ctx context.Context, code string) ([]adapter.PerformanceReport, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *Adapter) GetLatestPerformanceReport(ctx context.Context, code string) (*adapter.PerformanceReport, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *Adapter) GetShareholderCounts(ctx context.Context, code string) ([]adapter.ShareholderCount, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *Adapter) GetLatestShareholderCount(ctx context.Context, code string) (*adapter.ShareholderCount, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetShareChanges 获取历年股本变动数据
func (a *Adapter) GetShareChanges(ctx context.Context, code string) ([]adapter.ShareChange, error) {
	// TODO: 同花顺股本变动接口
	// 参考：https://basic.10jqka.com.cn/xxxxxx/share.html
	return nil, fmt.Errorf("not implemented")
}

// GetInstitutionalHoldings 获取机构持仓数据
func (a *Adapter) GetInstitutionalHoldings(_ context.Context, _ string) ([]adapter.InstitutionalHolding, error) {
	return nil, fmt.Errorf("not implemented")
}

// ========== HTTP请求辅助 ==========

// makeTHSRequest 发送同花顺请求（带限流和反爬）
func (a *Adapter) makeTHSRequest(url string) (string, error) {
	ctx := context.Background()
	if err := a.limiter.Wait(ctx); err != nil {
		return "", err
	}

	if time.Since(a.lastUpdateTime) > 1*time.Minute {
		a.currentUA = a.userAgentGen.GenerateUserAgent()
		a.lastUpdateTime = time.Now()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	a.setTHSHeaders(req, "https://stockpage.10jqka.com.cn/")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

// setTHSHeaders 设置同花顺请求头
func (a *Adapter) setTHSHeaders(req *http.Request, refer string) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-fetch-dest", "script")
	req.Header.Set("sec-fetch-mode", "no-cors")
	req.Header.Set("sec-fetch-site", "same-site")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("User-Agent", a.currentUA)
	req.Header.Set("Referer", refer)
	req.Header.Set("sec-ch-ua", `"Not;A=Brand";v="99", "Google Chrome";v="139", "Chromium":v="139"`)
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)
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
		Open:   toCents(parseFloat(openStr)),
		High:   toCents(parseFloat(highStr)),
		Low:    toCents(parseFloat(lowStr)),
		Close:  toCents(parseFloat(closeStr)),
		Volume: parseInt64(volStr),
		Amount: toCents(parseFloat(amountStr)),
	}, name, nil
}

// ========== 辅助函数 ==========

// buildTHSCode 构建同花顺代码格式 hs_xxxxxx
func buildTHSCode(symbol string) string {
	switch {
	case strings.HasPrefix(symbol, "60"), strings.HasPrefix(symbol, "68"):
		return "hs_" + symbol
	case strings.HasPrefix(symbol, "00"), strings.HasPrefix(symbol, "30"),
		strings.HasPrefix(symbol, "8"), strings.HasPrefix(symbol, "4"):
		return "hs_" + symbol
	default:
		return ""
	}
}

// detectTHSExchange 检测交易所
func detectTHSExchange(code string) (market, exchange, board string) {
	switch {
	case strings.HasPrefix(code, "60"), strings.HasPrefix(code, "68"):
		return "SH", model.ExchangeSSE, detectBoard(code)
	case strings.HasPrefix(code, "00"), strings.HasPrefix(code, "30"),
		strings.HasPrefix(code, "001"), strings.HasPrefix(code, "002"),
		strings.HasPrefix(code, "003"):
		return "SZ", model.ExchangeSZSE, detectBoard(code)
	default:
		return "SZ", model.ExchangeSZSE, model.BoardBSE
	}
}

// detectBoard 检测板块
func detectBoard(code string) string {
	switch {
	case strings.HasPrefix(code, "300"):
		return model.BoardChiNext
	case strings.HasPrefix(code, "688"):
		return model.BoardStar
	case strings.HasPrefix(code, "689"): // 北交所
		return model.BoardBSE
	default:
		return model.BoardMain
	}
}

// parseCode 解析代码
func parseCode(code string) (string, string) {
	parts := strings.Split(code, ".")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return code, model.ExchangeSZSE
}

// parseTHSKLineResponse 解析同花顺K线响应
func parseTHSKLineResponse(body, klineType string) ([]string, []string, []string) {
	callbackName := fmt.Sprintf("quotebridge_v6_line_%s_%s_all(", "placeholder", klineType)
	res := body
	if strings.Contains(res, callbackName) {
		res = strings.TrimPrefix(res, callbackName)
		res = strings.TrimSuffix(res, ")")
	}

	var response struct {
		Start    string   `json:"start"`
		SortYear [][]int  `json:"sortYear"`
		Price    string   `json:"price"`
		Volume   string   `json:"volumn"`
		Dates    string   `json:"dates"`
	}

	if err := json.Unmarshal([]byte(res), &response); err != nil {
		return nil, nil, nil
	}

	prices := strings.Split(response.Price, ",")
	volumes := strings.Split(response.Volume, ",")
	dates := strings.Split(response.Dates, ",")
	return dates, prices, volumes
}

// extractBetween 提取两个标记之间的字符串
func extractBetween(s, left, right string) string {
	start := strings.Index(s, left)
	if start == -1 {
		return ""
	}
	start += len(left)
	end := strings.Index(s[start:], right)
	if end == -1 {
		return ""
	}
	return s[start : start+end]
}

// isValidStockName 判断是否为有效股票名称
func isValidStockName(name string) bool {
	if len(name) == 0 || len(name) > 20 {
		return false
	}
	if strings.Contains(name, "<") || strings.Contains(name, ">") {
		return false
	}
	for _, r := range name {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

// formatTradeDate 格式化交易日期
func formatTradeDate(dateInt int) string {
	if dateInt == 0 {
		return time.Now().Format("2006-01-02")
	}
	dateStr := strconv.Itoa(dateInt)
	if len(dateStr) == 8 {
		return dateStr[:4] + "-" + dateStr[4:6] + "-" + dateStr[6:8]
	}
	return dateStr
}

// getString 从map获取字符串值
func getString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%.0f", val)
		}
		return fmt.Sprintf("%f", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// parseFloat 安全解析浮点数
func parseFloat(s string) float64 {
	if s == "" || s == "-" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// parseInt64 安全解析整数
func parseInt64(s string) int64 {
	if s == "" || s == "-" {
		return 0
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

// toCents 将元(float64)转换为分(int64)，四舍五入
func toCents(yuan float64) int64 {
	return int64(yuan*100 + 0.5)
}

// generateWencaiToken 生成问财token
// 简化版：返回随机hex token
func generateWencaiToken() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	const chars = "0123456789abcdef"
	b := make([]byte, 32)
	for i := range b {
		b[i] = chars[r.Intn(len(chars))]
	}
	return string(b)
}