package ths

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/helpers"

	"golang.org/x/time/rate"
)

// K线类型常量 - 同花顺采集器使用
const (
	KLineTypeDaily     = "0" // 日K线
	KLineTypeWeekly    = "1" // 周K线
	KLineTypeMonthly   = "2" // 月K线
	KLineTypeQuarterly = "9" // 季K线
	KLineTypeYearly    = "8" // 年K线
)

// Adapter 同花顺数据源适配器（参考 stock 项目 tonghuashun_collector.go 重写）
type Adapter struct {
	config         map[string]interface{}
	client         *http.Client
	parser         *helpers.KLineParser
	limiter        *rate.Limiter
	quota          adapter.QuotaInfo
	userAgentGen   *helpers.UserAgentGenerator
	currentUA      string
	lastUpdateTime time.Time
}

// New 创建同花顺数据源适配器
func New() *Adapter {
	q := adapter.QuotaInfo{
		DailyLimit: -1,
		RateLimit:  100,
		Burst:      200,
	}
	r, burst := q.LimiterConfig()
	return &Adapter{
		config:       make(map[string]interface{}),
		client:       &http.Client{Timeout: 10 * time.Second},
		parser:       helpers.NewKLineParser(),
		limiter:      rate.NewLimiter(rate.Limit(r), burst),
		quota:        q,
		userAgentGen: helpers.NewUserAgentGenerator(),
	}
}

func (a *Adapter) Name() string        { return "ths" }
func (a *Adapter) DisplayName() string { return "同花顺" }
func (a *Adapter) Type() string        { return "web_crawl" }

func (a *Adapter) Init(config map[string]interface{}) error {
	a.config = config
	a.updateUserAgent()
	return nil
}

func (a *Adapter) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://d.10jqka.com.cn", nil)
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
	log.Println("✅ 同花顺数据源连接正常")
	return nil
}

func (a *Adapter) Close() error { return nil }

func (a *Adapter) GetQuotaInfo() adapter.QuotaInfo { return a.quota }

// ========== 财务数据（空实现）==========

func (a *Adapter) GetPerformanceReports(_ context.Context, _ string) ([]adapter.PerformanceReport, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *Adapter) GetLatestPerformanceReport(_ context.Context, _ string) (*adapter.PerformanceReport, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *Adapter) GetShareholderCounts(_ context.Context, _ string) ([]adapter.ShareholderCount, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *Adapter) GetLatestShareholderCount(_ context.Context, _ string) (*adapter.ShareholderCount, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *Adapter) GetShareChanges(_ context.Context, _ string) ([]adapter.ShareChange, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *Adapter) GetInstitutionalHoldings(_ context.Context, _ string) ([]adapter.InstitutionalHolding, error) {
	return nil, fmt.Errorf("not implemented")
}
