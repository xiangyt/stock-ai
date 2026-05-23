package tencentstock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"stock-ai/internal/adapter"
)

// fundamentalRefer 基本面接口 Referer
const fundamentalRefer = "https://gu.qq.com/"

// ========== 股东户数 ==========

// holderCountResponse 腾讯股东户数接口响应
// 接口：https://proxy.finance.qq.com/ifzqgtimg/appstock/app/finance/holdernum
type holderCountResponse struct {
	Data []holderCountItem `json:"data"`
	Code int               `json:"code"`
	Msg  string            `json:"msg"`
}

type holderCountItem struct {
	EndDate        string  `json:"end_date"`         // 截止日 "2025-09-30"
	HolderNum      int64   `json:"holder_num"`       // 股东户数
	HolderNumChg   float64 `json:"holder_num_chg"`   // 较上期变化(%)
	AvgFreeShares  int64   `json:"avg_free_shares"`  // 人均流通股(股)
	AvgHoldAmount  float64 `json:"avg_hold_amount"`  // 人均持股市值(元)
	Price          float64 `json:"price"`            // 报告期末股价(元)
	FocusLevel     string  `json:"focus_level"`      // 筹码集中度
}

// GetShareholderCounts 获取股东户数历史列表
func (a *Adapter) GetShareholderCounts(ctx context.Context, code string) ([]adapter.ShareholderCount, error) {
	return a.fetchHolderCounts(ctx, code)
}

// GetLatestShareholderCount 获取最新股东户数
func (a *Adapter) GetLatestShareholderCount(ctx context.Context, code string) (*adapter.ShareholderCount, error) {
	counts, err := a.fetchHolderCounts(ctx, code)
	if err != nil {
		return nil, err
	}
	if len(counts) == 0 {
		return nil, fmt.Errorf("暂无股东户数数据: %s", code)
	}
	return &counts[0], nil
}

// fetchHolderCounts 拉取股东户数数据
func (a *Adapter) fetchHolderCounts(ctx context.Context, code string) ([]adapter.ShareholderCount, error) {
	tc := toTencentCode(code)
	urlStr := fmt.Sprintf(
		"https://proxy.finance.qq.com/ifzqgtimg/appstock/app/finance/holdernum?_var=holdernum&code=%s&count=20&r=%d",
		tc, time.Now().UnixMilli(),
	)
	body, err := a.doGet(ctx, urlStr, fundamentalRefer)
	if err != nil {
		return nil, fmt.Errorf("fetchHolderCounts 请求失败: %w", err)
	}

	if idx := strings.Index(body, "="); idx >= 0 {
		body = body[idx+1:]
	}

	var resp holderCountResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &resp); err != nil {
		return nil, fmt.Errorf("股东户数JSON解析失败: %w", err)
	}

	_, symbol := splitTencentCode(tc)
	result := make([]adapter.ShareholderCount, 0, len(resp.Data))
	for _, item := range resp.Data {
		result = append(result, adapter.ShareholderCount{
			Code:                   symbol,
			SecurityCode:           symbol,
			EndDate:                item.EndDate,
			HolderNum:              item.HolderNum,
			HolderNumChangePct:     item.HolderNumChg,
			AvgFreeShares:          item.AvgFreeShares,
			AvgHoldAmount:          item.AvgHoldAmount,
			Price:                  item.Price,
			HoldFocus:              item.FocusLevel,
		})
	}
	return result, nil
}

// ========== 股本变动 ==========

// shareChangeResponse 股本变动接口响应
type shareChangeResponse struct {
	Data []shareChangeItem `json:"data"`
	Code int               `json:"code"`
}

type shareChangeItem struct {
	Date            string `json:"change_date"` // 变动日期
	TotalShares     int64  `json:"total_share"`  // 总股本(股)
	LimitedShares   int64  `json:"limit_share"`  // 受限股(股)
	UnlimitedShares int64  `json:"flow_share"`   // 流通股(股)
	FloatAShares    int64  `json:"a_share"`      // 流通A股
	Reason          string `json:"reason"`       // 变动原因
}

