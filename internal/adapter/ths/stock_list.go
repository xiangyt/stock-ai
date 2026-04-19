package ths

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"stock-ai/internal/adapter"
)

// ========== 股票列表 ==========

func (a *Adapter) GetStockList(_ context.Context) ([]adapter.StockBasic, error) {
	var allStocks []adapter.StockBasic
	maxPages := 103

	for page := 1; page <= maxPages; page++ {
		stocks, hasMore, err := a.fetchStockListPage(page)
		if err != nil {
			if page == 1 {
				return nil, fmt.Errorf("获取股票列表失败: %w", err)
			}
			break
		}
		allStocks = append(allStocks, stocks...)
		if !hasMore || len(stocks) == 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	return allStocks, nil
}

func (a *Adapter) fetchStockListPage(page int) ([]adapter.StockBasic, bool, error) {
	url := fmt.Sprintf("https://data.10jqka.com.cn/funds/ggzjl/field/zdf/order/desc/page/%d/ajax/1/free/1/", page)

	req, _ := http.NewRequest("GET", url, nil)
	a.setTHSCommonHeaders(req, "https://data.10jqka.com.cn/funds/ggzjl/")
	req.Header.Set("Accept", "text/html, */*; q=0.01")
	req.Header.Set("Priority", "u=1, i")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	hexinV := generateWencaiToken()
	a.setTHSCookie(req, hexinV)
	req.Header.Set("Hexin-V", hexinV)

	resp, err := a.doRequest(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	body, err := a.readBody(resp)
	if err != nil {
		return nil, false, err
	}
	return a.parseStockListHTML(string(body))
}

func (a *Adapter) parseStockListHTML(html string) ([]adapter.StockBasic, bool, error) {
	lines := strings.Split(html, "\n")
	var stocks []adapter.StockBasic
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "stockCode") || !strings.Contains(line, "linkToGghq") {
			continue
		}
		code := extractBetween(line, "stockCode\">", "</a>")
		if code == "" {
			continue
		}
		name := a.extractNameFromLines(lines, i)
		_, exchange, board := detectExchange(code)
		stocks = append(stocks, adapter.StockBasic{
			Code:         code,
			Name:         name,
			Exchange:     exchange,
			ListingBoard: board,
		})
	}
	return stocks, len(stocks) > 0, nil
}

// GetStockDetail 获取股票详情
func (a *Adapter) GetStockDetail(_ context.Context, code string) (*adapter.StockBasic, error) {
	symbol, _, err := a.parseCode(code)
	if err != nil {
		return nil, fmt.Errorf("invalid tsCode format: %s", code)
	}
	_, exchange, board := detectExchange(symbol)
	return &adapter.StockBasic{
		Code:         symbol,
		Name:         "",
		Exchange:     exchange,
		ListingBoard: board,
	}, nil
}
