//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/tencentstock"
	"stock-ai/internal/api/handler"
	"stock-ai/internal/api/router"
	"stock-ai/internal/backtest"
	"stock-ai/internal/config"
	"stock-ai/internal/datacollect"
	"stock-ai/internal/indicator"
	"stock-ai/internal/subscription/monitor"
	"stock-ai/internal/notifier"
	"stock-ai/internal/subscription/quotecache"
	"stock-ai/internal/subscription/runner"
	subsched "stock-ai/internal/subscription/scheduler"
	"stock-ai/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

// ============================================================================
//  Provider 辅助函数
// ============================================================================

func provideRegistry() *adapter.Registry {
	return adapter.GetRegistry()
}

func provideQuoteSubscriber(cache quotecache.QuoteCache) quotecache.QuoteSubscriber {
	// model.QuoteSubscriber = quotecache.QuoteSubscriber (type alias), zero copy
	return cache.Subscriber()
}

func provideEngine(reg *indicator.Registry) *indicator.Engine {
	return reg.Engine()
}

func provideRouter(dcRunner *datacollect.DataCollectRunner, btHandler *backtest.Handler) *gin.Engine {
	router.BacktestHandlerRef = btHandler
	return router.SetupRouter(dcRunner)
}

// provideQuoteCacheConfig 构建 quotecache.Config。
// Registry 中的 adapter 在此转换为 CollectorChain 注入到底层包（依赖反转）。
func provideQuoteCacheConfig(reg *adapter.Registry) quotecache.Config {
	var chain quotecache.CollectorChain
	if ds, ok := reg.Get(tencentstock.AdapterName); ok {
		chain = append(chain, &collectorAdapter{ds: ds})
	}
	return quotecache.Config{
		Collector: chain,
		IsActive: utils.IsTradingHours,
	}
}

// collectorAdapter 将 adapter.DataSource 适配为 quotecache.IntradayCollector。
type collectorAdapter struct {
	ds adapter.DataSource
}

func (a *collectorAdapter) GetIntraday(ctx context.Context, code string) (*quotecache.MinuteData, error) {
	// Try GetIntraday first (tencentstock supports it)
	id, err := a.ds.GetIntraday(ctx, code)
	if err == nil && id != nil && len(id.Bars) > 0 {
		return convertAdapterIntraday(id), nil
	}

	// Fallback: GetTodayData as single-bar intraday
	daily, err := a.ds.GetTodayData(ctx, code)
	if err != nil {
		return nil, err
	}
	return convertDailyToIntraday(daily), nil
}

func convertAdapterIntraday(id *adapter.IntradayData) *quotecache.MinuteData {
	bars := make([]quotecache.MinuteBar, len(id.Bars))
	for i, b := range id.Bars {
		bars[i] = quotecache.MinuteBar{
			Time:   b.Time,
			Price:  b.Price,
			Volume: b.Volume,
			Amount: b.Amount,
		}
	}
	md := &quotecache.MinuteData{
		Bars:     bars,
		PreClose: id.PreClose,
		Date:     id.Date,

		Name:           id.Name,
		Current:        id.Current,
		High:           id.High,
		Low:            id.Low,
		Volume:         id.Volume,
		Amount:         id.Amount,
		Change:         id.Change,
		ChangePct:      id.ChangePct,
		Turnover:       id.Turnover,
		Pe:             id.Pe,
		Pb:             id.Pb,
		MarketCap:      id.MarketCap,
		FloatMarketCap: id.FloatMarketCap,
		Amplitude:      id.Amplitude,
	}
	if id.Depth != nil {
		md.Depth = &quotecache.MarketDepth{
			Ask1Price: id.Depth.Ask1Price, Ask1Volume: id.Depth.Ask1Volume,
			Ask2Price: id.Depth.Ask2Price, Ask2Volume: id.Depth.Ask2Volume,
			Ask3Price: id.Depth.Ask3Price, Ask3Volume: id.Depth.Ask3Volume,
			Ask4Price: id.Depth.Ask4Price, Ask4Volume: id.Depth.Ask4Volume,
			Ask5Price: id.Depth.Ask5Price, Ask5Volume: id.Depth.Ask5Volume,
			Bid1Price: id.Depth.Bid1Price, Bid1Volume: id.Depth.Bid1Volume,
			Bid2Price: id.Depth.Bid2Price, Bid2Volume: id.Depth.Bid2Volume,
			Bid3Price: id.Depth.Bid3Price, Bid3Volume: id.Depth.Bid3Volume,
			Bid4Price: id.Depth.Bid4Price, Bid4Volume: id.Depth.Bid4Volume,
			Bid5Price: id.Depth.Bid5Price, Bid5Volume: id.Depth.Bid5Volume,
		}
	}
	return md
}

func convertDailyToIntraday(daily *adapter.StockPriceDaily) *quotecache.MinuteData {
	preClose := daily.Close - int64(daily.ChangePct*float64(daily.Close)/100)
	return &quotecache.MinuteData{
		PreClose: preClose,
		Date:     daily.Date,
		Bars: []quotecache.MinuteBar{{
			Time:   "15:00",
			Price:  daily.Close,
			Volume: daily.Volume,
			Amount: daily.Amount,
		}},
	}
}

// ============================================================================
//  Wire 注入器入口
// ============================================================================

func InitializeApp(cfg *config.Config) (*App, error) {
	wire.Build(
		provideRegistry,
		provideQuoteSubscriber,
		provideEngine,
		provideRouter,
		provideQuoteCacheConfig,

		// --- 行情缓存 ---
		quotecache.New,

		// --- 缓存 Stock 构建 ---
		runner.NewCachedQuoteProvider,

		// --- 指标引擎 ---
		handler.AllBuiltins,
		indicator.NewRegistry,

		// --- 通知器 ---
		notifier.NewNotifier,

		// --- 策略订阅 ---
		runner.NewSubscriptionRunner,
		subsched.NewScheduler,

		// --- 数据采集 ---
		datacollect.NewDataCollectRunner,
		wire.Bind(new(datacollect.TaskRunner), new(*datacollect.DataCollectRunner)),
		datacollect.NewScheduler,

		// --- 回测 ---
		backtest.NewService,
		backtest.NewHandler,

		// --- 盯盘监控 ---
		monitor.NewMonitor,

		wire.Struct(new(App), "*"),
	)
	return &App{}, nil
}
