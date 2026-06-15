package tencentstock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/helpers"
)

// ========== K线数据 - 腾讯 newfqkline API ==========
//
// API: https://proxy.finance.qq.com/ifzqgtimg/appstock/app/newfqkline/get
//
// 请求格式 (无日期范围，全量拉取):
//
//	_var=kline_{period}{adj}           // JSONP回调名
//	param={code},{period},,,640,{adj}  // code/周期/空/空/条数(最多640)/复权方式
//
// 注意：API 忽略日期范围参数，始终返回全量数据（最多640条）。
// 因此采集策略为：单次全量拉取640条，若满640条则用首条日期-1天作为end继续向前补拉。
//
// K线数组每条记录格式 (10个元素，混合类型):
//
//	[0] 日期          "2020-03-20"
//	[1] 开盘价(元)     "40.69"
//	[2] 收盘价(元)     "38.19"
//	[3] 最高价(元)     "41.05"
//	[4] 最低价(元)     "37.11"
//	[5] 成交量(手)     "73506.85"  — 与东财(手)、THS(股)不同，需 ×100→股
//	[6] 除权事件       {} 或 {"nd":"2019","fh_sh":"2","djr":"2020-07-09",...}
//	[7] 换手率(%)     "19.75"
//	[8] 成交额(万元)   "28433.88"  — 与东财(元)不同，需 ParseWanYuanToCents
//	[9] 交易天数       "5"
//
// 采集字段: [0]Date [1]Open [2]Close [3]High [4]Low [5]Volume [7]Turnover [8]Amount

// newFqKLineURL 腾讯复权K线接口地址
const newFqKLineURL = "https://proxy.finance.qq.com/ifzqgtimg/appstock/app/newfqkline/get"

// ========== 公共接口实现 ==========

// maxKLinesPerRequest newfqkline API 单次最大返回条数
const maxKLinesPerRequest = 640

// GetDailyKLine 获取日K线（单次全量拉取，满640条则向前补拉）
func (a *Adapter) GetDailyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	tc := toTencentCode(code)
	adjParam, adjKey := mapAdjType(adjType)
	return a.fetchKLinesPaginated(ctx, tc, "day", adjParam, adjKey)
}

// GetWeeklyKLine 获取周K线（单次全量拉取，满640条则向前补拉）
func (a *Adapter) GetWeeklyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	tc := toTencentCode(code)
	adjParam, adjKey := mapAdjType(adjType)
	return a.fetchKLinesPaginated(ctx, tc, "week", adjParam, adjKey)
}

// GetMonthlyKLine 获取月K线（单次全量拉取，满640条则向前补拉）
func (a *Adapter) GetMonthlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	tc := toTencentCode(code)
	adjParam, adjKey := mapAdjType(adjType)
	return a.fetchKLinesPaginated(ctx, tc, "month", adjParam, adjKey)
}

// GetQuarterlyKLine 获取季K线（腾讯 newfqkline API 不支持 quarter 周期）
func (a *Adapter) GetQuarterlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

