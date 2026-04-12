package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
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
	KLineTypeDaily     = "101"
	KLineTypeWeekly    = "102"
	KLineTypeMonthly   = "103"
	KLineTypeQuarterly = "104"
	KLineTypeYearly    = "106"
)

// Adapter 东方财富数据源适配器
type Adapter struct {
	config         map[string]interface{}
	client         *http.Client
	parser         *helpers.KLineParser
	limiter        *rate.Limiter
	userAgentGen   *helpers.UserAgentGenerator
	cookieGen      *helpers.CookieGenerator
	currentUA      string
	currentCookie  string
	lastUpdateTime time.Time
}

// New 创建东方财富数据源适配器
func New() *Adapter {
	return &Adapter{
		config:       make(map[string]interface{}),
		client:       &http.Client{Timeout: 10 * time.Second},
		parser:       helpers.NewKLineParser(),
		limiter:      rate.NewLimiter(rate.Limit(5), 5), // 5次/秒
		userAgentGen: helpers.NewUserAgentGenerator(),
		cookieGen:    helpers.NewCookieGenerator(),
	}
}

func (a *Adapter) Name() string        { return "eastmoney" }
func (a *Adapter) DisplayName() string { return "东方财富" }
func (a *Adapter) Type() string        { return "web_crawl" }

func (a *Adapter) Init(config map[string]interface{}) error {
	a.config = config
	a.updateHeaders()
	return nil
}

func (a *Adapter) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://push2.eastmoney.com", nil)
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
	log.Println("✅ 东方财富数据源连接正常")
	return nil
}

func (a *Adapter) Close() error {
	return nil
}

// GetQuotaInfo 获取配额信息
func (a *Adapter) GetQuotaInfo() adapter.QuotaInfo {
	return adapter.QuotaInfo{
		DailyLimit: -1,
		RateLimit:  5,
	}
}

// ========== 股票列表 ==========

// StockListResponse 股票列表API响应
type StockListResponse struct {
	RC   int `json:"rc"`
	Data struct {
		Total int                             `json:"total"`
		Diff  []StockListResponseDataDiffItem `json:"diff"`
	} `json:"data"`
}

// StockListResponseDataDiffItem 股票列表单条数据
// 注意: 东财API字段类型不稳定，数值字段可能返回string("-")或number
type StockListResponseDataDiffItem struct {
	F12 string `json:"f12"` // 代码
	F13 int    `json:"f13"` // 市场 0=深市 1=沪市
	F14 string `json:"f14"` // 名称
}

func (a *Adapter) GetStockList(_ context.Context, cb adapter.ProgressCallback) ([]adapter.StockBasic, error) {
	var allStocks []adapter.StockBasic
	page := 1
	pageSize := 50

	for {
		if cb != nil {
			cb(len(allStocks), 0, fmt.Sprintf("正在获取第 %d 页...", page))
		}

		respData, err := a.fetchStockListPage(page, pageSize)
		if err != nil {
			return nil, fmt.Errorf("获取第%d页失败: %w", page, err)
		}

		if len(respData.Data.Diff) == 0 {
			break
		}

		for _, item := range respData.Data.Diff {
			stock := a.convertToStockBasic(item)
			allStocks = append(allStocks, stock)
		}

		if len(respData.Data.Diff) < pageSize {
			break
		}
		page++
		time.Sleep(100 * time.Millisecond)
	}

	if cb != nil {
		cb(len(allStocks), len(allStocks), "股票列表获取完成")
	}
	return allStocks, nil
}