// GetShareChanges 获取历年股本变动
func (a *Adapter) GetShareChanges(ctx context.Context, code string) ([]adapter.ShareChange, error) {
	tc := toTencentCode(code)
	urlStr := fmt.Sprintf(
		"https://proxy.finance.qq.com/ifzqgtimg/appstock/app/finance/sharechange?_var=sharechange&code=%s&count=30&r=%d",
		tc, time.Now().UnixMilli(),
	)
	body, err := a.doGet(ctx, urlStr, fundamentalRefer)
	if err != nil {
		return nil, fmt.Errorf("GetShareChanges 请求失败: %w", err)
	}

	if idx := strings.Index(body, "="); idx >= 0 {
		body = body[idx+1:]
	}

	var resp shareChangeResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &resp); err != nil {
		return nil, fmt.Errorf("股本变动JSON解析失败: %w", err)
	}

	_, symbol := splitTencentCode(tc)
	result := make([]adapter.ShareChange, 0, len(resp.Data))
	for _, item := range resp.Data {
		result = append(result, adapter.ShareChange{
			Code:            symbol,
			Date:            truncateDate(item.Date),
			TotalShares:     item.TotalShares,
			LimitedShares:   item.LimitedShares,
			UnlimitedShares: item.UnlimitedShares,
			FloatAShares:    item.FloatAShares,
			ChangeReason:    item.Reason,
		})
	}
	return result, nil
}

// ========== 机构持仓 ==========

// instHoldingResponse 机构持仓接口响应
type instHoldingResponse struct {
	Data []instHoldingItem `json:"data"`
	Code int               `json:"code"`
}

type instHoldingItem struct {
	ReportDate      string  `json:"report_date"`     // 报告期 "20250930"
	InstCount       int     `json:"inst_count"`      // 机构总数
	TotalFreeShares int64   `json:"total_shares"`    // 持仓合计(股)
	TotalMarketCap  float64 `json:"total_mktcap"`    // 持仓市值(元)
	FreeShareRatio  float64 `json:"free_share_ratio"` // 占流通股比(%)
	TotalShareRatio float64 `json:"total_ratio"`     // 占总股本比(%)
	ClosePrice      float64 `json:"close_price"`     // 期末收盘价
	HoldingChgRatio float64 `json:"chg_ratio"`       // 持仓变动幅度(%)
	FreeShareChgPct float64 `json:"shares_chg_pct"`  // 较上期变化(%)
	FreeShareChgNum int64   `json:"shares_chg_num"`  // 变动数量(股)
}

// GetInstitutionalHoldings 获取机构持仓历史
func (a *Adapter) GetInstitutionalHoldings(ctx context.Context, code string) ([]adapter.InstitutionalHolding, error) {
	tc := toTencentCode(code)
	urlStr := fmt.Sprintf(
		"https://proxy.finance.qq.com/ifzqgtimg/appstock/app/finance/orghold?_var=orghold&code=%s&count=20&r=%d",
		tc, time.Now().UnixMilli(),
	)
	body, err := a.doGet(ctx, urlStr, fundamentalRefer)
	if err != nil {
		return nil, fmt.Errorf("GetInstitutionalHoldings 请求失败: %w", err)
	}

	if idx := strings.Index(body, "="); idx >= 0 {
		body = body[idx+1:]
	}

	var resp instHoldingResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &resp); err != nil {
		return nil, fmt.Errorf("机构持仓JSON解析失败: %w", err)
	}

	_, symbol := splitTencentCode(tc)
	result := make([]adapter.InstitutionalHolding, 0, len(resp.Data))
	for _, item := range resp.Data {
		result = append(result, adapter.InstitutionalHolding{
			Code:               symbol,
			ReportDate:         truncateDate(item.ReportDate),
			InstitutionCount:   item.InstCount,
			TotalFreeShares:    item.TotalFreeShares,
			TotalMarketCap:     item.TotalMarketCap,
			FreeShareRatio:     item.FreeShareRatio,
			TotalShareRatio:    item.TotalShareRatio,
			ClosePrice:         item.ClosePrice,
			HoldingChangeRatio: item.HoldingChgRatio,
			FreeShareChangePct: item.FreeShareChgPct,
			FreeShareChangeNum: item.FreeShareChgNum,
		})
	}
	return result, nil
}
