package tencentstock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/helpers"
)

// qtRefer 腾讯实时行情接口 Referer
const qtRefer = "https://gu.qq.com/"

// ========== 实时/当期行情 ==========

// GetTodayData 获取当日实时行情快照
func (a *Adapter) GetTodayData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	return a.fetchSnapshot(ctx, code)
}

// GetThisWeekData 获取本周截止当前的数据（取当日快照）
func (a *Adapter) GetThisWeekData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

// GetThisMonthData 获取本月截止当前的数据（取当日快照）
func (a *Adapter) GetThisMonthData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

// GetThisQuarterData 获取本季截止当前的数据（取当日快照）
func (a *Adapter) GetThisQuarterData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

// GetThisYearData 获取本年截止当前的数据（取当日快照）
func (a *Adapter) GetThisYearData(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	return nil, adapter.ErrNotImplemented
}

// ========== 实时快照内部实现 ==========

// fetchSnapshot 拉取腾讯实时行情快照
// 接口：https://sqt.gtimg.cn/utf8/q=sh600519
// 返回格式（~分隔，双引号内为数据段）：
//
//	v_sh600519="1~贵州茅台~600519~1290.20~1311.00~1310.95~49157~...";
//
// 字段位置（0-based，基于真实 sqt.gtimg.cn 接口返回数据验证，总计约84字段）：
//
//	基础身份（0-2）：
//	  0=类型(1=股票), 1=名称, 2=代码
//	价量核心（3-6）：*已采集*
//	  3=当前价, 4=昨收, 5=今开, 6=成交量(手)
//	内外盘（7-8）：
//	  7=外盘, 8=内盘
//	卖五/买五档（9-28）：
//	  9=卖一价, 10=卖一量, 11=卖二价, 12=卖二量, 13=卖三价, 14=卖三量
//	  15=卖四价, 16=卖四量, 17=卖五价, 18=卖五量
//	  19=买一价, 20=买一量, 21=买二价, 22=买二量, 23=买三价, 24=买三量
//	  25=买四价, 26=买四量, 27=买五价, 28=买五量
//	时间/涨跌（30-34）：*已采集*
//	  30=日期时间(YYYYMMDDHHmmss), 31=涨跌额, 32=涨跌幅%, 33=最高, 34=最低
//	冗余/中间字段（35-36）：
//	  35=价量额串, 36=成交量(手,重复)
//	行情指标（37-39, 57）：*已采集*
//	  37=成交额(万元,整数), 38=换手率%, 39=市盈率(TTM)*
//	  57=成交额(万元,高精度,含小数)*
//	冗余高低（41-42）：
//	  41=最高(重复), 42=最低(重复)
//	估值/波动（43-49）：
//	  43=振幅%, 44=流通市值(亿), 45=总市值(亿)*, 46=市净率*
//	  47=52周最高, 48=52周最低, 49=量比
//	均价/PE（51-53）：
//	  51=均价, 52=市盈率(动), 53=市盈率(静)
//	冗余/未知（56-59）：
//	  56=?, 57=成交额(万元,高精度), 58=?, 59=?
//	证券类型（61）：
//	  61=证券类型(如GP-A)
//	区间涨跌幅（62-66）：
//	  62=5日涨幅%, 63=10日涨幅%, 64=20日涨幅%, 65=60日涨幅%, 66=年初至今涨幅%
//	历史极值（67-68）：
//	  67=历史最高价, 68=历史最低价
//	未知（69-71, 74-75, 79-80）：
//	  69=?, 70=?, 71=?, 74=?, 75=?
//	股本（72-76）：
//	  72=总股本, 73=流通股本, 76=流通股本(重复)
//	货币/状态（79-84）：
//	  79=?, 80=?, 82=货币(CNY), 84=状态标识
//
// * 表示已采集字段
func (a *Adapter) fetchSnapshot(ctx context.Context, code string) (*adapter.StockPriceDaily, error) {
	tc := toTencentCode(code)
	urlStr := fmt.Sprintf("https://sqt.gtimg.cn/utf8/q=%s&r=%d", tc, time.Now().UnixMilli())

	body, err := a.doGet(ctx, urlStr, qtRefer)
	if err != nil {
		return nil, fmt.Errorf("fetchSnapshot 请求失败: %w", err)
	}

	return parseSnapshotLine(body, code)
}