func (a *Adapter) fetchStockListPage(page, pageSize int) (*StockListResponse, error) {
	baseURL := "https://push2.eastmoney.com/api/qt/clist/get"
	params := url.Values{
		"cb":     {fmt.Sprintf("jQuery%d", time.Now().UnixMilli())},
		"fid":    {"f12"},
		"po":     {"0"},
		"pz":     {strconv.Itoa(pageSize)},
		"pn":     {strconv.Itoa(page)},
		"np":     {"1"},
		"fltt":   {"2"},
		"invt":   {"2"},
		"ut":     {"b2884a393a59ad64002292a3e90d46a5"},
		"fs":     {"m:0+t:6+f:!2,m:0+t:13+f:!2,m:0+t:80+f:!2,m:1+t:2+f:!2,m:1+t:23+f:!2,m:0+t:7+f:!2,m:1+t:3+f:!2"},
		"fields": {"f12,f14,f15,f16,f17,f18,f19,f20,f23,f116,f117,f162,f13,f128,f164,f167,f168,f169,f170,f171,f177,f183,f184,f185,f186,f187,f188,f189,f190,f191,f192,f193,f204,f205,f206,f207,f208,f209,f210,f211,f212,f213,f214,f215"},
	}

	requestURL := baseURL + "?" + params.Encode()
	body, err := a.makeGetRequest(requestURL, "https://data.eastmoney.com/zjlx/detail.html")
	if err != nil {
		return nil, err
	}

	jsonStr := extractJSONP(body)
	var result StockListResponse
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}
	return &result, nil
}

// convertToStockBasic 将API响应转换为StockBasic（含发行价等静态信息）
func (a *Adapter) convertToStockBasic(item StockListResponseDataDiffItem) adapter.StockBasic {
	exchange, listingBoard := detectExchangeAndBoard(item.F12)

	return adapter.StockBasic{
		Code:         item.F12,
		Name:         item.F14,
		Exchange:     exchange,
		ListingBoard: listingBoard,
	}
}

