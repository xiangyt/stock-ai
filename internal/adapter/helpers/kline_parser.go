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
// 格式: "2026-04-12,10.50,11.20,10.30,11.00,123456,678901.00"
// 价格单位: 元 → 内部统一转为 分(int64)
func (p *KLineParser) ParseDailyKline(code, kline string) (*adapter.StockPriceDaily, error) {
	fields := strings.Split(kline, ",")
	if len(fields) < 7 {
		return nil, fmt.Errorf("invalid kline format: %s", kline)
	}

	open := toCents(parseFloat(fields[1]))
	close := toCents(parseFloat(fields[2]))
	high := toCents(parseFloat(fields[3]))
	low := toCents(parseFloat(fields[4]))
	volume := parseInt64(fields[5]) * 100 // 东方财富单位是手
	amount := toCents(parseFloat(fields[6]))

	return &adapter.StockPriceDaily{
		Code:   code,
		Date:   fields[0],
		Open:   open,
		High:   high,
		Low:    low,
		Close:  close,
		Volume: volume,
		Amount: amount,
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

// toCents 将元(float64)转换为分(int64)，四舍五入
func toCents(yuan float64) int64 {
	return int64(yuan*100 + 0.5)
}
