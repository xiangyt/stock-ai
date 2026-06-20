package tencentstock

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/helpers"
)

// ============================================================================
//  GetIntraday 分时数据
//
//  接口: https://proxy.finance.qq.com/ifzqgtimg/appstock/app/minute/query
//  参数:
//    _var=min_data_{tc}  — JS 变量名前缀
//    code={tc}           — 腾讯格式代码 (sh600519 / sz000001)
//    r={random}          — 随机数
//
//  返回 JSONP:
//    min_data_{tc}={
//      "code": 0,
//      "data": {
//        "{tc}": {
//          "qt": { "{tc}": [...] },                         // quote 数组
//          "data": { "data": ["0930 14.61 6 8766.00",...] } // 分时 bar
//        }
//      }
//    }
//
//  分时 bar 格式: "HHmm 价格 成交量(手) 成交额(元)"
//  腾讯代码: sh600519 / sz000001 / bj830946
// ============================================================================

type intradayResponse struct {
	Code int                    `json:"code"`
	Msg  string                 `json:"msg"`
	Data map[string]intradayRaw `json:"data"`
}

type intradayRaw struct {
	Qt   map[string][]string     `json:"qt"`
	Data intradayMinuteContainer `json:"data"`
}

type intradayMinuteContainer struct {
	Data []string `json:"data"`
	Date string   `json:"date"` // "20260618" (YYYYMMDD)
}

// GetIntraday 获取当日分时行情
func (a *Adapter) GetIntraday(ctx context.Context, code string) (*adapter.IntradayData, error) {
	tc := toTencentCode(code)
	urlStr := fmt.Sprintf(
		"https://proxy.finance.qq.com/ifzqgtimg/appstock/app/minute/query?_var=min_data_%s&code=%s&r=%d",
		tc, tc, time.Now().UnixMilli(),
	)

	body, err := a.doGet(ctx, urlStr, qtRefer)
	if err != nil {
		return nil, fmt.Errorf("GetIntraday 请求失败: %w", err)
	}

	return parseIntradayResponse(body, code, tc)
}

// parseIntradayResponse 解析分时行情 JSONP
func parseIntradayResponse(body, origCode, tc string) (*adapter.IntradayData, error) {
	// 提取 JSONP body: min_data_{tc}={...}
	if idx := strings.Index(body, "="); idx >= 0 {
		body = body[idx+1:]
	}
	body = strings.TrimSpace(body)

	var resp intradayResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("GetIntraday JSON解析失败: %w", err)
	}

	raw, ok := resp.Data[tc]
	if !ok {
		return nil, fmt.Errorf("GetIntraday 未找到代码 %s 的数据", tc)
	}

	// 解析分时 bar
	var bars []adapter.IntradayBar
	for _, line := range raw.Data.Data {
		bar, err := parseMinuteBar(line)
		if err != nil {
			continue
		}
		bars = append(bars, bar)
	}

	if len(bars) == 0 {
		return nil, fmt.Errorf("GetIntraday %s 无分时数据", origCode)
	}

	// 从 qt 数组提取完整行情数据
	result := parseQtFields(raw.Qt, raw.Data.Date, tc, origCode)
	result.Bars = bars

	return result, nil
}

// parseQtFields 从 qt 数组提取完整行情数据
//
// qt 数组字段位置（基于 proxy.finance.qq.com minute/query 接口验证）：
//
//	0=市场状态,  1=名称,  2=代码,  3=当前价,  4=昨收,  5=今开
//	6=成交量(手), 7=外盘, 8=内盘, 9-28=买卖五档
//	30=日期时间(YYYYMMDDHHmmss), 31=涨跌额, 32=涨跌幅%, 33=最高, 34=最低
//	35=价格/量/额串, 36=成交量(手,重复), 37=成交额(万元)
//	38=换手率%, 39=市盈率(TTM), 43=振幅%
//	44=流通市值(亿), 45=总市值(亿), 46=市净率
//
// rawDate: API 返回的 date 字段 (YYYYMMDD)，优先使用
func parseQtFields(qt map[string][]string, rawDate, tc, origCode string) *adapter.IntradayData {
	var result adapter.IntradayData
	result.Code = origCode

	qtArr, ok := qt[tc]
	if !ok || len(qtArr) < 47 {
		if ok && len(qtArr) >= 5 {
			// Minimal: only basic fields
			result.Name = getArr(qtArr, 1)
			result.PreClose = helpers.ParsePriceToCents(getArr(qtArr, 4))
		}
		return &result
	}

	// === 基础身份 (1-2) ===
	result.Name = qtArr[1]

	// === 价量核心 (3-6) ===
	result.Current = helpers.ParsePriceToCents(qtArr[3])
	result.PreClose = helpers.ParsePriceToCents(qtArr[4])
	result.Open = helpers.ParsePriceToCents(qtArr[5])
	result.Volume = parseVolume(qtArr[6])

	// === 涨跌 (31-32) ===
	result.Change = helpers.ParsePriceToCents(qtArr[31])
	result.ChangePct = parseFloat(qtArr[32])

	// === 高低 (33-34) ===
	result.High = helpers.ParsePriceToCents(qtArr[33])
	result.Low = helpers.ParsePriceToCents(qtArr[34])
	if result.High == 0 && result.Current > 0 {
		result.High = result.Current
	}
	if result.Low == 0 && result.Current > 0 {
		result.Low = result.Current
	}

	// === 成交额/换手/估值 (37-39) ===
	result.Amount = helpers.ParseWanYuanToCents(qtArr[37])
	result.Turnover = parseFloat(qtArr[38])
	result.Pe = parseFloat(qtArr[39])

	// === 五档盘口 (7-28) ===
	result.Depth = parseDepth(qtArr)

	// === 振幅 (43) ===
	result.Amplitude = parseFloat(qtArr[43])

	// === 市值/市净率 (44-46) ===
	result.FloatMarketCap = parseFloat(qtArr[44])
	result.MarketCap = parseFloat(qtArr[45])
	result.Pb = parseFloat(qtArr[46])

	// === 日期：优先使用 API 返回的 date 字段 (YYYYMMDD) ===
	if len(rawDate) == 8 {
		result.Date = rawDate[:4] + "-" + rawDate[4:6] + "-" + rawDate[6:8]
	} else if len(qtArr) > 30 && len(qtArr[30]) >= 8 {
		raw := qtArr[30][:8]
		result.Date = raw[:4] + "-" + raw[4:6] + "-" + raw[6:8]
	}
	if result.Date == "" {
		result.Date = time.Now().Format("2006-01-02")
	}

	return &result
}