// GetStockDetail 获取股票详情（含IPO信息）
func (a *Adapter) GetStockDetail(ctx context.Context, code string) (*adapter.StockBasic, error) {
	symbol, market := parseCode(code)
	secid := buildSecID(symbol, market)
	refer := getQuoteReferURL(code)

	params := url.Values{
		"ut":     {"fa5fd1943c7b386f172d6893dbfba10b"},
		"invt":   {"2"},
		"fltt":   {"2"},
		"cb":     {fmt.Sprintf("jQuery%d", time.Now().UnixMilli())},
		"secid":  {secid},
		"fields": {"f57,f58,f107,f43,f169,f170,f171,f47,f48,f60,f46,f44,f45,f168,f50,f162,f177,f803,f129,f130,f131,f132,f133,f134,f135,f136,f137,f138,f139,f140,f141,f142,f143,f144,f145,f146,f147,f148,f149,f150,f151,f152,f153,f154,f155,f156,f157,f158,f159,f160,f161,f163,f164,f165,f166,f167"},
	}

	url := "https://push2.eastmoney.com/api/qt/stock/get?" + params.Encode()
	body, err := a.makeGetRequest(url, refer)
	if err != nil {
		return nil, err
	}

	jsonStr := extractJSONP(body)
	var result struct {
		RC   int `json:"rc"`
		Data struct {
			F57  string  `json:"f57"`  // 代码
			F58  string  `json:"f58"`  // 名称
			F107 int     `json:"f107"` // 停牌
			F43  float64 `json:"f43"`  // 现价
			F169 float64 `json:"f169"` // 涨跌额
			F170 float64 `json:"f170"` // PE动
			F171 float64 `json:"f171"` // PB
			F47  int64   `json:"f47"`  // 成交量
			F48  float64 `json:"f48"`  // 成交额
			F60  float64 `json:"f60"`  // 昨收
			F177 int     `json:"f177"` // 流通股本
			F803 string  `json:"f803"` // 板块
			// IPO相关字段
			F129 string  `json:"f129"` // 上市日期
			F130 float64 `json:"f130"` // 发行价
			F131 float64 `json:"f131"` // 发行PE
			F132 float64 `json:"f132"` // 发行PB
			F133 float64 `json:"f133"` // 总股本(万)
			F134 float64 `json:"f134"` // 流通股本(万)
			F135 string  `json:"f135"` // 所属行业
			F136 string  `json:"f136"` // 细分行业
			F137 string  `json:"f137"` // 地区
			F138 string  `json:"f138"` // 公司全称
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	exchange, listingBoard := detectExchangeAndBoard(result.Data.F57)
	return &adapter.StockBasic{
		Code:         result.Data.F57,
		Name:         result.Data.F58,
		FullName:     result.Data.F138,
		Exchange:     exchange,
		ListingBoard: listingBoard,
		ListDate:     result.Data.F129,
		IssuePrice:   result.Data.F130,
		IssuePE:      result.Data.F131,
		IssuePB:      result.Data.F132,
		IssueShares:  0,
		Industry:     result.Data.F135,
		Sector:       result.Data.F136,
	}, nil
}

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
func (a *Adapter) GetDailyKLine(ctx context.Context, code, startDate, endDate string, cb adapter.ProgressCallback) ([]adapter.StockPriceDaily, error) {
	return a.fetchKLines(ctx, code, startDate, endDate, KLineTypeDaily, cb)
}

// GetWeeklyKLine 获取周K线
func (a *Adapter) GetWeeklyKLine(ctx context.Context, code, startDate, endDate string, cb adapter.ProgressCallback) ([]adapter.StockPriceDaily, error) {
	return a.fetchKLines(ctx, code, startDate, endDate, KLineTypeWeekly, cb)
}

// GetMonthlyKLine 获取月K线
func (a *Adapter) GetMonthlyKLine(ctx context.Context, code, startDate, endDate string, cb adapter.ProgressCallback) ([]adapter.StockPriceDaily, error) {
	return a.fetchKLines(ctx, code, startDate, endDate, KLineTypeMonthly, cb)
}

// GetQuarterlyKLine 获取季K线
func (a *Adapter) GetQuarterlyKLine(ctx context.Context, code, startDate, endDate string, cb adapter.ProgressCallback) ([]adapter.StockPriceDaily, error) {
	return a.fetchKLines(ctx, code, startDate, endDate, KLineTypeQuarterly, cb)
}

// GetYearlyKLine 获取年K线
func (a *Adapter) GetYearlyKLine(ctx context.Context, code, startDate, endDate string, cb adapter.ProgressCallback) ([]adapter.StockPriceDaily, error) {
	return a.fetchKLines(ctx, code, startDate, endDate, KLineTypeYearly, cb)
}

// fetchKLines 通用K线获取方法
func (a *Adapter) fetchKLines(ctx context.Context, code, startDate, endDate, klineType string, cb adapter.ProgressCallback) ([]adapter.StockPriceDaily, error) {
	symbol, market := parseCode(code)
	secid := buildSecID(symbol, market)
	refer := "https://quote.eastmoney.com"

	params := url.Values{
		"fields1": {"f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13"},
		"fields2": {"f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61"},
		"beg":     {strings.ReplaceAll(startDate, "-", "")},
		"end":     {"20500101"},
		"ut":      {"fa5fd1943c7b386f172d6893dbfba10b"},
		"rtntype": {"6"},
		"secid":   {secid},
		"klt":     {klineType},
		"fqt":     {"1"},
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

	if cb != nil {
		cb(len(result), len(response.Data.Klines), "K线数据获取完成")
	}
	return result, nil
}

// ========== 实时数据 ==========

// RealtimeResponse 实时行情响应
type RealtimeResponse struct {
	RC   int `json:"rc"`
	Data struct {
		Diff []struct {
			F12 string      `json:"f12"` // 代码
			F13 int         `json:"f13"` // 市场
			F14 string      `json:"f14"` // 名称
			F2  interface{} `json:"f2"`  // 最新价
			F3  interface{} `json:"f3"`  // 涨跌幅
			F4  interface{} `json:"f4"`  // 涨跌额
			F5  interface{} `json:"f5"`  // 成交量
			F6  interface{} `json:"f6"`  // 成交额
			F7  interface{} `json:"f7"`  // 振幅
			F15 interface{} `json:"f15"` // 最高价
			F16 interface{} `json:"f16"` // 最低价
			F17 interface{} `json:"f17"` // 开盘价
			F18 interface{} `json:"f18"` // 昨收
			F20 int         `json:"f20"` // 涨跌
			F22 interface{} `json:"f22"` // 换手率
			F23 interface{} `json:"f23"` // 市盈率
			F24 interface{} `json:"f24"` // 市净率
			F25 interface{} `json:"f25"` // 总市值
			F26 interface{} `json:"f26"` // 流通市值
			F27 interface{} `json:"f27"` // 量比
		} `json:"diff"`
	} `json:"data"`
}

// GetRealtimeData 批量获取实时行情
func (a *Adapter) GetRealtimeData(ctx context.Context, codes []string, cb adapter.ProgressCallback) (map[string]adapter.StockPriceDaily, error) {
	result := make(map[string]adapter.StockPriceDaily)

	// 按市场分组
	sseCodes, szseCodes := groupByMarket(codes)
	total := len(codes)

	// 上证
	if len(sseCodes) > 0 {
		data, err := a.fetchRealtimeBatch(sseCodes, 1, ctx)
		if err != nil && cb != nil {
			cb(0, 0, fmt.Sprintf("上证实时数据获取失败: %v", err))
		}
		for k, v := range data {
			result[k] = v
		}
		if cb != nil {
			cb(len(data), total, "")
		}
	}

	// 深证
	if len(szseCodes) > 0 {
		data, err := a.fetchRealtimeBatch(szseCodes, 0, ctx)
		if err != nil && cb != nil {
			cb(0, 0, fmt.Sprintf("深证实时数据获取失败: %v", err))
		}
		for k, v := range data {
			result[k] = v
		}
		if cb != nil {
			cb(len(result), total, "实时数据获取完成")
		}
	}

	return result, nil
}

// fetchRealtimeBatch 批量获取单个市场实时行情（每批最多500只）
func (a *Adapter) fetchRealtimeBatch(codes []string, market int, ctx context.Context) (map[string]adapter.StockPriceDaily, error) {
	result := make(map[string]adapter.StockPriceDaily)
	batchSize := 500

	for i := 0; i < len(codes); i += batchSize {
		end := i + batchSize
		if end > len(codes) {
			end = len(codes)
		}
		batch := codes[i:end]

		secids := ""
		for _, c := range batch {
			secids += fmt.Sprintf("%d.%s,", market, c)
		}
		secids = strings.TrimRight(secids, ",")

		params := url.Values{
			"ut":     {"fa5fd1943c7b386f172d6893dbfba10b"},
			"fltt":   {"2"},
			"invt":   {"2"},
			"fields": {"f12,f13,f14,f2,f3,f4,f5,f6,f7,f15,f16,f17,f18,f20,f22,f23,f24,f25,f26,f27"},
			"secids": {secids},
		}

		url := "https://push2.eastmoney.com/api/qt/ulist.np/get?" + params.Encode()
		body, err := a.makeGetRequest(url, "https://quote.eastmoney.com")
		if err != nil {
			continue
		}

		jsonStr := extractJSONP(body)
		var resp RealtimeResponse
		if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
			continue
		}

		for _, item := range resp.Data.Diff {
			code := item.F12
			now := time.Now().Format("2006-01-02")
			result[code] = adapter.StockPriceDaily{
				Code:      code,
				Date:      now,
				Open:      toCents(parseFloatI(item.F17)), // 元→分
				High:      toCents(parseFloatI(item.F15)),
				Low:       toCents(parseFloatI(item.F16)),
				Close:     toCents(parseFloatI(item.F2)),
				Volume:    parseInt64I(item.F5),
				Amount:    toCents(parseFloatI(item.F6)), // 元→分
				Change:    toCents(parseFloatI(item.F4)), // 元→分
				ChangePct: parseFloatI(item.F3),
				Turnover:  parseFloatI(item.F22),
				Pe:        parseFloatI(item.F23),
				Pb:        parseFloatI(item.F24),
				MarketCap: parseFloatI(item.F25),
			}
		}

		// 批次间延迟
		time.Sleep(50 * time.Millisecond)
	}

	return result, nil
}

// GetTodayData 当日数据 - 使用K线接口
func (a *Adapter) GetTodayData(ctx context.Context, code string) (*adapter.StockPriceDaily, string, error) {
	klines, err := a.GetDailyKLine(ctx, code, "", "", nil)
	if err != nil || len(klines) == 0 {
		return nil, "", err
	}
	last := klines[len(klines)-1]
	detail, _ := a.GetStockDetail(ctx, code)
	name := ""
	if detail != nil {
		name = detail.Name
	}
	return &last, name, nil
}

// GetThisWeekData 本周数据
func (a *Adapter) GetThisWeekData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	klines, err := a.GetWeeklyKLine(ctx, code, "", "", nil)
	if err != nil || len(klines) == 0 {
		return nil, err
	}
	return &klines[len(klines)-1], nil
}

// GetThisMonthData 本月数据
func (a *Adapter) GetThisMonthData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	klines, err := a.GetMonthlyKLine(ctx, code, "", "", nil)
	if err != nil || len(klines) == 0 {
		return nil, err
	}
	return &klines[len(klines)-1], nil
}

