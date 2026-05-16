package handler

import (
	"math"
	"net/http"
	"strconv"

	"stock-ai/internal/db"
	"stock-ai/internal/model"

	"github.com/gin-gonic/gin"
)

// ========== 类型定义 ==========
type KLineQueryHandler struct{}

func NewKLineQueryHandler() *KLineQueryHandler {
	return &KLineQueryHandler{}
}

// KLineItem 纯K线数据（价格单位：元），不含指标
type KLineItem struct {
	Date   string  `json:"date"`            // YYYY-MM-DD
	Open   float64 `json:"open"`            // 开盘价
	High   float64 `json:"high"`            // 最高价
	Low    float64 `json:"low"`             // 最低价
	Close  float64 `json:"close"`           // 收盘价
	Volume int64   `json:"volume"`          // 成交量(股)
	Amount int64   `json:"amount"`          // 成交额(分)
}

// MAData 均线指标数据（数组，与 items 一一对应）
type MAData struct {
	MA5  []float64 `json:"ma5,omitempty"`
	MA10 []float64 `json:"ma10,omitempty"`
	MA20 []float64 `json:"ma20,omitempty"`
	MA60 []float64 `json:"ma60,omitempty"`
}

// MACDData MACD 指标数据
type MACDData struct {
	DIF  []float64 `json:"dif,omitempty"`
	DEA  []float64 `json:"dea,omitempty"`
	MACD []float64 `json:"macd,omitempty"`
}

// KDJData KDJ 指标数据
type KDJData struct {
	K []float64 `json:"k,omitempty"`
	D []float64 `json:"d,omitempty"`
	J []float64 `json:"j,omitempty"`
}

// IndicatorData 技术指标汇总
type IndicatorData struct {
	MA   MAData   `json:"ma,omitempty"`
	MACD MACDData `json:"macd,omitempty"`
	KDJ  KDJData  `json:"kdj,omitempty"`
}

// KLineResponse K线查询响应
type KLineResponse struct {
	Code       string         `json:"code"`
	Period     string         `json:"period"`
	Name       string         `json:"name,omitempty"`
	Items      []KLineItem    `json:"items"`
	Indicators *IndicatorData `json:"indicators"`
}

// centsToYuan 将分转换为元
func centsToYuan(cents int) float64 {
	return float64(cents) / 100.0
}

// GetKLine 获取股票 K 线数据(含技术指标)
//
// GET /api/v1/kline/:code?period=daily|weekly|monthly&limit=250
func (h *KLineQueryHandler) GetKLine(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "股票代码不能为空"})
		return
	}

	period := c.DefaultQuery("period", "daily")
	limitStr := c.DefaultQuery("limit", "250")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 1000 {
		limit = 250
	}

	var items []KLineItem
	var err error

	switch period {
	case "daily":
		items, err = fetchDailyKLines(code, limit)
	case "weekly":
		items, err = fetchWeeklyKLines(code, limit)
	case "monthly":
		items, err = fetchMonthlyKLines(code, limit)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的周期参数，支持: daily, weekly, monthly"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	if len(items) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"data": KLineResponse{Code: code, Period: period, Items: []KLineItem{}},
		})
		return
	}

	// 计算指标（返回独立数组，不污染K线数据）
	indicators := &IndicatorData{
		MA:   calcMA(items),
		MACD: calcMACD(items),
		KDJ:  calcKDJ(items),
	}

	c.JSON(http.StatusOK, gin.H{
		"data": KLineResponse{
			Code:       code,
			Period:     period,
			Items:      items,
			Indicators: indicators,
		},
	})
}

// ========== 日 K 线数据获取 ==========

func fetchDailyKLines(code string, limit int) ([]KLineItem, error) {
	rows, err := db.FindDailyKlines(code, 0, limit)
	if err != nil {
		return nil, err
	}
	return convertDailyKlines(rows), nil
}

func convertDailyKlines(rows []*model.DailyKline) []KLineItem {
	items := make([]KLineItem, len(rows))
	for i, r := range rows {
		items[i] = KLineItem{
			Date:   db.FormatTradeDate(r.TradeDate),
			Open:   centsToYuan(r.Open),
			High:   centsToYuan(r.High),
			Low:    centsToYuan(r.Low),
			Close:  centsToYuan(r.Close),
			Volume: r.Volume,
			Amount: r.Amount,
		}
	}
	// DB 返回 DESC，前端图表需要 ASC → 反转
	reverse(items)
	return items
}

// ========== 周 K 线数据获取 ==========

func fetchWeeklyKLines(code string, limit int) ([]KLineItem, error) {
	rows, err := db.FindWeeklyKlines(code, 0, limit)
	if err != nil {
		return nil, err
	}
	return convertWeeklyKlines(rows), nil
}

func convertWeeklyKlines(rows []*model.WeeklyKline) []KLineItem {
	items := make([]KLineItem, len(rows))
	for i, r := range rows {
		items[i] = KLineItem{
			Date:   db.FormatTradeDate(r.TradeDate),
			Open:   centsToYuan(r.Open),
			High:   centsToYuan(r.High),
			Low:    centsToYuan(r.Low),
			Close:  centsToYuan(r.Close),
			Volume: r.Volume,
			Amount: r.Amount,
		}
	}
	reverse(items)
	return items
}

