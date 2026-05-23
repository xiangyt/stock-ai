package tencentstock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/model"
)

// stockListRefer 股票列表接口 Referer
const stockListRefer = "https://gu.qq.com/"

// ========== 股票列表 ==========

// stockListResponse 腾讯自选股-股票列表 JSON 响应
// 接口：https://proxy.finance.qq.com/ifzqgtimg/appstock/app/mktg/hqlist
//
//	?_var=hqlist&type=szzs&page=1&count=500&code=&r=xxx
//
// 常用 type 值:
//
//	sha = 上交所A股   sza = 深交所A股   cyb = 创业板  kcb = 科创板  bjsA = 北交所A股
type stockListResponse struct {
	Data struct {
		Page  int              `json:"page"`
		Count int              `json:"count"`
		Total int              `json:"total"`
		List  []stockListItem  `json:"list"`
	} `json:"data"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type stockListItem struct {
	Code   string `json:"code"`   // 腾讯格式 "sh600519"
	Name   string `json:"name"`   // "贵州茅台"
	Market string `json:"market"` // "sh" / "sz"
}

// GetStockList 获取A股股票列表（沪深北三市A股）
func (a *Adapter) GetStockList(ctx context.Context) ([]adapter.StockBasic, error) {
	// 分批拉取沪A、深A、创业板、科创板、北交所
	markets := []string{"sha", "sza", "cyb", "kcb"}
	var all []adapter.StockBasic

	for _, mkt := range markets {
		items, err := a.fetchStockListByMarket(ctx, mkt)
		if err != nil {
			// 某个市场失败不中断，记录并继续
			fmt.Printf("⚠️ 腾讯自选股拉取 %s 股票列表失败: %v\n", mkt, err)
			continue
		}
		all = append(all, items...)
	}

	// 去重（按代码）
	seen := make(map[string]struct{})
	result := make([]adapter.StockBasic, 0, len(all))
	for _, s := range all {
		if _, ok := seen[s.Code]; !ok {
			seen[s.Code] = struct{}{}
			result = append(result, s)
		}
	}
	return result, nil
}

// fetchStockListByMarket 按市场类型分页拉取股票列表
func (a *Adapter) fetchStockListByMarket(ctx context.Context, mkt string) ([]adapter.StockBasic, error) {
	const pageSize = 500
	var all []adapter.StockBasic
	page := 1

	for {
		urlStr := fmt.Sprintf(
			"https://proxy.finance.qq.com/ifzqgtimg/appstock/app/mktg/hqlist?_var=hqlist&type=%s&page=%d&count=%d&code=&r=%d",
			mkt, page, pageSize, time.Now().UnixMilli(),
		)
		body, err := a.doGet(ctx, urlStr, stockListRefer)
		if err != nil {
			return nil, err
		}

		// 去掉 JS 变量前缀
		if idx := strings.Index(body, "="); idx >= 0 {
			body = body[idx+1:]
		}

		var resp stockListResponse
		if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &resp); err != nil {
			return nil, fmt.Errorf("股票列表JSON解析失败(mkt=%s page=%d): %w", mkt, page, err)
		}

		for _, item := range resp.Data.List {
			prefix, symbol := splitTencentCode(item.Code)
			exchange := tencentToExchange(prefix)
			board := inferBoard(symbol, exchange)
			all = append(all, adapter.StockBasic{
				Code:         symbol,
				Name:         item.Name,
				Exchange:     exchange,
				ListingBoard: board,
			})
		}

		if len(resp.Data.List) < pageSize || len(all) >= resp.Data.Total {
			break
		}
		page++
	}
	return all, nil
}

// GetStockDetail 获取股票基本信息（含公司简介等）
// 接口：https://proxy.finance.qq.com/ifzqgtimg/appstock/app/stockinfo/getInfo
func (a *Adapter) GetStockDetail(ctx context.Context, code string) (*adapter.StockBasic, error) {
	tc := toTencentCode(code)
	urlStr := fmt.Sprintf(
		"https://proxy.finance.qq.com/ifzqgtimg/appstock/app/stockinfo/getInfo?_var=stockinfo&code=%s&r=%d",
		tc, time.Now().UnixMilli(),
	)
	body, err := a.doGet(ctx, urlStr, stockListRefer)
	if err != nil {
		return nil, fmt.Errorf("GetStockDetail 请求失败: %w", err)
	}

	if idx := strings.Index(body, "="); idx >= 0 {
		body = body[idx+1:]
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &raw); err != nil {
		return nil, fmt.Errorf("GetStockDetail JSON解析失败: %w", err)
	}

	data, _ := raw["data"].(map[string]interface{})
	if data == nil {
		// 回退到基本快照（仅返回基础字段）
		prefix, symbol := splitTencentCode(tc)
		return &adapter.StockBasic{
			Code:     symbol,
			Exchange: tencentToExchange(prefix),
		}, nil
	}

	prefix, symbol := splitTencentCode(tc)
	exchange := tencentToExchange(prefix)

	sb := &adapter.StockBasic{
		Code:         symbol,
		Exchange:     exchange,
		ListingBoard: inferBoard(symbol, exchange),
	}

	// 安全提取字段
	safeStr := func(key string) string {
		if v, ok := data[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}

	sb.Name = safeStr("cnname")
	sb.FullName = safeStr("fullname")
	sb.Industry = safeStr("industry")
	sb.Province = safeStr("areaname")
	sb.OrgProfile = safeStr("profile")
	sb.MainBusiness = safeStr("mainbusiness")
	sb.ListDate = truncateDate(safeStr("listdate"))
	sb.OrgWeb = safeStr("website")
	sb.OrgEmail = safeStr("email")
	sb.OrgTel = safeStr("tel")
	sb.Address = safeStr("office")
	sb.President = safeStr("chairman")
	sb.Secretary = safeStr("secretary")

	return sb, nil
}

// ========== 工具 ==========

// inferBoard 根据代码和交易所推断板块
func inferBoard(symbol, exchange string) string {
	switch exchange {
	case "SSE":
		if strings.HasPrefix(symbol, "688") {
			return model.BoardStar
		}
		return model.BoardMain
	case "SZSE":
		if strings.HasPrefix(symbol, "300") {
			return model.BoardChiNext
		}
		return model.BoardMain
	case "BSE":
		return model.BoardBSE
	}
	return model.BoardMain
}
