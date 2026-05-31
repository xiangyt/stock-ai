package ths2

import (
	"os"
	"strings"
	"testing"

	"stock-ai/internal/adapter"
)

// 测试用股票代码
const (
	testCode   = "002404" // 平安银行（深市）
	testCodeSH = "600519" // 贵州茅台（沪市）
)

// newTestAdapter 创建带环境变量认证的测试适配器
func newTestAdapter() *Adapter {
	a := New()
	initConfig := map[string]interface{}{}
	if cookie := os.Getenv("THS2_COOKIE"); cookie != "" {
		initConfig["cookie"] = cookie
	}
	if auth := os.Getenv("THS2_FUYAO_AUTH"); auth != "" {
		initConfig["fuyao_auth"] = auth
	}
	_ = a.Init(initConfig)
	return a
}

// skipIfNoAuth 如果没有配置认证信息，跳过测试
func skipIfNoAuth(t *testing.T) {
	t.Helper()
	if os.Getenv("THS2_COOKIE") == "" || os.Getenv("THS2_FUYAO_AUTH") == "" {
		t.Skip("跳过集成测试: 需要设置 THS2_COOKIE 和 THS2_FUYAO_AUTH 环境变量")
	}
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
		// 跳过特殊价格（停牌/异常）
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
