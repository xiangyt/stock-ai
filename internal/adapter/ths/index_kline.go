package ths

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/helpers"
	"stock-ai/utils"
)

// ========== 指数K线数据 ==========

// thsIndexPrefixMap 指数代码 → 同花顺 zs_ 前缀映射（仅支持已知指数）
var thsIndexPrefixMap = map[string]string{
	adapter.IndexSH000001: "1A", // 上证指数  → zs_1A0001
	adapter.IndexSZ399001: "",   // 深证成指  → zs_399001
	adapter.IndexHS300:    "1B", // 沪深300  → zs_1B0300
	adapter.IndexSH399006: "",   // 创业板指  → zs_399006
}

// IndexCodeToTHS 指数代码 → 同花顺 zs_ 代码映射
//
// 仅对 thsIndexPrefixMap 中注册的 4 个指数做精准映射，
// 未注册的代码返回空字符串。
func IndexCodeToTHS(code string) string {
	code = strings.TrimPrefix(code, "SH.") // 兼容 tsCode 格式
	code = strings.TrimPrefix(code, "SZ.")

	prefix, ok := thsIndexPrefixMap[code]
	if !ok {
		return "" // 未注册的指数
	}

	if prefix == "" {
		// 深交所指数：原始代码直接拼接
		return "zs_" + code
	}
	return "zs_" + prefix + code[2:]
}

// yearTask 单年请求任务
type yearTask struct {
	year string
}

// GetIndexDailyKLine 获取指数日K线（时间区间）
//
// 自动按年拆分，并发请求各年份后合并结果。
func (a *Adapter) GetIndexDailyKLine(ctx context.Context, code string, startTime, endTime time.Time, adjType string) ([]adapter.StockPriceDaily, error) {
	zsCode := IndexCodeToTHS(code)
	if zsCode == "" {
		return nil, fmt.Errorf("unsupported index code: %s", code)
	}

	klineType := "0" + adjType // 0=日线 + 复权bit

	// 拆分为逐年任务
	tasks := a.splitYearTasks(startTime, endTime)
	if len(tasks) == 0 {
		return []adapter.StockPriceDaily{}, nil
	}

	// 并发请求每年数据，结果按任务顺序存入切片
	results := make([][]adapter.StockPriceDaily, len(tasks))
	err := utils.ConcurrentExec(tasks, 4, func(i int, t yearTask) error {
		url := fmt.Sprintf("https://d.10jqka.com.cn/v4/line/%s/%s/%s.js", zsCode, klineType, t.year)
		body, err := a.makeTHSRequest(context.Background(), url)
		if err != nil {
			return fmt.Errorf("year %s request failed: %w", t.year, err)
		}
		data, err := a.parseIndexKLineData(code, body)
		if err != nil {
			return fmt.Errorf("year %s parse failed: %w", t.year, err)
		}
		results[i] = data
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 合并并按时间区间过滤
	return mergeAndFilter(results, startTime.Format(time.DateOnly), endTime.Format(time.DateOnly)), nil
}

// splitYearTasks 将时间区间拆分为逐年任务
func (a *Adapter) splitYearTasks(start, end time.Time) []yearTask {
	if start.After(end) {
		return nil
	}

	startY := start.Year()
	endY := end.Year()
	count := endY - startY + 1
	tasks := make([]yearTask, count)
	for i := range tasks {
		tasks[i].year = strconv.Itoa(startY + i)
	}
	return tasks
}

// parseIndexKLineData 解析单年响应的 data 字段（内部方法，不含回调名剥离逻辑）
func (a *Adapter) parseIndexKLineData(code string, res string) ([]adapter.StockPriceDaily, error) {
	// 去除 JSONP 包装
	var response struct {
		Data string `json:"data"`
	}
	if err := unmarshalJSONP(&response, res); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	if response.Data == "" {
		return []adapter.StockPriceDaily{}, nil
	}

	items := strings.Split(response.Data, ";")
	result := make([]adapter.StockPriceDaily, 0, len(items))

	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		fields := strings.Split(item, ",")
		if len(fields) < 8 {
			continue
		}

		var data adapter.StockPriceDaily
		data.Code = code

		dateStr := fields[0]
		if len(dateStr) == 8 {
			data.Date = dateStr[:4] + "-" + dateStr[4:6] + "-" + dateStr[6:8]
		} else {
			continue
		}

		data.Open = helpers.ParsePriceToCents(fields[1])
		data.High = helpers.ParsePriceToCents(fields[2])
		data.Low = helpers.ParsePriceToCents(fields[3])
		data.Close = helpers.ParsePriceToCents(fields[4])

		data.Volume = parseInt64(fields[5])
		data.Amount = helpers.ParsePriceToCents(fields[6])
		data.Turnover = parseFloat(fields[7])

		result = append(result, data)
	}

	return result, nil
}

// mergeAndFilter 合并多年份数据并按时间区间过滤
//
// 前提：splitYearTasks 按时间顺序生成年份任务，ConcurrentExec 保持结果切片索引与任务一致，
// 且同花顺单年响应内数据本身按日期升序排列。因此直接按序拼接即可保证全局有序。
// 拼接后只保留 [startDate, endDate] 区间内的数据（闭区间），去除边界外数据。
func mergeAndFilter(yearResults [][]adapter.StockPriceDaily, startDate, endDate string) []adapter.StockPriceDaily {
	total := 0
	for _, r := range yearResults {
		total += len(r)
	}
	if total == 0 {
		return nil
	}

	merged := make([]adapter.StockPriceDaily, 0, total)
	for _, r := range yearResults {
		merged = append(merged, r...)
	}

	// 数据有序，二分找起点，线性扫到终点，一次切片完成过滤
	startIdx := sort.Search(len(merged), func(i int) bool { return merged[i].Date >= startDate })
	if startIdx >= len(merged) {
		return nil // 所有数据都在起始时间之前
	}
	endIdx := startIdx
	for i := startIdx; i < len(merged); i++ {
		if merged[i].Date > endDate {
			break
		}
		endIdx = i + 1
	}
	return merged[startIdx:endIdx]
}

// unmarshalJSONP 通用 JSONP 响应解析（去除回调包装后 Unmarshal）
//
// 支持三种格式：
//
//	{"data":"..."}                    — 纯JSON
//	({"data":"..."})                  — 括号包裹
//	callbackName({"data":"..."})      — 完整JSONP（同花顺实际返回格式）
func unmarshalJSONP(v interface{}, body string) error {
	body = strings.TrimSpace(body)

	// 去除 callbackName( 前缀：找到最后一个 ( 作为 JSON 起始
	if idx := strings.LastIndex(body, "("); idx >= 0 {
		body = body[idx+1:]
	}

	// 去除尾部 )
	if len(body) > 0 && body[len(body)-1] == ')' {
		body = body[:len(body)-1]
	}

	return json.Unmarshal([]byte(body), v)
}
