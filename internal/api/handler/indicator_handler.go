package handler

import (
	"net/http"
	"sync"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/db"
	"stock-ai/internal/service"
	"stock-ai/utils"

	"github.com/gin-gonic/gin"
)

// resolveTradeDate 将请求中的日期字符串解析为实际交易日 YYYYMMDD。
// date 为空时取最近交易日，非空时对齐到该日期之前最近的交易日。
func resolveTradeDate(date string) (int, error) {
	if date == "" {
		return db.GetLatestDailyKlineDate()
	}
	parsed, err := utils.ParseDateToTradeDate(date)
	if err != nil {
		return db.GetLatestDailyKlineDate()
	}
	return db.FindNearestDailyKlineDate(parsed)
}

// IndicatorHandler 指标 HTTP Handler
type IndicatorHandler struct {
	registry  *indicator.Registry
	screenSvc *service.ScreenService // 用于构建选股/回测引擎输入数据
}

// NewIndicatorHandler 创建指标 Handler（初始化全部内置指标）
func NewIndicatorHandler(screenSvc *service.ScreenService) *IndicatorHandler {
	reg := indicator.NewRegistry(allBuiltins())
	return &IndicatorHandler{registry: reg, screenSvc: screenSvc}
}

// Registry 返回底层指标注册表（供命令行工具等非 HTTP 场景使用）
func (h *IndicatorHandler) Registry() *indicator.Registry {
	return h.registry
}

// ListIndicators 获取全部指标元数据
// GET /api/v1/indicators
func (h *IndicatorHandler) ListIndicators(c *gin.Context) {
	meta := h.registry.ToAPIMeta()

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"categories":   meta.Categories,
			"indicators":   meta.Indicators,
			"enum_options": meta.EnumOptions,
		},
	})
}

// GetIndicatorByID 获取单个指标详情
// GET /api/v1/indicators/:id
func (h *IndicatorHandler) GetIndicatorByID(c *gin.Context) {
	id := c.Param("id")
	ind, ok := h.registry.GetIndicatorByID(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "指标不存在: " + id})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": indicator.ToAPIMeta(ind)})
}

// ExecuteRequest 选股执行请求体
type ExecuteRequest struct {
	Configs        []*indicator.SignalConfig `json:"configs"`         // 信号配置列表
	MaxConcurrency int                       `json:"max_concurrency"` // 最大并发数 (默认10)
	Date           string                    `json:"date"`            // 选股日期
}

// EnrichedStock 前端展示用的富化结果（引擎 EvaluatedStock + 快照/行业展示字段）
type EnrichedStock struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Price    float64 `json:"price"`
	Result   int    `json:"result"`
	SignalID string `json:"signal_id"`
	Message  string `json:"message"`

	// K线计算
	ChangePct float64 `json:"change_pct"` // 涨跌幅(%)

	// 快照 — 估值
	PETTM              float64 `json:"pe_ttm"`               // 市盈率TTM
	PSTTM              float64 `json:"ps_ttm"`               // 市销率TTM
	PB                 float64 `json:"pb"`                    // 市净率
	CirculateMarketCap float64 `json:"circulate_market_cap"`  // 流通市值(元)
	TotalMarketCap     float64 `json:"total_market_cap"`      // 总市值(元)

	// 快照 — 盈利
	ROE        float64 `json:"roe"`          // 净资产收益率(%)
	ROA        float64 `json:"roa"`          // 总资产收益率(%)
	GrossMargin float64 `json:"gross_margin"` // 毛利率(%)
	NetMargin  float64 `json:"net_margin"`   // 净利率(%)

	// 快照 — 每股
	BVPS    float64 `json:"bvps"`     // 每股净资产
	BasicEPS float64 `json:"basic_eps"` // 基本每股收益

	// 快照 — 偿债
	DebtRatio float64 `json:"debt_ratio"` // 资产负债率(%)

	// 行业
	Industry string `json:"industry"` // 所属东财行业
	Sector   string `json:"sector"`   // 细分行业
}

// ExecuteResponse 选股执行响应
type ExecuteResponse struct {
	Total    int              `json:"total"`    // 输入股票总数
	Passed   []EnrichedStock  `json:"passed"`   // 通过列表（富化）
	Rejected []indicator.EvaluatedStock `json:"rejected"` // 未通过列表
}