// ========== 月 K 线数据获取 ==========

func fetchMonthlyKLines(code string, limit int) ([]KLineItem, error) {
	rows, err := db.FindMonthlyKlines(code, 0, limit)
	if err != nil {
		return nil, err
	}
	return convertMonthlyKlines(rows), nil
}

func convertMonthlyKlines(rows []*model.MonthlyKline) []KLineItem {
	items := make([]KLineItem, len(rows))
	for i, r := range rows {
		items[i] = KLineItem{
			Date:   db.FormatTradeDate(r.TradeDate),
			Open:   centsToYuan(r.Open),
			High:   centsToYuan(r.High),
			Low:    centsToYuan(r.Low),
			Close:  centsToYuan(r.Close),
			Volume: r.Volume,
			Amount: r.Amount,
		}
	}
	reverse(items)
	return items
}

// ========== 工具函数 ==========

// reverse 反转切片（DB 返回 DESC → ASC 用于图表渲染）
func reverse(items []KLineItem) {
	n := len(items)
	for i := 0; i < n/2; i++ {
		items[i], items[n-1-i] = items[n-1-i], items[i]
	}
}

// ========== 均线计算 ==========

func calcMA(items []KLineItem) MAData {
	n := len(items)
	ma5 := make([]float64, n)
	ma10 := make([]float64, n)
	ma20 := make([]float64, n)
	ma60 := make([]float64, n)

	for i := 0; i < n; i++ {
		if i >= 4 {
			ma5[i] = round2(avgClose(items, i, 5))
		} else {
			ma5[i] = 0
		}
		if i >= 9 {
			ma10[i] = round2(avgClose(items, i, 10))
		} else {
			ma10[i] = 0
		}
		if i >= 19 {
			ma20[i] = round2(avgClose(items, i, 20))
		} else {
			ma20[i] = 0
		}
		if i >= 59 {
			ma60[i] = round2(avgClose(items, i, 60))
		} else {
			ma60[i] = 0
		}
	}

	return MAData{MA5: ma5, MA10: ma10, MA20: ma20, MA60: ma60}
}

func avgClose(items []KLineItem, idx, period int) float64 {
	sum := 0.0
	start := idx - period + 1
	for j := start; j <= idx; j++ {
		sum += items[j].Close
	}
	return sum / float64(period)
}

// ========== MACD 计算 (12, 26, 9) ==========

func calcMACD(items []KLineItem) MACDData {
	n := len(items)
	difArr := make([]float64, n)
	deaArr := make([]float64, n)
	macdArr := make([]float64, n)

	if n == 0 {
		return MACDData{DIF: difArr, DEA: deaArr, MACD: macdArr}
	}

	short, long, mid := 12, 26, 9
	shortEMA := 0.0
	longEMA := 0.0
	emaAlphaShort := 2.0 / float64(short+1)
	emaAlphaLong := 2.0 / float64(long+1)
	emaAlphaMid := 2.0 / float64(mid+1)
	dea := 0.0
	first := true

	for i := 0; i < n; i++ {
		closePrice := items[i].Close

		if first {
			shortEMA = closePrice
			longEMA = closePrice
			first = false
		} else {
			shortEMA = closePrice*emaAlphaShort + shortEMA*(1-emaAlphaShort)
			longEMA = closePrice*emaAlphaLong + longEMA*(1-emaAlphaLong)
		}

		dif := shortEMA - longEMA

		if i == 0 {
			dea = dif
		} else {
			dea = dif*emaAlphaMid + dea*(1-emaAlphaMid)
		}

		macd := 2 * (dif - dea)

		difArr[i] = dif
		deaArr[i] = dea
		macdArr[i] = macd
	}

	return MACDData{DIF: difArr, DEA: deaArr, MACD: macdArr}
}

// ========== KDJ 计算 (9, 3, 3) ==========

func calcKDJ(items []KLineItem) KDJData {
	n := len(items)
	kArr := make([]float64, n)
	dArr := make([]float64, n)
	jArr := make([]float64, n)

	if n == 0 {
		return KDJData{K: kArr, D: dArr, J: jArr}
	}

	kdn := 9 // RSV 周期
	kVal := 50.0
	dVal := 50.0

	for i := 0; i < n; i++ {
		start := i - kdn + 1
		if start < 0 {
			start = 0
		}

		highN := math.MaxFloat64 * -1
		lowN := math.MaxFloat64

		for j := start; j <= i; j++ {
			if items[j].High > highN {
				highN = items[j].High
			}
			if items[j].Low < lowN {
				lowN = items[j].Low
			}
		}

		var rsv float64
		if highN == lowN {
			rsv = 50.0
		} else {
			rsv = (items[i].Close - lowN) / (highN - lowN) * 100
		}

		// 标准 KDJ 平滑公式: K=(2×昨K+RSV)/3, D=(2×昨D+K)/3, J=3K-2D
		kVal = (2*kVal + rsv) / 3.0
		dVal = (2*dVal + kVal) / 3.0
		jVal := 3*kVal - 2*dVal

		kArr[i] = kVal
		dArr[i] = dVal
		jArr[i] = jVal
	}

	return KDJData{K: kArr, D: dArr, J: jArr}
}

// round2 保留两位小数
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