// GetThisQuarterData 本季数据
func (a *Adapter) GetThisQuarterData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	klines, err := a.GetQuarterlyKLine(ctx, code, "", "", nil)
	if err != nil || len(klines) == 0 {
		return nil, err
	}
	return &klines[len(klines)-1], nil
}

// GetThisYearData 本年数据
func (a *Adapter) GetThisYearData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	klines, err := a.GetYearlyKLine(ctx, code, "", "", nil)
	if err != nil || len(klines) == 0 {
		return nil, err
	}
	return &klines[len(klines)-1], nil
}

// ========== 财务数据 ==========

// GetPerformanceReports 获取业绩报表
func (a *Adapter) GetPerformanceReports(ctx context.Context, code string) ([]adapter.PerformanceReport, error) {
	// TODO: 移植 stock 项目中东方财富的业绩报表逻辑
	return nil, fmt.Errorf("not implemented")
}

// GetLatestPerformanceReport 获取最新业绩报表
func (a *Adapter) GetLatestPerformanceReport(ctx context.Context, code string) (*adapter.PerformanceReport, error) {
	reports, err := a.GetPerformanceReports(ctx, code)
	if err != nil {
		return nil, err
	}
	if len(reports) == 0 {
		return nil, fmt.Errorf("no reports for %s", code)
	}
	latest := reports[0]
	for i := 1; i < len(reports); i++ {
		if reports[i].ReportDate > latest.ReportDate {
			latest = reports[i]
		}
	}
	return &latest, nil
}

