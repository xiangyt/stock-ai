package eastmoney

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
		limiter:      rate.NewLimiter(rate.Limit(20), 20),
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
