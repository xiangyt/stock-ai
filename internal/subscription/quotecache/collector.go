package quotecache

import "context"

// IntradayCollector 分时数据采集器。
//
// 这是 quotecache 包唯一的对外依赖。adapter 层实现此接口并注入，
// quotecache 本身不依赖任何上层包（依赖反转）。
type IntradayCollector interface {
	// GetIntraday 获取指定股票当日分时数据。
	// 返回的 MinuteData.Bars 按时间升序排列，Volume/Amount 为累计值。
	GetIntraday(ctx context.Context, code string) (*MinuteData, error)
}

// CollectorChain 多采集器优先级链。
// 按顺序尝试，第一个成功即返回。用于多数据源降级（如腾讯→同花顺→东方财富）。
type CollectorChain []IntradayCollector

// Collect 按链顺序采集，返回第一个成功结果。
func (c CollectorChain) Collect(ctx context.Context, code string) (*MinuteData, error) {
	var lastErr error
	for _, collector := range c {
		data, err := collector.GetIntraday(ctx, code)
		if err == nil && data != nil {
			return data, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, nil
}
