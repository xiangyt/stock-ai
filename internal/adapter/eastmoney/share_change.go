package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"time"

	"stock-ai/internal/adapter"
)

// ========== 股本变动 ==========

// equityItem 东财F10股本结构单条记录 (RPT_F10_EH_EQUITY)
type equityItem struct {
	Secucode        string `json:"SECUCODE"`
	SecurityCode    string `json:"SECURITY_CODE"`
	EndDate         string `json:"END_DATE"`
	TotalShares     int64  `json:"TOTAL_SHARES"`
	LimitedShares   int64  `json:"LIMITED_SHARES"`
	UnlimitedShares int64  `json:"UNLIMITED_SHARES"`
	ListedAShares   int64  `json:"LISTED_A_SHARES"`
	ChangeReason    string `json:"CHANGE_REASON"`
}

// equityResponse 东财F10股本结构API响应
type equityResponse struct {
	Result struct {
		Pages int          `json:"pages"`
		Count int          `json:"count"`
		Data  []equityItem `json:"data"`
	} `json:"result"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetShareChanges 获取历年股本变动数据
func (a *Adapter) GetShareChanges(ctx context.Context, code string) ([]adapter.ShareChange, error) {
	symbol, market := parseCode(code)
	secucode := symbol + "." + market
	if market == "" {
		secucode = buildSecucode(symbol)
	}

	columns := "SECUCODE,SECURITY_CODE,END_DATE,TOTAL_SHARES,LIMITED_SHARES," +
		"UNLIMITED_SHARES,LISTED_A_SHARES,FREE_SHARES,LIMITED_A_SHARES," +
		"LOCK_SHARES,CHANGE_REASON"

	var allChanges []adapter.ShareChange
	page := 1
	pageSize := 50
	totalPages := 0

	for {
		params := url.Values{
			"reportName":   {"RPT_F10_EH_EQUITY"},
			"columns":      {columns},
			"quoteColumns": {""},
			"filter":       {fmt.Sprintf(`(SECUCODE="%s")`, secucode)},
			"pageNumber":   {strconv.Itoa(page)},
			"pageSize":     {strconv.Itoa(pageSize)},
			"sortTypes":    {"-1"},
			"sortColumns":  {"END_DATE"},
			"source":       {"HSF10"},
			"client":       {"PC"},
		}

		urlStr := "https://datacenter.eastmoney.com/securities/api/data/v1/get?" + params.Encode()
		body, err := a.makeGetRequest(urlStr, "https://emweb.securities.eastmoney.com/")
		if err != nil {
			return nil, fmt.Errorf("请求股本变动第%d页失败: %w", page, err)
		}

		var resp equityResponse
		if err := json.Unmarshal([]byte(body), &resp); err != nil {
			return nil, fmt.Errorf("解析股本变动JSON失败: %w", err)
		}
		if !resp.Success {
			return nil, fmt.Errorf("股本变动API错误: %s", resp.Message)
		}

		if totalPages == 0 {
			totalPages = resp.Result.Pages
		}

		for _, item := range resp.Result.Data {
			dateStr := item.EndDate
			if len(dateStr) >= 10 {
				dateStr = dateStr[:10]
			}

			allChanges = append(allChanges, adapter.ShareChange{
				Code:            code,
				Date:            dateStr,
				TotalShares:     item.TotalShares,
				LimitedShares:   item.LimitedShares,
				UnlimitedShares: item.UnlimitedShares,
				FloatAShares:    item.ListedAShares,
				ChangeReason:    item.ChangeReason,
			})
		}

		if page >= totalPages || len(resp.Result.Data) < pageSize {
			break
		}
		page++
		time.Sleep(80 * time.Millisecond)
	}

	log.Printf("[eastmoney] %s 股本变动: %d 条记录 (%d页)", code, len(allChanges), totalPages)
	return allChanges, nil
}
