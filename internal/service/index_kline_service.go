package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/ths"
)

// ========== 指数日线 ==========

// 支持的指数代码 → 展示名（同花顺 zs_ 接口目前注册了这 4 个指数）
var indexNames = map[string]string{
	adapter.IndexSH000001: "上证指数",
	adapter.IndexSZ399001: "深证成指",
	adapter.IndexHS300:    "沪深300",
	adapter.IndexSH399006: "创业板指",
}

// IndexDailyPoint 指数单日行情（价格单位：元）
type IndexDailyPoint struct {
	Date  string  `json:"date"`   // YYYY-MM-DD
	Open  float64 `json:"open"`   // 开盘价
	High  float64 `json:"high"`   // 最高价
	Low   float64 `json:"low"`    // 最低价
	Close float64 `json:"close"`  // 收盘价
}

// IndexDailyResult 指数日线查询结果
type IndexDailyResult struct {
	Code  string            `json:"code"`  // 指数代码，如 000001
	Name  string            `json:"name"`  // 指数名称，如 上证指数
	Items []IndexDailyPoint `json:"items"` // 按日期升序
}

// IndexKLineService 提供指数日线查询（用于复盘时绘制同期大盘基准曲线）。
type IndexKLineService struct{}

// NewIndexKLineService 创建指数日线服务。
func NewIndexKLineService() *IndexKLineService {
	return &IndexKLineService{}
}

// Daily 查询指数在 [startDate, endDate] 区间内的日线（升序）。
//
// code 使用 adapter 中的指数常量（默认 IndexSH000001 上证指数）；
// 数据来自同花顺指数日线接口，跨年区间会自动按年拆分请求后合并。
func (s *IndexKLineService) Daily(ctx context.Context, code, startDate, endDate string) (*IndexDailyResult, error) {
	if code == "" {
		code = adapter.IndexSH000001
	}
	if _, ok := indexNames[code]; !ok {
		return nil, fmt.Errorf("不支持的指数代码: %s", code)
	}
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("起始日期无效: %w", err)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, fmt.Errorf("结束日期无效: %w", err)
	}
	if start.After(end) {
		return nil, fmt.Errorf("起始日期不能晚于结束日期")
	}

	klines, err := ths.New().GetIndexDailyKLine(ctx, code, start, end, adapter.AdjNone)
	if err != nil {
		return nil, fmt.Errorf("拉取%s日线失败: %w", indexNames[code], err)
	}

	items := make([]IndexDailyPoint, 0, len(klines))
	for _, k := range klines {
		if k.Close <= 0 {
			continue
		}
		items = append(items, IndexDailyPoint{
			Date:  k.Date,
			Open:  centsToYuan(int(k.Open)),
			High:  centsToYuan(int(k.High)),
			Low:   centsToYuan(int(k.Low)),
			Close: centsToYuan(int(k.Close)),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Date < items[j].Date })

	return &IndexDailyResult{
		Code:  code,
		Name:  indexNames[code],
		Items: items,
	}, nil
}
