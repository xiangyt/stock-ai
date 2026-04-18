package eastmoney

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"stock-ai/internal/model"
)

// ========== HTTP 请求辅助 ==========

// makeGetRequest 发送GET请求（带限流和反爬）
func (a *Adapter) makeGetRequest(urlStr, refer string) (string, error) {
	ctx := context.Background()

	if err := a.limiter.Wait(ctx); err != nil {
		return "", err
	}

	if time.Since(a.lastUpdateTime) > 1*time.Minute {
		a.updateHeaders()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", err
	}

	setCommonHeaders(req, a.currentUA, a.currentCookie, refer)
	req.Header.Set("Referer", refer)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// updateHeaders 更新随机UA和Cookie
func (a *Adapter) updateHeaders() {
	a.currentUA = a.userAgentGen.GenerateUserAgent()
	a.currentCookie = a.cookieGen.GenerateCookie()
	a.lastUpdateTime = time.Now()
}

// ========== 辅助函数 ==========

// setCommonHeaders 设置公共请求头
func setCommonHeaders(req *http.Request, ua, cookie, refer string) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("Origin", "https://emweb.securities.eastmoney.com")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", refer)
	req.Header.Set("Sec-Ch-Ua", "\"Chromium\";v=\"146\", \"Not-A.Brand\";v=\"24\", \"Google Chrome\";v=\"146\"")
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", "\"macOS\"")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("User-Agent", ua)
}

// extractJSONP 从JSONP响应中提取JSON
func extractJSONP(body string) string {
	re := regexp.MustCompile(`jQuery[\d_]+\((.*)\)`)
	matches := re.FindStringSubmatch(body)
	if len(matches) >= 2 {
		return matches[1]
	}
	re2 := regexp.MustCompile(`jsonp\d+\((.*)\)`)
	matches2 := re2.FindStringSubmatch(body)
	if len(matches2) >= 2 {
		return matches2[1]
	}
	return body
}

// parseCode 解析代码，返回 symbol 和 market
func parseCode(code string) (string, string) {
	parts := strings.Split(code, ".")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return code, ""
}

// buildSecID 构建东方财富 secid
func buildSecID(symbol, market string) string {
	switch market {
	case "SH", "SSE":
		return "1." + symbol
	case "SZ", "SZSE":
		return "0." + symbol
	case "BSE":
		return "0." + symbol
	default:
		if strings.HasPrefix(symbol, "60") || strings.HasPrefix(symbol, "68") {
			return "1." + symbol
		}
		return "0." + symbol
	}
}

// buildSecucode 根据代码构建东财 SECUCODE 格式 (如 002404.SZ / 600000.SH)
func buildSecucode(symbol string) string {
	switch {
	case strings.HasPrefix(symbol, "60"), strings.HasPrefix(symbol, "68"):
		return symbol + ".SH"
	case strings.HasPrefix(symbol, "8"), strings.HasPrefix(symbol, "4"):
		return symbol + ".BJ"
	default:
		return symbol + ".SZ"
	}
}

// detectExchangeAndBoard 根据代码判断交易所和板块
func detectExchangeAndBoard(code string) (exchange, board string) {
	switch {
	case strings.HasPrefix(code, "600"), strings.HasPrefix(code, "601"),
		strings.HasPrefix(code, "603"), strings.HasPrefix(code, "605"):
		return model.ExchangeSSE, model.BoardMain
	case strings.HasPrefix(code, "688"):
		return model.ExchangeSSE, model.BoardStar
	case strings.HasPrefix(code, "000"), strings.HasPrefix(code, "001"),
		strings.HasPrefix(code, "002"), strings.HasPrefix(code, "003"):
		return model.ExchangeSZSE, model.BoardMain
	case strings.HasPrefix(code, "300"):
		return model.ExchangeSZSE, model.BoardChiNext
	case strings.HasPrefix(code, "8"), strings.HasPrefix(code, "4"):
		return model.ExchangeBSE, model.BoardBSE
	default:
		return model.ExchangeSZSE, model.BoardMain
	}
}

// getQuoteReferURL 获取行情页面的Refer URL
func getQuoteReferURL(code string) string {
	symbol, market := parseCode(code)
	switch market {
	case "SH", "SSE":
		return fmt.Sprintf("https://quote.eastmoney.com/sh%s.html", symbol)
	default:
		return fmt.Sprintf("https://quote.eastmoney.com/sz%s.html", symbol)
	}
}

// toCents 将元(float64)转换为分(int64)，四舍五入
func toCents(yuan float64) int64 {
	return int64(yuan*100 + 0.5)
}

// truncateDate 截取日期 "2010-09-15 00:00:00" → "2010-09-15"
func truncateDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// strOrEmpty 安全解引用 *string 指针
func strOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// trimSpaces 去除首尾空白和多余空格
func trimSpaces(s string) string {
	return strings.TrimSpace(s)
}