// Execute 执行选股筛选
// POST /api/v1/indicators/execute
//
// 处理流程:
//  1. 从请求体解析 SignalConfig 列表
//  2. 调用 ScreenService.BuildAll() 获取当日有 K 线数据的股票
//  3. 调用 Engine.Execute() 并发评估
//  4. 按 Passed/Rejected 分组
//  5. 对 Passed 列表批量查库富化展示字段（涨跌幅/市盈率/行业）
func (h *IndicatorHandler) Execute(c *gin.Context) {
	var req ExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if len(req.Configs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "信号配置不能为空"})
		return
	}

	maxConcurrency := req.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}

	// 获取股票数据
	stocks, err := h.screenSvc.BuildAll(maxConcurrency, req.Date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取股票数据失败: " + err.Error()})
		return
	}

	if len(stocks) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": ExecuteResponse{
			Total:    0,
			Passed:   []EnrichedStock{},
			Rejected: []indicator.EvaluatedStock{},
		}})
		return
	}

	// 执行选股
	results := h.registry.Engine().Execute(stocks, req.Configs, maxConcurrency)

	// 解析交易日（用于富化展示字段）
	tradeDate, _ := resolveTradeDate(req.Date)

	// 分组
	var passedRaw []indicator.EvaluatedStock
	var rejected []indicator.EvaluatedStock
	for _, r := range results {
		switch r.Result {
		case indicator.ResultPassed:
			passedRaw = append(passedRaw, *r)
		default:
			rejected = append(rejected, *r)
		}
	}

	// 富化通过列表：查库补充涨跌幅/市盈率/行业（基于 tradeDate）
	enriched := enrichPassedStocks(passedRaw, tradeDate)

	c.JSON(http.StatusOK, gin.H{"data": ExecuteResponse{
		Total:    len(stocks),
		Passed:   enriched,
		Rejected: rejected,
	}})
}

// enrichPassedStocks 对通过列表并发查库补充展示字段。
// tradeDate 为选股日期 YYYYMMDD，涨跌幅和市盈率均基于该日期计算。
func enrichPassedStocks(passed []indicator.EvaluatedStock, tradeDate int) []EnrichedStock {
	if len(passed) == 0 {
		return nil
	}
	result := make([]EnrichedStock, len(passed))
	var mu sync.Mutex
	_ = utils.ConcurrentExec(passed, 50, func(i int, stock indicator.EvaluatedStock) error {
		enriched := EnrichedStock{
			Code:     stock.Code,
			Name:     stock.Name,
			Price:    stock.Price,
			Result:   stock.Result,
			SignalID: stock.SignalID,
			Message:  stock.Message,
		}

		// 涨跌幅：取 <= tradeDate 的最近2根K线计算
		if klines, err := db.FindDailyKlines(stock.Code, tradeDate, 2); err == nil && len(klines) >= 2 && klines[1].Close > 0 {
			enriched.ChangePct = float64(klines[0].Close-klines[1].Close) / float64(klines[1].Close) * 100
		}

		// 快照：取 <= tradeDate 的最近一条快照，填充12个字段
		if snap, err := db.FindLatestSnapshotBefore(stock.Code, tradeDate); err == nil && snap != nil {
			enriched.PETTM = snap.PETTM
			enriched.PSTTM = snap.PSTTM
			enriched.PB = snap.PB
			enriched.CirculateMarketCap = snap.CirculateMarketCap
			enriched.TotalMarketCap = snap.TotalMarketCap
			enriched.ROE = snap.ROE
			enriched.ROA = snap.ROA
			enriched.GrossMargin = snap.GrossMargin
			enriched.NetMargin = snap.NetMargin
			enriched.BVPS = snap.BVPS
			enriched.BasicEPS = snap.BasicEPS
			enriched.DebtRatio = snap.DebtRatio
		}

		// 行业
		if detail, err := db.FindStockByCode(stock.Code); err == nil {
			enriched.Industry = detail.Industry
			enriched.Sector = detail.Sector
		}

		mu.Lock()
		result[i] = enriched
		mu.Unlock()
		return nil
	})
	return result
}
