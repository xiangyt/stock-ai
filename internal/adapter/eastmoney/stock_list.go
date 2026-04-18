package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"stock-ai/internal/adapter"
)

// ========== 股票列表 ==========

// StockListResponse 股票列表API响应
type StockListResponse struct {
	RC   int `json:"rc"`
	Data struct {
		Total int                             `json:"total"`
		Diff  []StockListResponseDataDiffItem `json:"diff"`
	} `json:"data"`
}

// StockListResponseDataDiffItem 股票列表单条数据
type StockListResponseDataDiffItem struct {
	F12 string `json:"f12"` // 代码
	F13 int    `json:"f13"` // 市场 0=深市 1=沪市
	F14 string `json:"f14"` // 名称
}

func (a *Adapter) GetStockList(_ context.Context) ([]adapter.StockBasic, error) {
	var allStocks []adapter.StockBasic
	page := 1
	pageSize := 50

	for {
		respData, err := a.fetchStockListPage(page, pageSize)
		if err != nil {
			return nil, fmt.Errorf("获取第%d页失败: %w", page, err)
		}

		if len(respData.Data.Diff) == 0 {
			break
		}

		for _, item := range respData.Data.Diff {
			stock := a.convertToStockBasic(item)
			allStocks = append(allStocks, stock)
		}

		if len(respData.Data.Diff) < pageSize {
			break
		}
		page++
		time.Sleep(100 * time.Millisecond)
	}

	return allStocks, nil
}

func (a *Adapter) fetchStockListPage(page, pageSize int) (*StockListResponse, error) {
	baseURL := "https://push2.eastmoney.com/api/qt/clist/get"
	params := url.Values{
		"cb":     {fmt.Sprintf("jQuery%d", time.Now().UnixMilli())},
		"fid":    {"f12"},
		"po":     {"0"},
		"pz":     {strconv.Itoa(pageSize)},
		"pn":     {strconv.Itoa(page)},
		"np":     {"1"},
		"fltt":   {"2"},
		"invt":   {"2"},
		"ut":     {"b2884a393a59ad64002292a3e90d46a5"},
		"fs":     {"m:0+t:6+f:!2,m:0+t:13+f:!2,m:0+t:80+f:!2,m:1+t:2+f:!2,m:1+t:23+f:!2,m:0+t:7+f:!2,m:1+t:3+f:!2"},
		"fields": {"f12,f14,f15,f16,f17,f18,f19,f20,f23,f116,f117,f162,f13,f128,f164,f167,f168,f169,f170,f171,f177,f183,f184,f185,f186,f187,f188,f189,f190,f191,f192,f193,f204,f205,f206,f207,f208,f209,f210,f211,f212,f213,f214,f215"},
	}

	requestURL := baseURL + "?" + params.Encode()
	body, err := a.makeGetRequest(requestURL, "https://data.eastmoney.com/zjlx/detail.html")
	if err != nil {
		return nil, err
	}

	jsonStr := extractJSONP(body)
	var result StockListResponse
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}
	return &result, nil
}

func (a *Adapter) convertToStockBasic(item StockListResponseDataDiffItem) adapter.StockBasic {
	exchange, listingBoard := detectExchangeAndBoard(item.F12)

	return adapter.StockBasic{
		Code:         item.F12,
		Name:         item.F14,
		Exchange:     exchange,
		ListingBoard: listingBoard,
	}
}
