package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"stock-ai/internal/adapter"
)

// ========== 财务数据（财报） ==========

// financeResponse 东财财报接口响应结构
type financeResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message"`
	Result  *financeResult      `json:"result"`
}

type financeResult struct {
	Pages int               `json:"pages"`
	Count int               `json:"count"`
	Data []financeItem      `json:"data"`
}

// financeItem 东财原始财报字段（JSON tag 对应接口返回的 key，金额单位：元）
type financeItem struct {
	SECUCODE             string   `json:"SECUCODE"`
	SECURITY_CODE        string   `json:"SECURITY_CODE"`
	SECURITY_NAME_ABBR   string   `json:"SECURITY_NAME_ABBR"`
	REPORT_DATE          string   `json:"REPORT_DATE"`
	REPORT_TYPE          string   `json:"REPORT_TYPE"`
	REPORT_DATE_NAME     string   `json:"REPORT_DATE_NAME"`
	CURRENCY             string   `json:"CURRENCY"`
	NOTICE_DATE          string   `json:"NOTICE_DATE"`
	UPDATE_DATE          string   `json:"UPDATE_DATE"`
	EPSJB                *float64 `json:"EPSJB"`           // 基本每股收益(元)
	EPSKCJB              *float64 `json:"EPSKCJB"`         // 扣非每股收益(元)
	EPSXS                *float64 `json:"EPSXS"`            // 摊薄每股收益(元)
	BPS                  *float64 `json:"BPS"`              // 每股净资产(元)
	MGZBGJ               *float64 `json:"MGZBGJ"`           // 每股公积金(元)
	MGWFPLR              *float64 `json:"MGWFPLR"`          // 每股未分配利润(元)
	MGJYXJJE             *float64 `json:"MGJYXJJE"`         // 每股经营现金流(元)
	TOTALOPERATEREVE     *float64 `json:"TOTALOPERATEREVE"` // 营业总收入(元)
	MLR                  *float64 `json:"MLR"`              // 毛利润(元)
	PARENTNETPROFIT      *float64 `json:"PARENTNETPROFIT"`  // 归属净利润(元)
	KCFJCXSYJLR          *float64 `json:"KCFJCXSYJLR"`      // 扣非净利润(元)
	TOTALOPERATEREVETZ   *float64 `json:"TOTALOPERATEREVETZ"`  // 营收同比(%)
	PARENTNETPROFITTZ    *float64 `json:"PARENTNETPROFITTZ"`   // 归母净利同比(%)
	KCFJCXSYJLRTZ        *float64 `json:"KCFJCXSYJLRTZ"`       // 扣非净利同比(%)
	YYZSRGDHBZC          *float64 `json:"YYZSRGDHBZC"`       // 营收滚动环比(%)
	NETPROFITRPHBZC      *float64 `json:"NETPROFITRPHBZC"`   // 归母净利滚动环比(%)
	KFJLRGDHBZC          *float64 `json:"KFJLRGDHBZC"`       // 扣非滚动环比(%)
	ROEJQ                *float64 `json:"ROEJQ"`             // 净资产收益率(加权)(%)
	ROEKCJQ              *float64 `json:"ROEKCJQ"`           // 净资产收益率(扣非加权)(%)
	ZZCJLL               *float64 `json:"ZZCJLL"`            // 总资产收益率(%)
	XSJLL                *float64 `json:"XSJLL"`             // 销售净利率(%)
	XSMLL                *float64 `json:"XSMLL"`             // 销售毛利率(%)
	TAXRATE              *float64 `json:"TAXRATE"`           // 实际税率(%)
	LD                   *float64 `json:"LD"`                // 流动比率(倍)
	SD                   *float64 `json:"SD"`                // 速动比率(倍)
	XJLLB                *float64 `json:"XJLLB"`             // 现金流比率(倍)
	ZCFZL                *float64 `json:"ZCFZL"`             // 资产负债率(%)
	QYCS                 *float64 `json:"QYCS"`              // 权益乘数(倍)
	CQBL                 *float64 `json:"CQBL"`              // 产权比率(倍)
	LIABILITY            *float64 `json:"LIABILITY"`         // 总负债(元)
	ROIC                 *float64 `json:"ROIC"`              // 投资资本回报率(%)
	YSZKYYSR             *float64 `json:"YSZKYYSR"`          // 应收账款/营业收入
	XSJXLYYSR            *float64 `json:"XSJXLYYSR"`         // 销售净现金流/营业收入
	JYXJLYYSR            *float64 `json:"JYXJLYYSR"`         // 经营净现金流/营业收入
	ZZCZZTS              *float64 `json:"ZZCZZTS"`           // 总资产周转天数(天)
	CHZZTS               *float64 `json:"CHZZTS"`            // 存货周转天数(天)
	YSZKZZTS             *float64 `json:"YSZKZZTS"`          // 应收账款周转天数(天)
	TOAZZL               *float64 `json:"TOAZZL"`            // 总资产周转率(次)
	CHZZL                *float64 `json:"CHZZL"`             // 存货周转率(次)
	YSZKZZL              *float64 `json:"YSZKZZL"`           // 应收账款周转率(次)
	DJD_TOI_YOY          *float64 `json:"DJD_TOI_YOY"`       // 营收环比增长(%)
	DJD_DPNP_YOY         *float64 `json:"DJD_DPNP_YOY"`     // 归属净利环比增长(%)
	DJD_DEDUCTDPNP_YOY   *float64 `json:"DJD_DEDUCTDPNP_YOY"` // 扣非净利环比增长(%)
	DJD_TOI_QOQ          *float64 `json:"DJD_TOI_QOQ"`       // 营收滚动环比增长(%)
	DJD_DPNP_QOQ         *float64 `json:"DJD_DPNP_QOQ"`      // 归属净利滚动环比增长(%)
	DJD_DEDUCTDPNP_QOQ   *float64 `json:"DJD_DEDUCTDPNP_QOQ"` // 扣非净利滚动环比增长(%)
	STAFF_NUM            *int     `json:"STAFF_NUM"`         // 员工人数(人)
	AVG_TOI              *float64 `json:"AVG_TOI"`           // 人均创收(元)
	AVG_NET_PROFIT       *float64 `json:"AVG_NET_PROFIT"`    // 人均创利(元)
	PREPAID_ACCOUNTS_RATIO *float64 `json:"PREPAID_ACCOUNTS_RATIO"` // 预付款项/营业成本(%)
	ACCOUNTS_PAYABLE_TR  *float64 `json:"ACCOUNTS_PAYABLE_TR"`  // 应付账款周转率(次)
	FIXED_ASSET_TR       *float64 `json:"FIXED_ASSET_TR"`    // 固定资产周转率(次)
	CURRENT_ASSET_TR     *float64 `json:"CURRENT_ASSET_TR"`  // 流动资产周转率(次)
	PREPAID_ACCOUNTS_TDAYS *float64 `json:"PREPAID_ACCOUNTS_TDAYS"` // 预付款项周转天数(天)
	PAYABLE_TDAYS        *float64 `json:"PAYABLE_TDAYS"`     // 应付账款周转天数(天)
	OPERATE_CYCLE        *float64 `json:"OPERATE_CYCLE"`     // 营运周期(天)
	GUARD_SPEED_RATIO    *float64 `json:"GUARD_SPEED_RATIO"` // 速动比率(修正)(倍)
	CASH_RATIO           *float64 `json:"CASH_RATIO"`        // 现金比率(倍)
	INTEREST_COVERAGE_RATIO *float64 `json:"INTEREST_COVERAGE_RATIO"` // 利息保障倍数(倍)
	CA_TA                *float64 `json:"CA_TA"`             // 流动资产/总资产(%)
	NCA_TA               *float64 `json:"NCA_TA"`            // 非流动资产/总资产(%)
	LIQUIDATION_RATIO    *float64 `json:"LIQUIDATION_RATIO"` // 清算价值比率(倍)
	INTEREST_DEBT_RATIO  *float64 `json:"INTEREST_DEBT_RATIO"` // 有息负债率(%)
	FC_LIABILITIES       *float64 `json:"FC_LIABILITIES"`    // 金融负债占比(%)
	FCFF_FORWARD         *float64 `json:"FCFF_FORWARD"`     // 企业自由现金流(预测,元)
	FCFF_BACK            *float64 `json:"FCFF_BACK"`        // 企业自由现金流(回溯,元)
	IS_BZ                *string `json:"IS_BZ"`             // 是否本期
}

