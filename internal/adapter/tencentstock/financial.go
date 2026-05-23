package tencentstock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"stock-ai/internal/adapter"
)

// financialRefer 财务数据接口 Referer
const financialRefer = "https://gu.qq.com/"

// ========== 业绩报表 ==========

// perfReportResponse 腾讯财务报表接口响应
// 接口：https://proxy.finance.qq.com/ifzqgtimg/appstock/app/finance/mainFinancial
//
//	?_var=mainFinancial&code=sh600519&type=Q4&count=8&r=xxx
//
// type: Y=年报, Q4=四季报, Q3=三季报, Q2=中报, Q1=一季报
type perfReportResponse struct {
	Data []perfReportItem `json:"data"`
	Code int              `json:"code"`
	Msg  string           `json:"msg"`
}

type perfReportItem struct {
	// 基本信息
	ReportDate string `json:"report_date"` // 报告期 "20251231"
	ReportType string `json:"report_type"` // 报告类型 "年报"/"三季报"等

	// 每股指标（元）
	EPS  float64 `json:"jbmgsy"` // 基本每股收益
	BVPS float64 `json:"mgsy"`   // 每股净资产

	// 规模指标（元）
	TotalRevenue    float64 `json:"yysr"`     // 营业收入
	GrossProfit     float64 `json:"mlr"`      // 毛利润
	ParentNetProfit float64 `json:"归属净利润"` // 归属净利润（字段名为中文）

	// 盈利能力（%）
	ROE         float64 `json:"jzcsyl"` // 净资产收益率
	ROA         float64 `json:"zzcsyl"` // 总资产收益率
	GrossMargin float64 `json:"mll"`    // 毛利率
	NetMargin   float64 `json:"jll"`    // 净利率

	// 增长（%）
	RevenueYoY     float64 `json:"yysrtbzz"` // 营收同比
	NetProfitYoY   float64 `json:"归属净利润同比"` // 净利润同比

	// 风险
	DebtRatio float64 `json:"zcfzl"` // 资产负债率
}

// mainFinancialResponse 腾讯主要财务指标接口响应
// 接口格式更通用，字段以数组形式返回
type mainFinancialResponse struct {
	Data map[string]interface{} `json:"data"`
	Code int                    `json:"code"`
}

// GetPerformanceReports 获取历史业绩报表列表
func (a *Adapter) GetPerformanceReports(ctx context.Context, code string) ([]adapter.PerformanceReport, error) {
	return a.fetchPerfReports(ctx, code, 20)
}

// GetLatestPerformanceReport 获取最新一期业绩报表
func (a *Adapter) GetLatestPerformanceReport(ctx context.Context, code string) (*adapter.PerformanceReport, error) {
	reports, err := a.fetchPerfReports(ctx, code, 1)
	if err != nil {
		return nil, err
	}
	if len(reports) == 0 {
		return nil, fmt.Errorf("暂无业绩数据: %s", code)
	}
	return &reports[0], nil
}

// fetchPerfReports 拉取业绩报表（腾讯财报接口）
func (a *Adapter) fetchPerfReports(ctx context.Context, code string, count int) ([]adapter.PerformanceReport, error) {
	tc := toTencentCode(code)

	// 拉取主要财务指标（qqstock/data/hs/detail/performance?code=sh600519）
	urlStr := fmt.Sprintf(
		"https://proxy.finance.qq.com/ifzqgtimg/appstock/app/finance/mainFinancial?_var=mainFinancial&code=%s&type=Q4&count=%d&r=%d",
		tc, count, time.Now().UnixMilli(),
	)
	body, err := a.doGet(ctx, urlStr, financialRefer)
	if err != nil {
		return nil, fmt.Errorf("fetchPerfReports 请求失败: %w", err)
	}

	if idx := strings.Index(body, "="); idx >= 0 {
		body = body[idx+1:]
	}

	var resp mainFinancialResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &resp); err != nil {
		return nil, fmt.Errorf("业绩报表JSON解析失败: %w", err)
	}

	return parseMainFinancialData(resp.Data, code), nil
}

// parseMainFinancialData 解析腾讯主要财务指标数据
// 腾讯财报接口返回格式较复杂，以报告期数组形式排列各指标
func parseMainFinancialData(data map[string]interface{}, origCode string) []adapter.PerformanceReport {
	if data == nil {
		return nil
	}

	// 尝试提取报告期列表（"date" 字段）
	dates := extractStringSlice(data, "date")
	if len(dates) == 0 {
		return nil
	}

	reports := make([]adapter.PerformanceReport, 0, len(dates))
	for i, date := range dates {
		rpt := adapter.PerformanceReport{
			Code:       origCode,
			ReportDate: truncateDate(date),
		}

		// 工具函数：从 data[key] 数组中取第 i 个值
		fv := func(key string) float64 {
			arr := extractFloatSlice(data, key)
			if i < len(arr) {
				return arr[i]
			}
			return 0
		}

		rpt.BasicEPS = fv("jbmgsy")         // 基本每股收益(元)
		rpt.BVPS = fv("mgsy")               // 每股净资产(元)
		rpt.TotalRevenue = fv("yysr")        // 营业收入(元)
		rpt.GrossProfit = fv("mlr")          // 毛利润(元)
		rpt.ParentNetProfit = fv("归属净利润")
		rpt.ROEW = fv("jzcsyl")             // 净资产收益率-加权
		rpt.ROA = fv("zzcsyl")              // 总资产收益率
		rpt.GrossMargin = fv("mll")         // 毛利率
		rpt.NetMargin = fv("jll")           // 净利率
		rpt.RevenueYoY = fv("yysrtbzz")     // 营收同比
		rpt.ParentNetProfitYoY = fv("归属净利润同比")
		rpt.DebtRatio = fv("zcfzl")         // 资产负债率
		rpt.CurrentRatio = fv("lldb")       // 流动比率
		rpt.QuickRatio = fv("sddb")         // 速动比率

		reports = append(reports, rpt)
	}
	return reports
}

// ========== 辅助：从 map[string]interface{} 提取数组 ==========

func extractStringSlice(data map[string]interface{}, key string) []string {
	raw, ok := data[key]
	if !ok {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		result = append(result, fmt.Sprintf("%v", v))
	}
	return result
}

func extractFloatSlice(data map[string]interface{}, key string) []float64 {
	raw, ok := data[key]
	if !ok {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	result := make([]float64, 0, len(arr))
	for _, v := range arr {
		result = append(result, parseFloat(fmt.Sprintf("%v", v)))
	}
	return result
}
