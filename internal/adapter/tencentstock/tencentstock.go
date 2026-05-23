// Package tencentstock 腾讯自选股数据源适配器
// 数据接口来源于腾讯证券（stock.finance.qq.com / ifzq.gtimg.cn / sqt.gtimg.cn）
// 接口类型：web_crawl（抓取腾讯公开行情API，无需登录）
package tencentstock

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

// AdapterName 注册名称（registry key）
const AdapterName = "tencentstock"

// 腾讯股票代码前缀 → 市场
const (
	MarketSH = "sh" // 上交所
	MarketSZ = "sz" // 深交所
	MarketBJ = "bj" // 北交所
)

// Adapter 腾讯自选股数据源适配器
type Adapter struct {
	config       map[string]interface{}
	client       *http.Client
	limiter      *rate.Limiter
	quota        adapter.QuotaInfo
	userAgentGen *helpers.UserAgentGenerator
	currentUA    string
	lastUAUpdate time.Time
}

// New 创建腾讯自选股数据源适配器
func New() *Adapter {
	q := adapter.QuotaInfo{
		DailyLimit: -1,
		RateLimit:  100, // 100rps，腾讯行情接口较宽松
		Burst:      500,
	}
	r, burst := q.LimiterConfig()
	return &Adapter{
		config:       make(map[string]interface{}),
		client:       &http.Client{Timeout: 15 * time.Second},
		limiter:      rate.NewLimiter(rate.Limit(r), burst),
		quota:        q,
		userAgentGen: helpers.NewUserAgentGenerator(),
	}
}

func (a *Adapter) Name() string        { return AdapterName }
func (a *Adapter) DisplayName() string { return "腾讯自选股" }
func (a *Adapter) Type() string        { return "web_crawl" }

func (a *Adapter) Init(config map[string]interface{}) error {
	a.config = config
	a.refreshUA()
	return nil
}

func (a *Adapter) TestConnection(ctx context.Context) error {
	// 测试腾讯行情接口连通性（拉取上证指数快照）
	url := "https://sqt.gtimg.cn/utf8/q=sh000001"
	body, err := a.doGet(ctx, url, "https://gu.qq.com/")
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	if len(body) < 10 {
		return fmt.Errorf("响应异常: %s", body)
	}
	log.Println("✅ 腾讯自选股数据源连接正常")
	return nil
}

func (a *Adapter) Close() error {
	return nil
}

func (a *Adapter) GetQuotaInfo() adapter.QuotaInfo {
	return a.quota
}

// refreshUA 刷新 UserAgent（每分钟更新一次）
func (a *Adapter) refreshUA() {
	if a.currentUA == "" || time.Since(a.lastUAUpdate) > time.Minute {
		a.currentUA = a.userAgentGen.GenerateUserAgent()
		a.lastUAUpdate = time.Now()
	}
}