// GetPerformanceReports 获取业绩报表（东财 RPT_F10_FINANCE_MAINFINADATA）
func (a *Adapter) GetPerformanceReports(ctx context.Context, code string) ([]adapter.PerformanceReport, error) {
	secucode := buildSecucode(code)
	urlStr := fmt.Sprintf(
		"https://datacenter.eastmoney.com/securities/api/data/get?type=RPT_F10_FINANCE_MAINFINADATA"+
			"&sty=APP_F10_MAINFINADATA&quoteColumns=&filter=(SECUCODE%%3D%%22%s%%22)"+
			"&p=1&ps=200&sr=-1&st=REPORT_DATE&source=HSF10&client=PC",
		secucode)

	body, err := a.makeGetRequest(urlStr, "https://emweb.securities.eastmoney.com/")
	if err != nil {
		return nil, fmt.Errorf("fetch finance reports: %w", err)
	}

	var resp financeResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal finance response: %w", err)
	}

	if !resp.Success || resp.Result == nil {
		return nil, fmt.Errorf("finance reports API failed: %s", resp.Message)
	}

	reports := make([]adapter.PerformanceReport, len(resp.Result.Data))
	for i, item := range resp.Result.Data {
		reports[i] = convertToPerformanceReport(item)
	}

	return reports, nil
}

