package eastmoney

import (
	"context"
	"strings"
	"testing"

	"stock-ai/internal/adapter"
)

// ========== 测试公共辅助 ==========

// 测试用股票代码
const (
	testCode = "000001" // 平安银行
)

func newTestAdapter() *Adapter {
	a := New()
	_ = a.Init(map[string]interface{}{
		"cookie": "", // todo
	})
	return a
}

// validateKLines 验证K线数据的完整性
func validateKLines(t *testing.T, klines []adapter.StockPriceDaily, period string) {
	for i := 1; i < len(klines); i++ {
		if strings.Compare(klines[i].Date, klines[i-1].Date) <= 0 {
			t.Errorf("[%s] 时间顺序异常: [%d]=%s >= [%d]=%s",
				period, i-1, klines[i-1].Date, i, klines[i].Date)
		}
	}
	for i, p := range klines {
		isSpecialPrice := p.Open < 0 || p.Close < 0 || p.High < 0 || p.Low < 0 || p.Low == 0
		if isSpecialPrice {
			continue
		}
		if p.High < p.Low {
			t.Errorf("[%s] 第%d条(%s) High(%d) < Low(%d)",
				period, i, p.Date, p.High, p.Low)
		}
		if p.High < p.Open || p.High < p.Close {
			t.Errorf("[%s] 第%d条(%s) 非最高价: O=%d H:%d L:%d C:%d",
				period, i, p.Date, p.Open, p.High, p.Low, p.Close)
		}
		if p.Low > p.Open || p.Low > p.Close {
			t.Errorf("[%s] 第%d条(%s) 非最低价: O=%d H:%d L:%d C:%d",
				period, i, p.Date, p.Open, p.High, p.Low, p.Close)
		}
		if p.Volume < 0 {
			t.Errorf("[%s] 第%d条(%s) Volume=%d < 0", period, i, p.Date, p.Volume)
		}
		if p.Amount < 0 {
			t.Errorf("[%s] 第%d条(%s) Amount=%d < 0", period, i, p.Date, p.Amount)
		}
	}
}

// mustGetTodayData 获取当日数据，失败则 t.Fatal（供 realtime 测试使用）
func mustGetTodayData(t *testing.T, code string) *adapter.StockPriceDaily {
	t.Helper()
	a := newTestAdapter()
	defer a.Close()
	ctx := context.Background()
	data, name, err := a.GetTodayData(ctx, code)
	if err != nil {
		t.Fatalf("GetTodayData(%s) failed: %v", code, err)
	}
	if data == nil {
		t.Fatalf("GetTodayData(%s) returned nil", code)
	}
	t.Logf("%s(%s) 当日数据: O:%d H:%d L:%d C:%d Vol:%d", name, code,
		data.Open, data.High, data.Low, data.Close, data.Volume)
	return data
}

// mustGetPeriodData 获取周期数据的通用 helper
func mustGetPeriodData(t *testing.T, code string, fn func(context.Context, string) (*adapter.StockPriceDaily, error)) *adapter.StockPriceDaily {
	t.Helper()
	a := newTestAdapter()
	defer a.Close()
	data, err := fn(context.Background(), code)
	if err != nil {
		t.Fatalf("failed for %s: %v", code, err)
	}
	return data
}