// parseVolume 解析成交量（手 → 股）
func parseVolume(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v * 100
}

// parseDepth 从 qt 数组提取五档买卖盘口
//
//	qt[9-18]=卖一~卖五 (价,量,价,量...)
//	qt[19-28]=买一~买五 (价,量,价,量...)
//	价格单位: 元→分, 数量单位: 手→股
func parseDepth(qtArr []string) *adapter.MarketDepth {
	if len(qtArr) < 29 {
		return nil
	}
	return &adapter.MarketDepth{
		Ask1Price:  helpers.ParsePriceToCents(qtArr[9]),
		Ask1Volume: parseVolume(qtArr[10]),
		Ask2Price:  helpers.ParsePriceToCents(qtArr[11]),
		Ask2Volume: parseVolume(qtArr[12]),
		Ask3Price:  helpers.ParsePriceToCents(qtArr[13]),
		Ask3Volume: parseVolume(qtArr[14]),
		Ask4Price:  helpers.ParsePriceToCents(qtArr[15]),
		Ask4Volume: parseVolume(qtArr[16]),
		Ask5Price:  helpers.ParsePriceToCents(qtArr[17]),
		Ask5Volume: parseVolume(qtArr[18]),
		Bid1Price:  helpers.ParsePriceToCents(qtArr[19]),
		Bid1Volume: parseVolume(qtArr[20]),
		Bid2Price:  helpers.ParsePriceToCents(qtArr[21]),
		Bid2Volume: parseVolume(qtArr[22]),
		Bid3Price:  helpers.ParsePriceToCents(qtArr[23]),
		Bid3Volume: parseVolume(qtArr[24]),
		Bid4Price:  helpers.ParsePriceToCents(qtArr[25]),
		Bid4Volume: parseVolume(qtArr[26]),
		Bid5Price:  helpers.ParsePriceToCents(qtArr[27]),
		Bid5Volume: parseVolume(qtArr[28]),
	}
}

// getArr 安全获取数组元素，越界返回空字符串
func getArr(arr []string, idx int) string {
	if idx < len(arr) {
		return arr[idx]
	}
	return ""
}

// parseMinuteBar 解析单行分时数据 "HHmm price volume(手) amount(元)"
// volume 单位: 手, 需 ×100 转为股
// amount 单位: 元(浮点), 需 ×100 转为分
func parseMinuteBar(line string) (adapter.IntradayBar, error) {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return adapter.IntradayBar{}, fmt.Errorf("分时 bar 字段不足: %s", line)
	}

	// 时间: HHmm → HH:mm
	timeStr := parts[0]
	if len(timeStr) == 4 {
		timeStr = timeStr[:2] + ":" + timeStr[2:]
	}

	// 价格: 元 → 分
	price := helpers.ParsePriceToCents(parts[1])
	if price == 0 && parts[1] != "0" && parts[1] != "0.00" {
		return adapter.IntradayBar{}, fmt.Errorf("分时 bar 价格无法解析: %s", line)
	}

	// 成交量: 手 → 股
	volume := int64(0)
	if len(parts) >= 3 {
		if v, err := strconv.ParseInt(parts[2], 10, 64); err == nil {
			volume = v * 100
		}
	}

	// 成交额: 元(浮点) → 分
	amount := int64(0)
	if len(parts) >= 4 {
		if f, err := strconv.ParseFloat(parts[3], 64); err == nil {
			amount = int64(f * 100)
		}
	}

	return adapter.IntradayBar{
		Time:   timeStr,
		Price:  price,
		Volume: volume,
		Amount: amount,
	}, nil
}