// parseSnapshotLine 解析腾讯实时行情字符串
func parseSnapshotLine(body, origCode string) (*adapter.StockPriceDaily, error) {
	// 找到引号内的数据段
	start := strings.Index(body, "\"")
	end := strings.LastIndex(body, "\"")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("行情响应格式异常: %q", truncateStr(body, 200))
	}
	dataStr := body[start+1 : end]
	fields := strings.Split(dataStr, "~")
	if len(fields) < 40 {
		return nil, fmt.Errorf("行情字段不足(%d个): %q", len(fields), truncateStr(body, 200))
	}

	// 字段解析（基于真实 sqt.gtimg.cn 接口返回数据验证）
	curPrice := helpers.ParsePriceToCents(fields[3])   // 当前价(分)
	todayOpen := helpers.ParsePriceToCents(fields[5])   // 今开(分)
	volume := parseInt(fields[6])                        // 成交量（手，×100=股）
	// 高低价：fields[33]=最高, fields[34]=最低
	var high, low int64
	if len(fields) > 34 {
		high = helpers.ParsePriceToCents(fields[33])
		low = helpers.ParsePriceToCents(fields[34])
	}
	// 成交额：fields[57]，单位：万元 → 分
	var amount int64
	if len(fields) > 57 {
		amount = helpers.ParseWanYuanToCents(fields[57])
	}
	// 涨跌额/幅：fields[31]=涨跌额, fields[32]=涨跌幅%
	var change int64
	var changePct float64
	if len(fields) > 32 {
		change = helpers.ParsePriceToCents(fields[31])
		changePct = parseFloat(fields[32])
	}
	// 换手率：fields[38]
	var turnover float64
	if len(fields) > 38 {
		turnover = parseFloat(fields[38])
	}
	// 市盈率(TTM)：fields[39]
	var pe float64
	if len(fields) > 39 {
		pe = parseFloat(fields[39])
	}
	// 市净率：fields[46]
	var pb float64
	if len(fields) > 46 {
		pb = parseFloat(fields[46])
	}
	// 总市值：fields[45]，单位：亿（已是亿，无需转换）
	var marketCap float64
	if len(fields) > 45 {
		marketCap = parseFloat(fields[45])
	}
	// 日期：fields[30]，格式 YYYYMMDDHHmmss（如 "20260522161417"），截取前8位
	date := time.Now().Format("2006-01-02")
	if len(fields) > 30 && len(fields[30]) >= 8 {
		date = truncateDate(fields[30][:8])
	}

	if high == 0 && curPrice > 0 {
		high = curPrice
	}
	if low == 0 && curPrice > 0 {
		low = curPrice
	}
	if todayOpen == 0 {
		return nil, fmt.Errorf("今日未开盘")
	}

	return &adapter.StockPriceDaily{
		Code:      origCode,
		Date:      date,
		Open:      todayOpen,
		High:      high,
		Low:       low,
		Close:     curPrice,
		Volume:    volume * 100, // 手→股
		Amount:    amount,
		Change:    change,
		ChangePct: changePct,
		Turnover:  turnover,
		Pe:        pe,
		Pb:        pb,
		MarketCap: marketCap,
	}, nil
}

// ========== 批量快照（供 stock_list 复用）==========

// batchSnapshotResponse 腾讯批量行情接口响应（JSON格式）
type batchSnapshotResponse struct {
	Data map[string]snapshotItem `json:"data"`
}

type snapshotItem struct {
	Name      string      `json:"name"`
	Code      string      `json:"code"`
	Price     json.Number `json:"price"`
	Open      json.Number `json:"open"`
	High      json.Number `json:"high"`
	Low       json.Number `json:"low"`
	Close     json.Number `json:"close"` // 昨收
	Volume    int64       `json:"volume"`
	Amount    float64     `json:"amount"`
	ChangePct float64     `json:"percent"`
}

// fetchBatchSnapshot 批量拉取行情（最多200个/次），返回 tc→StockPriceDaily map
func (a *Adapter) fetchBatchSnapshot(ctx context.Context, codes []string) (map[string]*adapter.StockPriceDaily, error) {
	// 腾讯批量接口：pricatstock/api/stock/info?_var=xx&code=sh600519,sz000001
	tcCodes := make([]string, len(codes))
	for i, c := range codes {
		tcCodes[i] = toTencentCode(c)
	}

	urlStr := fmt.Sprintf(
		"https://proxy.finance.qq.com/ifzqgtimg/appstock/app/mktg/batchhq?_var=batchhq&code=%s&r=%d",
		strings.Join(tcCodes, ","),
		time.Now().UnixMilli(),
	)
	body, err := a.doGet(ctx, urlStr, qtRefer)
	if err != nil {
		return nil, err
	}
	// 去掉 JS 变量前缀
	if idx := strings.Index(body, "="); idx >= 0 {
		body = body[idx+1:]
	}

	var resp batchSnapshotResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &resp); err != nil {
		return nil, fmt.Errorf("批量行情JSON解析失败: %w", err)
	}

	result := make(map[string]*adapter.StockPriceDaily, len(resp.Data))
	today := time.Now().Format("2006-01-02")
	for tc, item := range resp.Data {
		priceCents := helpers.ParsePriceToCents(item.Price.String())
		h := helpers.ParsePriceToCents(item.High.String())
		l := helpers.ParsePriceToCents(item.Low.String())
		if h == 0 {
			h = priceCents
		}
		if l == 0 {
			l = priceCents
		}
		_, symbol := splitTencentCode(tc)
		result[tc] = &adapter.StockPriceDaily{
			Code:      symbol,
			Date:      today,
			Open:      helpers.ParsePriceToCents(item.Open.String()),
			High:      h,
			Low:       l,
			Close:     priceCents,
			Volume:    item.Volume,
			Amount:    int64(item.Amount * 100),
			ChangePct: item.ChangePct,
		}
	}
	return result, nil
}

// truncateStr 安全截断字符串，用于错误信息
func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