// convertToPerformanceReport 将东财原始字段转换为通用 PerformanceReport
func convertToPerformanceReport(item financeItem) adapter.PerformanceReport {
	return adapter.PerformanceReport{
		Code:                    item.SECURITY_CODE,
		SecurityName:            item.SECURITY_NAME_ABBR,
		ReportDate:              truncateDate(item.REPORT_DATE),
		ReportType:              item.REPORT_TYPE,
		ReportDateName:          item.REPORT_DATE_NAME,
		Currency:                item.CURRENCY,
		NoticeDate:              truncateDate(item.NOTICE_DATE),
		UpdateDate:              truncateDate(item.UPDATE_DATE),
		IsBZ:                     ptrStrOrEmpty(item.IS_BZ),

		BasicEPS:               floatPtrOrZero(item.EPSJB),
		DeductedEPS:            floatPtrOrZero(item.EPSKCJB),
		DilutedEPS:             floatPtrOrZero(item.EPSXS),
		BVPS:                   floatPtrOrZero(item.BPS),
		EquityReservePerShare:  floatPtrOrZero(item.MGZBGJ),
		UndistributedProfitPS:  floatPtrOrZero(item.MGWFPLR),
		OCFPS:                  floatPtrOrZero(item.MGJYXJJE),

		TotalRevenue:        floatPtrOrZero(item.TOTALOPERATEREVE),
		GrossProfit:         floatPtrOrZero(item.MLR),
		ParentNetProfit:     floatPtrOrZero(item.PARENTNETPROFIT),
		DeductNetProfit:     floatPtrOrZero(item.KCFJCXSYJLR),
		RevenueYoY:          floatPtrOrZero(item.TOTALOPERATEREVETZ),
		ParentNetProfitYoY:  floatPtrOrZero(item.PARENTNETPROFITTZ),
		DeductNetProfitYoY:  floatPtrOrZero(item.KCFJCXSYJLRTZ),
		RevenueRollQoQ:      floatPtrOrZero(item.YYZSRGDHBZC),
		NetProfitRollQoQ:    floatPtrOrZero(item.NETPROFITRPHBZC),
		DeductNPTRollQoQ:    floatPtrOrZero(item.KFJLRGDHBZC),
		RevenueQoQ:          floatPtrOrZero(item.DJD_TOI_YOY),
		NetProfitQoQ:        floatPtrOrZero(item.DJD_DPNP_YOY),
		DeductNPTQoQ:        floatPtrOrZero(item.DJD_DEDUCTDPNP_YOY),

		ROEW:           floatPtrOrZero(item.ROEJQ),
		ROEDW:          floatPtrOrZero(item.ROEKCJQ),
		ROA:            floatPtrOrZero(item.ZZCJLL),
		NetMargin:      floatPtrOrZero(item.XSJLL),
		GrossMargin:    floatPtrOrZero(item.XSMLL),
		NetProfitMargin: floatPtrOrZero(item.XSMLL), // 东财无独立净利率，暂用毛利率
		ROIC:           floatPtrOrZero(item.ROIC),
		TaxRate:        floatPtrOrZero(item.TAXRATE),

		ARToRevenue:      floatPtrOrZero(item.YSZKYYSR),
		SaleOCFToRevenue: floatPtrOrZero(item.XSJXLYYSR),
		OCFToRevenue:     floatPtrOrZero(item.JYXJLYYSR),

		CurrentRatio:      floatPtrOrZero(item.LD),
		QuickRatio:        floatPtrOrZero(item.SD),
		CashFlowRatio:     floatPtrOrZero(item.XJLLB),
		DebtRatio:         floatPtrOrZero(item.ZCFZL),
		EquityMultiplier:  floatPtrOrZero(item.QYCS),
		DebtEquityRatio:   floatPtrOrZero(item.CQBL),
		Liability:         floatPtrOrZero(item.LIABILITY),

		TotalAssetTurnoverDays: floatPtrOrZero(item.ZZCZZTS),
		InvTurnoverDays:        floatPtrOrZero(item.CHZZTS),
		ARTurnoverDays:         floatPtrOrZero(item.YSZKZZTS),
		PayableTurnoverDays:    floatPtrOrZero(item.PAYABLE_TDAYS),
		PrepaidTurnoverDays:    floatPtrOrZero(item.PREPAID_ACCOUNTS_TDAYS),
		FixedAssetTurnover:     floatPtrOrZero(item.FIXED_ASSET_TR),
		CurrentAssetTurnover:   floatPtrOrZero(item.CURRENT_ASSET_TR),
		OperateCycle:           floatPtrOrZero(item.OPERATE_CYCLE),
		GuardSpeedRatio:        floatPtrOrZero(item.GUARD_SPEED_RATIO),
		CashRatio:              floatPtrOrZero(item.CASH_RATIO),
		InterestCoverageRatio:  floatPtrOrZero(item.INTEREST_COVERAGE_RATIO),
		LiquidationRatio:       floatPtrOrZero(item.LIQUIDATION_RATIO),
		InterestDebtRatio:      floatPtrOrZero(item.INTEREST_DEBT_RATIO),
		FCLiabilities:          floatPtrOrZero(item.FC_LIABILITIES),

		CurrentAssetRatio:    floatPtrOrZero(item.CA_TA),
		NonCurrentAssetRatio: floatPtrOrZero(item.NCA_TA),

		FCFFForward: floatPtrOrZero(item.FCFF_FORWARD),
		FCFFBack:    floatPtrOrZero(item.FCFF_BACK),

		StaffNum:             intPtrOrZero(item.STAFF_NUM),
		AvgTOI:               floatPtrOrZero(item.AVG_TOI),
		AvgNetProfit:         floatPtrOrZero(item.AVG_NET_PROFIT),
		PrepaidAccountsRatio: floatPtrOrZero(item.PREPAID_ACCOUNTS_RATIO),
		AccountsPayableTR:    floatPtrOrZero(item.ACCOUNTS_PAYABLE_TR),
		FixedAssetTR:         floatPtrOrZero(item.FIXED_ASSET_TR),
	}
}

// GetLatestPerformanceReport 获取最新业绩报表
func (a *Adapter) GetLatestPerformanceReport(ctx context.Context, code string) (*adapter.PerformanceReport, error) {
	reports, err := a.GetPerformanceReports(ctx, code)
	if err != nil {
		return nil, err
	}
	if len(reports) == 0 {
		return nil, fmt.Errorf("no reports for %s", code)
	}
	return &reports[0], nil
}

// ========== 辅助函数 ==========

func floatPtrOrZero(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func intPtrOrZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func ptrStrOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// parseReportYear 从 REPORT_YEAR 或 REPORT_DATE 中提取年份
// 用于按年度筛选财报
func parseReportYear(reportDate string) int {
	// "2025-12-31" or "2025" → 2025
	s := strings.TrimSpace(reportDate)
	if len(s) >= 4 {
		if year, err := strconv.Atoi(s[:4]); err == nil {
			return year
		}
	}
	return 0
}