// GetShareholderCounts 获取股东户数
func (a *Adapter) GetShareholderCounts(ctx context.Context, code string) ([]adapter.ShareholderCount, error) {
	// TODO: 移植 stock 项目中东方财富的股东户数逻辑
	return nil, fmt.Errorf("not implemented")
}

// GetLatestShareholderCount 获取最新股东户数
func (a *Adapter) GetLatestShareholderCount(ctx context.Context, code string) (*adapter.ShareholderCount, error) {
	counts, err := a.GetShareholderCounts(ctx, code)
	if err != nil {
		return nil, err
	}
	if len(counts) == 0 {
		return nil, fmt.Errorf("no data for %s", code)
	}
	latest := counts[0]
	for i := 1; i < len(counts); i++ {
		if counts[i].EndDate > latest.EndDate {
			latest = counts[i]
		}
	}
	return &latest, nil
}

// ========== 股本变动 ==========

// equityItem 东财F10股本结构单条记录 (RPT_F10_EH_EQUITY)
//
// 数据来源: datacenter.eastmoney.com/securities/api/data/v1/get
// 报告名: RPT_F10_EH_EQUITY (HSF10)
// 单位: 股
type equityItem struct {
	Secucode        string `json:"SECUCODE"`         // 全称 600519.SH
	SecurityCode    string `json:"SECURITY_CODE"`    // 纯代码 600519
	EndDate         string `json:"END_DATE"`         // 变动日期 YYYY-MM-DD HH:mm:ss
	TotalShares     int64  `json:"TOTAL_SHARES"`     // 总股本(股)
	LimitedShares   int64  `json:"LIMITED_SHARES"`   // 流通受限股份(股)
	UnlimitedShares int64  `json:"UNLIMITED_SHARES"` // 已流通股份(股)
	ListedAShares   int64  `json:"LISTED_A_SHARES"`  // 已上市流通A股(股)
	ChangeReason    string `json:"CHANGE_REASON"`    // 变动原因
}

