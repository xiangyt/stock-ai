package tencentstock

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/helpers"
)

// ========== K线数据 - 腾讯 newfqkline API ==========
//
// API: https://proxy.finance.qq.com/ifzqgtimg/appstock/app/newfqkline/get
//
// 请求参数 (无日期范围，全量拉取):
//
//	_var=kline_{period}{adj}           // JSONP回调名
//	param={code},{period},,,320,{adj}  // code/周期/空/空/条数/复权方式
//
// 请求参数 (指定日期范围):
//
//	_var=kline_{period}{adj}{year}                          // JSONP回调名(带年份)
//	param={code},{period},{start},{end},640,{adj}           // 日期格式 YYYY-MM-DD
//
// 响应格式 (JSONP):
//
//	kline_weekhfq=({"code":0,"data":{"sh688010":{"hfqweek":[...], "fsStartDate":"...", ...}}})
//
// 全量采集策略：先探测得到上市年份(fsStartDate)，再按年循环拉取，最后去重合并
//
// K线数组每条记录格式 (10个元素，混合类型):
//
//	[0] 日期          "2020-03-20"
//	[1] 开盘价(元)     "40.69"
//	[2] 收盘价(元)     "38.19"
//	[3] 最高价(元)     "41.05"
//	[4] 最低价(元)     "37.11"
//	[5] 成交量(股)     "7350685.00"
//	[6] 除权事件       {} 或 {"nd":"2019","fh_sh":"2","djr":"2020-07-09",...}
//	[7] 换手率(%)     "19.75"
//	[8] 成交额(万元)   "28433.88"
//	[9] 交易天数       "5"
//
// 采集字段: [0]Date [1]Open [2]Close [3]High [4]Low [5]Volume [7]Turnover [8]Amount

// newFqKLineURL 腾讯复权K线接口地址
const newFqKLineURL = "https://proxy.finance.qq.com/ifzqgtimg/appstock/app/newfqkline/get"

// ========== 公共接口实现 ==========

// GetDailyKLine 获取日K线（按年循环拉取全量，每次1年）
func (a *Adapter) GetDailyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.fetchNewFqKLines(ctx, code, "day", adjType, 1)
}

// GetWeeklyKLine 获取周K线（2年一批拉取全量）
func (a *Adapter) GetWeeklyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return a.fetchNewFqKLines(ctx, code, "week", adjType, 2)
}

// GetMonthlyKLine 获取月K线（先单次全量拉，若满320条说明可能截断，回退10年一批）
func (a *Adapter) GetMonthlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	tc := toTencentCode(code)
	adjParam, adjKey := mapAdjType(adjType)

	// 先尝试单次全量拉取
	data, err := a.fetchAllKLines(ctx, tc, "month", adjParam, adjKey)
	if err == nil && len(data) < 320 {
		return data, nil
	}

	// 满 320 条 → 可能被截断，改为 10 年一批循环拉取
	return a.fetchNewFqKLines(ctx, code, "month", adjType, 10)
}

// GetQuarterlyKLine 获取季K线（腾讯 newfqkline API 不支持 quarter 周期）
func (a *Adapter) GetQuarterlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

// GetYearlyKLine 获取年K线（单次全量拉取，320条足够）
func (a *Adapter) GetYearlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	tc := toTencentCode(code)
	adjParam, adjKey := mapAdjType(adjType)
	return a.fetchAllKLines(ctx, tc, "year", adjParam, adjKey)
}

