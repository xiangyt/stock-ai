package ths2

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"stock-ai/internal/adapter"

	"golang.org/x/time/rate"
)

// K线周期常量 — ths2 新接口使用 time_period 参数
const (
	TimePeriodDaily   = "day_1"   // 日K
	TimePeriodWeekly  = "week_1"  // 周K
	TimePeriodMonthly = "month_1" // 月K
)

// AdapterName 数据源注册名称
const AdapterName = "ths2"

// Adapter 同花顺 v2 数据源适配器（quota-h API）
type Adapter struct {
	config    map[string]interface{}
	client    *http.Client
	limiter   *rate.Limiter
	quota     adapter.QuotaInfo
	cookie    string // Cookie 字符串（含 userid + v）
	fuyaoAuth string // x-fuyao-auth JWT token
}

// New 创建 ths2 适配器
func New() *Adapter {
	q := adapter.QuotaInfo{
		DailyLimit: -1,
		RateLimit:  200,
		Burst:      400,
	}
	r, burst := q.LimiterConfig()
	return &Adapter{
		config:  make(map[string]interface{}),
		client:  &http.Client{Timeout: 10 * time.Second},
		limiter: rate.NewLimiter(rate.Limit(r), burst),
		quota:   q,
	}
}

func (a *Adapter) Name() string        { return AdapterName }
func (a *Adapter) DisplayName() string { return "同花顺v2" }
func (a *Adapter) Type() string        { return "web_crawl" }

// Init 初始化配置
// config["cookie"] → Cookie 字符串
// config["fuyao_auth"] → x-fuyao-auth JWT token
func (a *Adapter) Init(config map[string]interface{}) error {
	a.config = config
	if cookie, ok := config["cookie"].(string); ok {
		a.cookie = cookie
	}
	if auth, ok := config["fuyao_auth"].(string); ok {
		a.fuyaoAuth = auth
	}
	return nil
}

func (a *Adapter) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://quota-h.10jqka.com.cn", nil)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	log.Println("✅ 同花顺v2数据源连接正常")
	return nil
}

func (a *Adapter) Close() error { return nil }

func (a *Adapter) GetQuotaInfo() adapter.QuotaInfo { return a.quota }

// ========== 未实现的方法 ==========

func (a *Adapter) GetStockList(_ context.Context) ([]adapter.StockBasic, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetStockDetail(_ context.Context, _ string) (*adapter.StockBasic, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetQuarterlyKLine(_ context.Context, _ string, _ string) ([]adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetYearlyKLine(_ context.Context, _ string, _ string) ([]adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetIndexDailyKLine(_ context.Context, _ string, _, _ time.Time, _ string) ([]adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetTodayData(_ context.Context, _ string) (*adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetThisWeekData(_ context.Context, _ string) (*adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetThisMonthData(_ context.Context, _ string) (*adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetThisQuarterData(_ context.Context, _ string) (*adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetThisYearData(_ context.Context, _ string) (*adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetPerformanceReports(_ context.Context, _ string) ([]adapter.PerformanceReport, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetLatestPerformanceReport(_ context.Context, _ string) (*adapter.PerformanceReport, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetShareholderCounts(_ context.Context, _ string) ([]adapter.ShareholderCount, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetLatestShareholderCount(_ context.Context, _ string) (*adapter.ShareholderCount, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetShareChanges(_ context.Context, _ string) ([]adapter.ShareChange, error) {
	return nil, adapter.ErrNotImplemented
}

func (a *Adapter) GetInstitutionalHoldings(_ context.Context, _ string) ([]adapter.InstitutionalHolding, error) {
	return nil, adapter.ErrNotImplemented
}