// equityResponse 东财F10股本结构API响应
type equityResponse struct {
	Result struct {
		Pages int          `json:"pages"`
		Count int          `json:"count"`
		Data  []equityItem `json:"data"`
	} `json:"result"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetShareChanges 获取历年股本变动数据
//
// API: datacenter.eastmoney.com/securities/api/data/v1/get?reportName=RPT_F10_EH_EQUITY
//
// 返回数据与图片完全对应:
//   - TotalShares(总股本)     → 灰色柱
//   - ListedAShares(流通A股)  → 橙色柱
//   - LimitedShares(受限股份) → 浅黄色柱
//   - ChangeReason(变动原因)  → 底部文字(高管股份变动/回购/增发...)
//
// 内部自动完成:
//   - 股→万股转换 (÷10000)
//   - 日期格式化 (截取 YYYY-MM-DD)
//   - 自动分页拉取全部历史记录
func (a *Adapter) GetShareChanges(ctx context.Context, code string) ([]adapter.ShareChange, error) {
	symbol, market := parseCode(code)
	secucode := symbol + "." + market
	if market == "" {
		secucode = buildSecucode(symbol)
	}

	// 核心字段: 日期 + 总股本 + 流通A股 + 受限股份 + 变动原因
	columns := "SECUCODE,SECURITY_CODE,END_DATE,TOTAL_SHARES,LIMITED_SHARES," +
		"UNLIMITED_SHARES,LISTED_A_SHARES,FREE_SHARES,LIMITED_A_SHARES," +
		"LOCK_SHARES,CHANGE_REASON"

	var allChanges []adapter.ShareChange
	page := 1
	pageSize := 50
	totalPages := 0

	for {
		params := url.Values{
			"reportName":   {"RPT_F10_EH_EQUITY"},
			"columns":      {columns},
			"quoteColumns": {""},
			"filter":       {fmt.Sprintf(`(SECUCODE="%s")`, secucode)},
			"pageNumber":   {strconv.Itoa(page)},
			"pageSize":     {strconv.Itoa(pageSize)},
			"sortTypes":    {"-1"},       // 降序（最新在前）
			"sortColumns":  {"END_DATE"}, // 按结束日期排序
			"source":       {"HSF10"},
			"client":       {"PC"},
		}

		url := "https://datacenter.eastmoney.com/securities/api/data/v1/get?" + params.Encode()
		body, err := a.makeGetRequest(url, "https://emweb.securities.eastmoney.com/")
		if err != nil {
			return nil, fmt.Errorf("请求股本变动第%d页失败: %w", page, err)
		}

		var resp equityResponse
		if err := json.Unmarshal([]byte(body), &resp); err != nil {
			return nil, fmt.Errorf("解析股本变动JSON失败: %w", err)
		}
		if !resp.Success {
			return nil, fmt.Errorf("股本变动API错误: %s", resp.Message)
		}

		if totalPages == 0 {
			totalPages = resp.Result.Pages
		}

		for _, item := range resp.Result.Data {
			// 截取日期 "2025-11-28 00:00:00" → "2025-11-28"
			dateStr := item.EndDate
			if len(dateStr) >= 10 {
				dateStr = dateStr[:10]
			}

			allChanges = append(allChanges, adapter.ShareChange{
				Code:            code,
				Date:            dateStr,
				TotalShares:     item.TotalShares,      // 股
				LimitedShares:   item.LimitedShares,    // 股
				UnlimitedShares: item.UnlimitedShares,  // 股
				FloatAShares:    item.ListedAShares,    // 股
				ChangeReason:    item.ChangeReason,
			})
		}

		if page >= totalPages || len(resp.Result.Data) < pageSize {
			break
		}
		page++
		time.Sleep(80 * time.Millisecond)
	}

	log.Printf("[eastmoney] %s 股本变动: %d 条记录 (%d页)", code, len(allChanges), totalPages)
	return allChanges, nil
}

// buildSecucode 根据代码构建东财 SECUCODE 格式 (如 002404.SZ / 600000.SH)
func buildSecucode(symbol string) string {
	switch {
	case strings.HasPrefix(symbol, "60"), strings.HasPrefix(symbol, "68"):
		return symbol + ".SH"
	case strings.HasPrefix(symbol, "8"), strings.HasPrefix(symbol, "4"):
		return symbol + ".BJ"
	default:
		return symbol + ".SZ"
	}
}

// ========== HTTP请求辅助 ==========

// makeGetRequest 发送GET请求（带限流和反爬）
func (a *Adapter) makeGetRequest(url, refer string) (string, error) {
	ctx := context.Background()

	// 限流
	if err := a.limiter.Wait(ctx); err != nil {
		return "", err
	}

	// 定期更新UA/Cookie
	if time.Since(a.lastUpdateTime) > 1*time.Minute {
		a.updateHeaders()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	// 设置请求头
	setCommonHeaders(req, a.currentUA, a.currentCookie, refer)
	req.Header.Set("Referer", refer)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// updateHeaders 更新随机UA和Cookie
func (a *Adapter) updateHeaders() {
	a.currentUA = a.userAgentGen.GenerateUserAgent()
	a.currentCookie = a.cookieGen.GenerateCookie()
	a.lastUpdateTime = time.Now()
}

// ========== 辅助函数 ==========

// setCommonHeaders 设置公共请求头
func setCommonHeaders(req *http.Request, ua, cookie, refer string) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-fetch-dest", "script")
	req.Header.Set("sec-fetch-mode", "no-cors")
	req.Header.Set("sec-fetch-site", "same-site")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("Referer", refer)
}

// extractJSONP 从JSONP响应中提取JSON
func extractJSONP(body string) string {
	// jQuery格式: jQuery1234567890(...)
	re := regexp.MustCompile(`jQuery[\d_]+\((.*)\)`)
	matches := re.FindStringSubmatch(body)
	if len(matches) >= 2 {
		return matches[1]
	}
	// jsonp格式
	re2 := regexp.MustCompile(`jsonp\d+\((.*)\)`)
	matches2 := re2.FindStringSubmatch(body)
	if len(matches2) >= 2 {
		return matches2[1]
	}
	// 直接返回
	return body
}

// parseCode 解析代码，返回 symbol 和 market
func parseCode(code string) (string, string) {
	parts := strings.Split(code, ".")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return code, ""
}

// buildSecID 构建东方财富 secid
func buildSecID(symbol, market string) string {
	switch market {
	case "SH", "SSE":
		return "1." + symbol
	case "SZ", "SZSE":
		return "0." + symbol
	case "BSE":
		return "0." + symbol // 北交所也用0
	default:
		// 根据代码前缀判断
		if strings.HasPrefix(symbol, "60") || strings.HasPrefix(symbol, "68") {
			return "1." + symbol
		}
		return "0." + symbol
	}
}

// detectExchangeAndBoard 根据代码判断交易所和板块
func detectExchangeAndBoard(code string) (exchange, board string) {
	switch {
	case strings.HasPrefix(code, "600"), strings.HasPrefix(code, "601"),
		strings.HasPrefix(code, "603"), strings.HasPrefix(code, "605"):
		return model.ExchangeSSE, model.BoardMain
	case strings.HasPrefix(code, "688"):
		return model.ExchangeSSE, model.BoardStar
	case strings.HasPrefix(code, "000"), strings.HasPrefix(code, "001"),
		strings.HasPrefix(code, "002"), strings.HasPrefix(code, "003"):
		return model.ExchangeSZSE, model.BoardMain
	case strings.HasPrefix(code, "300"):
		return model.ExchangeSZSE, model.BoardChiNext
	case strings.HasPrefix(code, "8"), strings.HasPrefix(code, "4"):
		return model.ExchangeBSE, model.BoardBSE
	default:
		return model.ExchangeSZSE, model.BoardMain
	}
}

// getQuoteReferURL 获取行情页面的Refer URL
func getQuoteReferURL(code string) string {
	symbol, market := parseCode(code)
	switch market {
	case "SH", "SSE":
		return fmt.Sprintf("https://quote.eastmoney.com/sh%s.html", symbol)
	default:
		return fmt.Sprintf("https://quote.eastmoney.com/sz%s.html", symbol)
	}
}

// groupByMarket 按市场分组
func groupByMarket(codes []string) ([]string, []string) {
	var sse, szse []string
	for _, code := range codes {
		symbol, _ := parseCode(code)
		switch {
		case strings.HasPrefix(symbol, "60"), strings.HasPrefix(symbol, "68"):
			sse = append(sse, symbol)
		default:
			szse = append(szse, symbol)
		}
	}
	return sse, szse
}

// parseFloatI 安全转换 interface{} -> float64
func parseFloatI(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0
		}
		return f
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case float32:
		return float64(val)
	default:
		return 0
	}
}

// parseInt64I 安全转换 interface{} -> int64
func parseInt64I(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		i, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return 0
		}
		return i
	default:
		return 0
	}
}

// toCents 将元(float64)转换为分(int64)，四舍五入
func toCents(yuan float64) int64 {
	return int64(yuan*100 + 0.5)
}
