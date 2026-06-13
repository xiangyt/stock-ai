package helpers

import (
	"strconv"
	"strings"
)

// ParsePriceToCents 将价格字符串(元)转换为分(int64)，零浮点精度损失。
//
// 支持格式: "11.20"→1120  "-2.72"→-272  "3.25"→325  "15.2"→1520  "5"→500
//
// 核心思路: 去掉小数点直接当整数解析，消除 float64 精度问题。
//
//	< 2位小数: 末尾补0  → "15.2" → "1520" → 1520
//	= 2位小数: 直接解析  → "40.69" → "4069" → 4069
//	> 2位小数: 截断到2位  → "1.234" → "123" → 123
//	无小数点: 补2个0     → "5" → "500" → 500
func ParsePriceToCents(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "--" || s == "null" {
		return 0
	}

	sign := int64(1)
	if s[0] == '-' {
		sign = -1
		s = s[1:]
	}

	dotIdx := strings.Index(s, ".")
	if dotIdx >= 0 {
		intPart := s[:dotIdx]
		decPart := s[dotIdx+1:]
		// 截断/补齐到2位小数
		if len(decPart) > 2 {
			decPart = decPart[:2]
		}
		for len(decPart) < 2 {
			decPart += "0"
		}
		s = intPart + decPart
	} else {
		// 无小数点 → 元，补2位小数
		s += "00"
	}

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return sign * val
}

// ParseWanYuanToCents 将金额字符串(万元)转换为分(int64)，零浮点精度损失。
//
// 腾讯K线按日K返回原始数据中成交额单位为万元（如 "28433.88"）。
//
// 转换: "28433.88" 万元 → 去掉小数点 → 2843388 → ×10000 → 28433880000 分
//
// 证明:
//
//	"x.yy" 万元 = (x*100 + yy)*10000 分 = 去掉小数点后的整数 × 10000
func ParseWanYuanToCents(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "--" || s == "null" {
		return 0
	}

	// 去掉小数点
	clean := strings.ReplaceAll(s, ".", "")
	val, err := strconv.ParseInt(clean, 10, 64)
	if err != nil {
		return 0
	}
	// 万元 → 分：× 10000(万→元) × 100(元→分) = × 1,000,000
	// 但上面去掉小数点相当于已经乘了 100（2位小数），所以只需 × 10,000
	return val * 10000
}
