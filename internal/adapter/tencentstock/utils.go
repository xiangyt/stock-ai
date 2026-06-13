package tencentstock

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ========== HTTP 请求辅助 ==========

// doGet 发起限速 GET 请求，自动带反爬 Header 和 gzip 解压
func (a *Adapter) doGet(ctx context.Context, urlStr, referer string) (string, error) {
	if err := a.limiter.Wait(ctx); err != nil {
		return "", err
	}
	a.refreshUA()

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", err
	}
	setHeaders(req, a.currentUA, referer)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", fmt.Errorf("gzip reader 失败: %w", err)
		}
		defer gr.Close()
		reader = gr
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("读取响应体失败: %w", err)
	}
	return string(body), nil
}

// setHeaders 设置腾讯行情 API 反爬请求头
func setHeaders(req *http.Request, ua, referer string) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", referer)
	req.Header.Set("User-Agent", ua)
}

// ========== 股票代码转换 ==========

// toTencentCode 将6位纯数字代码转为腾讯格式（sh600519 / sz000001 / bj830946）
func toTencentCode(code string) string {
	code = strings.TrimSpace(code)
	// 已带前缀则直接返回（大小写不敏感）
	lower := strings.ToLower(code)
	if strings.HasPrefix(lower, "sh") || strings.HasPrefix(lower, "sz") ||
		strings.HasPrefix(lower, "bj") || strings.HasPrefix(lower, "hk") ||
		strings.HasPrefix(lower, "us") {
		return lower
	}
	return detectPrefix(code) + lower
}

// detectPrefix 根据代码前缀推断市场前缀
func detectPrefix(code string) string {
	switch {
	case strings.HasPrefix(code, "60"), strings.HasPrefix(code, "68"),
		strings.HasPrefix(code, "000"), strings.HasPrefix(code, "399"),
		strings.HasPrefix(code, "51"):
		// 上交所：60xxxx / 68xxxx（科创板）；399xxx（深证指数用sz）；510xxx ETF
		if strings.HasPrefix(code, "60") || strings.HasPrefix(code, "68") {
			return MarketSH
		}
		return MarketSZ
	case strings.HasPrefix(code, "00"), strings.HasPrefix(code, "30"),
		strings.HasPrefix(code, "15"), strings.HasPrefix(code, "16"),
		strings.HasPrefix(code, "18"):
		return MarketSZ
	case strings.HasPrefix(code, "8"), strings.HasPrefix(code, "4"):
		return MarketBJ
	default:
		return MarketSZ
	}
}

// splitTencentCode 分拆腾讯格式代码为 (prefix, symbol)，如 "sh600519" → ("sh","600519")
func splitTencentCode(tc string) (prefix, symbol string) {
	tc = strings.ToLower(tc)
	for _, p := range []string{"sh", "sz", "bj", "hk", "us"} {
		if strings.HasPrefix(tc, p) {
			return p, tc[len(p):]
		}
	}
	return "", tc
}

// tencentToExchange 腾讯前缀 → 标准交易所名
func tencentToExchange(prefix string) string {
	switch prefix {
	case "sh":
		return "SSE"
	case "sz":
		return "SZSE"
	case "bj":
		return "BSE"
	default:
		return ""
	}
}

// ========== 数值工具 ==========

// parseFloat 安全解析 float64，解析失败返回 0
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "--" || s == "null" {
		return 0
	}
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

// parseInt 安全解析 int64，解析失败返回 0
func parseInt(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "--" {
		return 0
	}
	var v int64
	fmt.Sscanf(s, "%d", &v)
	return v
}

// truncateDate 截取日期前缀 "20260501" → "2026-05-01"
// 同时处理 "2026-05-01" 和 "20260501" 两种格式
func truncateDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 8 {
		// YYYYMMDD → YYYY-MM-DD
		return s[:4] + "-" + s[4:6] + "-" + s[6:8]
	}
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