// GetYearlyKLine 获取年K线（单次全量拉取，320条足够）
func (a *Adapter) GetYearlyKLine(ctx context.Context, code, adjType string) ([]adapter.StockPriceDaily, error) {
	tc := toTencentCode(code)
	adjParam, adjKey := mapAdjType(adjType)
	return a.fetchKLinesNoLimit(ctx, tc, "year", adjParam, adjKey, 320)
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

// fetchKLinesPaginated 分页拉取全量K线（单次最多640条，满则向前补拉）
//
// 腾讯 newfqkline API 忽略日期范围参数，始终返回全量数据（最近N条）。
// 因此：先拉640条，若满则用首条日期-1天作为end继续向前拉，直到不满640条或到底。
func (a *Adapter) fetchKLinesPaginated(ctx context.Context, tc, period, adjParam, adjKey string) ([]adapter.StockPriceDaily, error) {
	dateMap := make(map[string]adapter.StockPriceDaily)
	endDate := "" // 首次不设end，拉最新640条

	for {
		data, err := a.fetchKLinesBatch(ctx, tc, period, adjParam, adjKey, maxKLinesPerRequest, endDate)
		if err != nil {
			if len(dateMap) == 0 {
				return nil, err
			}
			break // 已有部分数据，不再继续
		}

		for _, item := range data {
			if existing, ok := dateMap[item.Date]; !ok || item.Date > existing.Date {
				dateMap[item.Date] = item
			}
		}

		// 不满640条 → 已到底，结束
		if len(data) < maxKLinesPerRequest {
			break
		}

		// 用首条日期-1天作为下次请求的end
		endDate = data[0].Date
		if endDate <= "1990-01-01" {
			break
		}
	}

	if len(dateMap) == 0 {
		return nil, fmt.Errorf("K线数据为空 (%s/%s)", tc, period)
	}

	return sortByDate(dateMap), nil
}

// fetchKLinesNoLimit 单次全量拉取（不分页，年K用）
func (a *Adapter) fetchKLinesNoLimit(ctx context.Context, tc, period, adjParam, adjKey string, limit int) ([]adapter.StockPriceDaily, error) {
	return a.fetchKLinesBatch(ctx, tc, period, adjParam, adjKey, limit, "")
}

// fetchKLinesBatch 单次K线请求
// endDate: 截止日期（不含），格式 YYYY-MM-DD；空字符串表示拉最新数据
func (a *Adapter) fetchKLinesBatch(ctx context.Context, tc, period, adjParam, adjKey string, limit int, endDate string) ([]adapter.StockPriceDaily, error) {
	var urlStr string
	if endDate == "" {
		urlStr = fmt.Sprintf(
			"%s?_var=kline_%s%s&param=%s,%s,,,%d,%s&r=%d",
			newFqKLineURL, period, adjKey, tc, period, limit, adjParam, time.Now().UnixMilli(),
		)
	} else {
		urlStr = fmt.Sprintf(
			"%s?_var=kline_%s%s&param=%s,%s,,%s,%d,%s&r=%d",
			newFqKLineURL, period, adjKey, tc, period, endDate, limit, adjParam, time.Now().UnixMilli(),
		)
	}

	body, err := a.doGet(ctx, urlStr, qtRefer)
	if err != nil {
		return nil, fmt.Errorf("fetchKLinesBatch(%s) 请求失败: %w", period, err)
	}

	return parseNewFqKLineResponse(body, tc, period, adjKey)
}

// ========== 响应解析 ==========

// parseNewFqKLineResponse 解析腾讯 newfqkline 的 JSONP 响应
//
// 两阶段解析:
//  1. 外层 envelope → 拿到 stock 原始数据
//  2. 股票数据 → 找到 K线数组键（{adjKey}{period}，如 qfqday）→ 逐行解析
func parseNewFqKLineResponse(body, tc, period, adjKey string) ([]adapter.StockPriceDaily, error) {
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

	// 数据键: {adjKey}{period}（如 qfqday）
	dataKey := adjKey + period
	klineRaw, found := stockData[dataKey]
	if !found {
		return nil, fmt.Errorf("K线响应中无数据 (key=%s, available: %v)", dataKey, mapKeys(stockData))
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

		// 提取纯数字代码（与东财/THS一致），腾讯内部用 sz000422 格式
		_, symbol := splitTencentCode(tc)
		result = append(result, adapter.StockPriceDaily{
			Code:     symbol,
			Date:     getStr(0),
			Open:     helpers.ParsePriceToCents(getStr(1)),
			Close:    helpers.ParsePriceToCents(getStr(2)),
			High:     helpers.ParsePriceToCents(getStr(3)),
			Low:      helpers.ParsePriceToCents(getStr(4)),
			Volume:   parseInt(getStr(5)) * 100, // 手→股（与东财一致，东财也是手×100）
			Turnover: parseFloat(getStr(7)),
			Amount:   helpers.ParseWanYuanToCents(getStr(8)), // 万元→分
		})
	}

	return result, nil
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
