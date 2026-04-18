package helpers

import (
	"stock-ai/internal/adapter"
	"fmt"
	"strconv"
	"strings"
)

// KLineParser 东方财富K线数据解析器
// 从 stock 项目移植
type KLineParser struct{}

// NewKLineParser 创建K线解析器
func NewKLineParser() *KLineParser {
	return &KLineParser{}
}

// ParseDailyKline 解析东方财富日K线字符串
// 格式(11字段): "2010-05-14,3.15,2.53,3.41,2.15,285595,598452980.53,41.72,-16.23,-0.49,106.57"
//            字段: 日期,开盘,收盘,最高,最低,成交量(手),成交额(元),...,换手率(最后1个)
// 价格单位: 元 → 内部统一转为 分(int64)
func (p *KLineParser) ParseDailyKline(code, kline string) (*adapter.StockPriceDaily, error) {
	fields := strings.Split(kline, ",")
	if len(fields) < 7 {
		return nil, fmt.Errorf("invalid kline format: %s", kline)
	}

	open := toCents(fields[1])
	close := toCents(fields[2])
	high := toCents(fields[3])
	low := toCents(fields[4])
	volume := parseInt64(fields[5]) * 100 // 东方财富单位是手
	amount := toCents(fields[6])

	// 换手率(最后1个字段)，可选字段，解析失败则默认0
	turnover := parseFloat(fields[len(fields)-1])

	return &adapter.StockPriceDaily{
		Code:     code,
		Date:     fields[0],
		Open:     open,
		High:     high,
		Low:      low,
		Close:    close,
		Volume:   volume,
		Amount:   amount,
		Turnover: turnover,
	}, nil
}

// ParseTHSDailyKline 解析同花顺日K线数据
// 同花顺格式：价格已为分单位(lowCent + offset)，无需转换
func (p *KLineParser) ParseTHSDailyKline(code string, dates []string, prices []string, volumes []string) ([]*adapter.StockPriceDaily, error) {
	result := make([]*adapter.StockPriceDaily, 0, len(dates))

	for i := range dates {
		if len(prices) < (i+1)*4 || len(volumes) <= i {
			break
		}

		lowCent, _ := strconv.Atoi(prices[i*4])
		openOffset, _ := strconv.Atoi(prices[i*4+1])
		highOffset, _ := strconv.Atoi(prices[i*4+2])
		closeOffset, _ := strconv.Atoi(prices[i*4+3])
		vol, _ := strconv.ParseInt(volumes[i], 10, 64)

		result = append(result, &adapter.StockPriceDaily{
			Code:   code,
			Date:   dates[i],
			Open:   int64(lowCent + openOffset),
			High:   int64(lowCent + highOffset),
			Low:    int64(lowCent),
			Close:  int64(lowCent + closeOffset),
			Volume: vol,
			Amount: int64(lowCent + closeOffset) * vol, // 分 × 股 = 分(金额)
		})
	}

	return result, nil
}

// ParseFloat 安全解析浮点数
func (p *KLineParser) ParseFloat(s string) float64 {
	return parseFloat(s)
}

// ParseInt64 安全解析整数
func (p *KLineParser) ParseInt64(s string) int64 {
	return parseInt64(s)
}

// ParseTradeDate 将日期字符串转为 YYYYMMDD 整数
func (p *KLineParser) ParseTradeDate(dateStr string) (int, error) {
	if len(dateStr) == 10 && strings.Contains(dateStr, "-") {
		parts := strings.Split(dateStr, "-")
		if len(parts) == 3 {
			y, _ := strconv.Atoi(parts[0])
			m, _ := strconv.Atoi(parts[1])
			d, _ := strconv.Atoi(parts[2])
			return y*10000 + m*100 + d, nil
		}
	} else if len(dateStr) == 8 {
		val, err := strconv.Atoi(dateStr)
		return val, err
	}
	return 0, fmt.Errorf("invalid date format: %s", dateStr)
}

// parseFloat 全局辅助函数
func parseFloat(s string) float64 {
	if s == "" || s == "-" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// parseInt64 全局辅助函数
func parseInt64(s string) int64 {
	if s == "" || s == "-" {
		return 0
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

// toCents 将价格字符串(元)转换为分(int64)，零浮点精度损失
// 东财价格固定2位小数，直接去掉小数点即为"分"
//
// 示例: "11.20"→1120  "-2.72"→-272  "3.25"→325
func toCents(s string) int64 {
	if s == "" || s == "-" {
		return 0
	}
	dotIdx := strings.Index(s, ".")
	if dotIdx >= 0 && len(s)-dotIdx-1 != 2 {
		// 小数点后不是2位，说明数据格式变了，需要排查
		fmt.Printf("WARNING: 价格小数位异常(%d位): %s\n", len(s)-dotIdx-1, s)
		return 0
	}
	clean := strings.ReplaceAll(s, ".", "")
	val, err := strconv.ParseInt(clean, 10, 64)
	if err != nil {
		return 0
	}
	return val
}