// GetIndexDailyKLine 获取指数日K线（暂不支持）
func (a *Adapter) GetIndexDailyKLine(ctx context.Context, code string, startTime, endTime time.Time, adjType string) ([]adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

// ========== 内部实现 ==========

// mapAdjType 将 adapter.AdjType 常量映射为腾讯API参数和响应键前缀
func mapAdjType(adjType string) (param, keyPrefix string) {
	switch adjType {
	case adapter.AdjQFQ:
		return "qfq", "qfq"
	case adapter.AdjBQQ:
		return "hfq", "hfq"
	default:
		return "", ""
	}
}

// newFqKLineEnvelope 腾讯 newfqkline API 最外层响应
type newFqKLineEnvelope struct {
	Code int                        `json:"code"`
	Data map[string]json.RawMessage `json:"data"`
}

// fetchNewFqKLines 按年份批次循环拉取全量K线数据
//
// yearStep: 每批覆盖的年数（日K=1年, 周K=2年）
//
// 流程:
//  1. 探测 — 获取 fsStartDate 确定起始年份
//  2. 从起始年份到当前年份，按 yearStep 分批拉取
//  3. 去重合并 → 日期升序返回
func (a *Adapter) fetchNewFqKLines(ctx context.Context, code, period, adjType string, yearStep int) ([]adapter.StockPriceDaily, error) {
	tc := toTencentCode(code)
	adjParam, adjKey := mapAdjType(adjType)

	startYear, err := a.probeStartYear(ctx, tc, period, adjParam, adjKey)
	if err != nil {
		return nil, fmt.Errorf("探测起始年份失败: %w", err)
	}

	currentYear := time.Now().Year()
	dateMap := make(map[string]adapter.StockPriceDaily)

	for y := startYear; y <= currentYear; y += yearStep {
		endY := y + yearStep - 1
		if endY > currentYear {
			endY = currentYear
		}

		data, err := a.fetchKLinesRange(ctx, tc, period, adjParam, adjKey, y, endY)
		if err != nil {
			continue
		}
		for _, item := range data {
			if existing, ok := dateMap[item.Date]; !ok || item.Date > existing.Date {
				dateMap[item.Date] = item
			}
		}
	}

	if len(dateMap) == 0 {
		return nil, fmt.Errorf("按年循环拉取后无数据 (startYear=%d, currentYear=%d)", startYear, currentYear)
	}

	return sortByDate(dateMap), nil
}

// fetchAllKLines 单次全量拉取（月K/年K，无日期范围）
func (a *Adapter) fetchAllKLines(ctx context.Context, tc, period, adjParam, adjKey string) ([]adapter.StockPriceDaily, error) {
	urlStr := fmt.Sprintf(
		"%s?_var=kline_%s%s&param=%s,%s,,,320,%s&r=%d",
		newFqKLineURL, period, adjKey, tc, period, adjParam, time.Now().UnixMilli(),
	)

	body, err := a.doGet(ctx, urlStr, qtRefer)
	if err != nil {
		return nil, fmt.Errorf("fetchAllKLines(%s) 请求失败: %w", period, err)
	}

	return parseNewFqKLineResponse(body, tc, period, adjKey, 0)
}

// probeStartYear 探测请求获取数据起始年份
//
// 发起一次仅拉5条的无日期范围请求，从响应中提取 fsStartDate。
// 若提取失败，回退到 2000 年（绝大多A股在此之后上市）。
func (a *Adapter) probeStartYear(ctx context.Context, tc, period, adjParam, adjKey string) (int, error) {
	urlStr := fmt.Sprintf(
		"%s?_var=kline_%s%s&param=%s,%s,,,5,%s&r=%d",
		newFqKLineURL, period, adjKey, tc, period, adjParam, time.Now().UnixMilli(),
	)

	body, err := a.doGet(ctx, urlStr, qtRefer)
	if err != nil {
		return 2000, nil // 回退默认值
	}

	fsDate := extractFSStartDate(body, tc)
	if fsDate == "" {
		return 2000, nil
	}

	year, err := strconv.Atoi(fsDate[:4])
	if err != nil || year < 1990 || year > time.Now().Year() {
		return 2000, nil
	}
	return year, nil
}

// fetchKLinesRange 按年份范围拉取K线数据
//
// startYear/endYear: 起始/结束年份（含）
//
// 请求格式:
//
//	_var=kline_{period}{adj}{startYear}
//	param={tc},{period},{startYear}-01-01,{endYear}-12-31,640,{adj}
//
// 响应数据键: {adj}{period}{startYear} (如 qfqday2021)
func (a *Adapter) fetchKLinesRange(ctx context.Context, tc, period, adjParam, adjKey string, startYear, endYear int) ([]adapter.StockPriceDaily, error) {
	urlStr := fmt.Sprintf(
		"%s?_var=kline_%s%s%d&param=%s,%s,%d-01-01,%d-12-31,640,%s&r=%d",
		newFqKLineURL, period, adjKey, startYear,
		tc, period, startYear, endYear, adjParam,
		time.Now().UnixMilli(),
	)

	body, err := a.doGet(ctx, urlStr, qtRefer)
	if err != nil {
		return nil, err
	}

	return parseNewFqKLineResponse(body, tc, period, adjKey, startYear)
}

// ========== 响应解析 ==========

// parseNewFqKLineResponse 解析腾讯 newfqkline 的 JSONP 响应
//
// 两阶段解析:
//  1. 外层 envelope → 拿到 stock 原始数据
//  2. 股票数据 → 找到 K线数组键 → 逐行解析
//
// 数据键匹配策略:
//   - year > 0: 带年份后缀 {adjKey}{period}{year} (如 qfqday2021) — 日期范围请求
//   - year = 0: 不带后缀 {adjKey}{period} (如 qfqmonth) — 全量请求
//   - 回退: 优先尝试有后缀 → 无后缀
func parseNewFqKLineResponse(body, tc, period, adjKey string, year int) ([]adapter.StockPriceDaily, error) {
	jsonStr := extractJSONPFromNewFq(body)
	if jsonStr == "" {
		return nil, fmt.Errorf("K线响应为空或格式异常: %q", truncateStr(body, 200))
	}

	var env newFqKLineEnvelope
	if err := json.Unmarshal([]byte(jsonStr), &env); err != nil {
		return nil, fmt.Errorf("K线响应JSON解析失败: %w", err)
	}
	if env.Code != 0 {
		return nil, fmt.Errorf("K线API返回错误 code=%d", env.Code)
	}

	// 定位股票数据
	stockRaw, ok := env.Data[tc]
	if !ok {
		var found bool
		for k, v := range env.Data {
			if strings.HasSuffix(k, tc[2:]) || k == strings.ToUpper(tc) {
				stockRaw = v
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("K线响应中无股票 %s 的数据", tc)
		}
	}

	var stockData map[string]json.RawMessage
	if err := json.Unmarshal(stockRaw, &stockData); err != nil {
		return nil, fmt.Errorf("解析股票K线数据失败: %w", err)
	}

	// 数据键匹配
	var dataKey string
	var klineRaw json.RawMessage
	var found bool
	if year > 0 {
		dataKey = fmt.Sprintf("%s%s%d", adjKey, period, year)
		klineRaw, found = stockData[dataKey]
	}
	if !found {
		dataKey = adjKey + period
		klineRaw, found = stockData[dataKey]
		if !found {
			return nil, fmt.Errorf("K线响应中无数据 (key=%s, available: %v)", dataKey, mapKeys(stockData))
		}
	}

	return parseKLineRows(klineRaw, tc)
}

// parseKLineRows 解析 K线行数组 → []StockPriceDaily
func parseKLineRows(klineRaw json.RawMessage, tc string) ([]adapter.StockPriceDaily, error) {
	var rows []json.RawMessage
	if err := json.Unmarshal(klineRaw, &rows); err != nil {
		return nil, fmt.Errorf("解析K线行数组失败: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("K线数组为空")
	}

	result := make([]adapter.StockPriceDaily, 0, len(rows))
	for _, rowRaw := range rows {
		var fields []interface{}
		if err := json.Unmarshal(rowRaw, &fields); err != nil {
			continue
		}
		if len(fields) < 6 {
			continue
		}

		getStr := func(idx int) string {
			if idx >= len(fields) {
				return ""
			}
			switch v := fields[idx].(type) {
			case string:
				return v
			case float64:
				return fmt.Sprintf("%.2f", v)
			default:
				return ""
			}
		}

		result = append(result, adapter.StockPriceDaily{
			Code:     tc,
			Date:     getStr(0),
			Open:     helpers.ParsePriceToCents(getStr(1)),
			Close:    helpers.ParsePriceToCents(getStr(2)),
			High:     helpers.ParsePriceToCents(getStr(3)),
			Low:      helpers.ParsePriceToCents(getStr(4)),
			Volume:   parseInt(getStr(5)),
			Turnover: parseFloat(getStr(7)),
			Amount:   helpers.ParseWanYuanToCents(getStr(8)),
		})
	}

	return result, nil
}

// extractFSStartDate 从一次探测响应中提取 fsStartDate
//
// 响应中 stockData 包含 "fsStartDate": "2020-03-20" 字段。
func extractFSStartDate(body, tc string) string {
	jsonStr := extractJSONPFromNewFq(body)
	if jsonStr == "" {
		return ""
	}

	var env newFqKLineEnvelope
	if err := json.Unmarshal([]byte(jsonStr), &env); err != nil {
		return ""
	}

	stockRaw, ok := env.Data[tc]
	if !ok {
		for _, v := range env.Data {
			stockRaw = v
			break
		}
	}

	var stockData map[string]json.RawMessage
	if err := json.Unmarshal(stockRaw, &stockData); err != nil {
		return ""
	}

	var fsDate string
	if raw, ok := stockData["fsStartDate"]; ok {
		json.Unmarshal(raw, &fsDate)
	}
	return fsDate
}

// ========== 辅助函数 ==========

// sortDates 日期字符串升序排序（格式 YYYY-MM-DD 可直接字典序比较）
func sortDates(dates []string) {
	for i := 0; i < len(dates); i++ {
		for j := i + 1; j < len(dates); j++ {
			if dates[i] > dates[j] {
				dates[i], dates[j] = dates[j], dates[i]
			}
		}
	}
}

// sortByDate 将 date→item map 转为日期升序的 slice
func sortByDate(m map[string]adapter.StockPriceDaily) []adapter.StockPriceDaily {
	dates := make([]string, 0, len(m))
	for d := range m {
		dates = append(dates, d)
	}
	sortDates(dates)
	result := make([]adapter.StockPriceDaily, len(dates))
	for i, d := range dates {
		item := m[d]
		result[i] = item
	}
	return result
}

// extractJSONPFromNewFq 从腾讯 newfqkline 响应中提取 JSON 部分
func extractJSONPFromNewFq(body string) string {
	body = strings.TrimSpace(body)
	start := strings.IndexByte(body, '{')
	if start < 0 {
		return ""
	}
	end := strings.LastIndexByte(body, '}')
	if end < 0 || end <= start {
		return ""
	}
	return body[start : end+1]
}

// mapKeys 提取 map 的键列表，用于诊断日志
func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
