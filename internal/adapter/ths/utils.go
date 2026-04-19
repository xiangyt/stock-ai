package ths

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stock-ai/internal/adapter/helpers"
	"stock-ai/internal/model"
)

// ========== HTTP 请求辅助 ==========

// doRequest 发送请求并处理限流、gzip解压
func (a *Adapter) doRequest(req *http.Request) (*http.Response, error) {
	if err := a.limiter.Wait(context.Background()); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %v", err)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return resp, nil
}

// readBody 读取响应体（自动解压gzip）
func (a *Adapter) readBody(resp *http.Response) ([]byte, error) {
	reader := io.Reader(resp.Body)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("create gzip reader failed: %w", err)
		}
		defer gr.Close()
		reader = gr
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// makeTHSRequest 发送同花顺通用请求
func (a *Adapter) makeTHSRequest(url string) (string, error) {
	req, _ := http.NewRequest("GET", url, nil)
	a.setTHSHeaders(req, "https://stockpage.10jqka.com.cn/")
	resp, err := a.doRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := a.readBody(resp)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// makeTodayDataRequest 发送当日数据请求（完整cookie + hexin-v）
func (a *Adapter) makeTodayDataRequest(url string) (string, error) {
	req, _ := http.NewRequest("GET", url, nil)
	a.setTHSTodayHeaders(req)
	hexinV := generateWencaiToken()
	a.setTHSCookie(req, hexinV)
	req.Header.Set("Hexin-V", hexinV)
	resp, err := a.doRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := a.readBody(resp)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ========== 请求头设置 ==========

// setTHSCommonHeaders 设置同花顺通用请求头
func (a *Adapter) setTHSCommonHeaders(req *http.Request, refer string) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-fetch-dest", "script")
	req.Header.Set("sec-fetch-mode", "no-cors")
	req.Header.Set("sec-fetch-site", "same-site")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("pragma", "no-cache")
	req.Header.Set("User-Agent", a.currentUA)
	req.Header.Set("sec-ch-ua", a.userAgentGen.GenerateSecChUa(a.currentUA))
	req.Header.Set("sec-ch-ua-platform", `"`+helpers.GetPlatformFromUA(a.currentUA)+`"`)
	req.Header.Set("Referer", refer)
}

// setTHSHeaders 设置K线请求头（简化版）
func (a *Adapter) setTHSHeaders(req *http.Request, refer string) {
	a.setTHSCommonHeaders(req, refer)
}

// setTHSTodayHeaders 设置当日数据请求头（完整版）
func (a *Adapter) setTHSTodayHeaders(req *http.Request) {
	a.setTHSCommonHeaders(req, "https://stockpage.10jqka.com.cn/")
	req.Header.Set("If-Modified-Since", "Sun, 05 Oct 2025 06:43:51 GMT")
}

// setTHSCookie 设置同花顺Cookie
func (a *Adapter) setTHSCookie(req *http.Request, hexinV string) {
	timestamp := time.Now().Unix()
	cookieValue := fmt.Sprintf(
		"Hm_lvt_722143063e4892925903024537075d0=%d; "+
			"HMACCOUNT=17C55F0F7B5ABE69; Hm_lvt_929f8b362150b1f77b477230541dbbc2=%d; "+
			"Hm_lvt_78c58f01938e4d85eaf619eae71b4ed1=%d; Hm_lvt_69929b9dce4c22a060bd22d703b2a280=%d; "+
			"spversion=20130314; historystock=600930%%7C*%%7C001208%%7C*%%7C600930; "+
			"Hm_lpvt_929f8b362150b1f77b477230541dbbc2=%d; Hm_lpvt_69929b9dce4c22a060bd22d703b2a280=%d; "+
			"Hm_lpvt_722143063e4892925903024537075d0d=%d; "+
			"Hm_lpvt_78c58f01938e4d85eaf619eae71b4ed1=%d; v=%s",
		timestamp, timestamp, timestamp, timestamp,
		timestamp, timestamp, timestamp, timestamp, hexinV,
	)
	req.Header.Set("Cookie", cookieValue)
}

// ========== UA 管理 ==========

// updateUserAgent 更新随机UA（每分钟）
func (a *Adapter) updateUserAgent() {
	a.currentUA = a.userAgentGen.GenerateUserAgent()
	a.lastUpdateTime = time.Now()
}

func (a *Adapter) maybeUpdateUA() {
	if time.Since(a.lastUpdateTime) > 1*time.Minute {
		a.updateUserAgent()
	}
}

// ========== 代码转换辅助函数 ==========

// buildTHSCode 构建同花顺代码格式 hs_xxxxxx
func buildTHSCode(symbol string) string {
	switch {
	case strings.HasPrefix(symbol, "60"), strings.HasPrefix(symbol, "68"):
		return "hs_" + symbol
	case strings.HasPrefix(symbol, "00"), strings.HasPrefix(symbol, "30"),
		strings.HasPrefix(symbol, "8"), strings.HasPrefix(symbol, "4"):
		return "hs_" + symbol
	default:
		return ""
	}
}

// detectExchange 检测交易所和板块
func detectExchange(code string) (market, exchange, board string) {
	switch {
	case strings.HasPrefix(code, "60"), strings.HasPrefix(code, "68"):
		return "SH", model.ExchangeSSE, detectBoard(code)
	case strings.HasPrefix(code, "000"), strings.HasPrefix(code, "001"),
		strings.HasPrefix(code, "002"), strings.HasPrefix(code, "003"):
		return "SZ", model.ExchangeSZSE, detectBoard(code)
	default:
		return "SZ", model.ExchangeSZSE, model.BoardBSE
	}
}

func detectBoard(code string) string {
	switch {
	case strings.HasPrefix(code, "300"):
		return model.BoardChiNext
	case strings.HasPrefix(code, "688"):
		return model.BoardStar
	case strings.HasPrefix(code, "689"):
		return model.BoardBSE
	default:
		return model.BoardMain
	}
}

// parseCode 解析代码
func (a *Adapter) parseCode(code string) (string, string, error) {
	parts := strings.Split(code, ".")
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	return code, "", nil
}

// ========== HTML 解析辅助 ==========

// extractBetween 提取两个标记之间的字符串
func extractBetween(s, left, right string) string {
	start := strings.Index(s, left)
	if start == -1 {
		return ""
	}
	start += len(left)
	end := strings.Index(s[start:], right)
	if end == -1 {
		return ""
	}
	return s[start : start+end]
}

// isValidStockName 判断是否为有效股票名称
func (a *Adapter) isValidStockName(name string) bool {
	if len(name) == 0 || len(name) > 20 {
		return false
	}
	if strings.Contains(name, "<") || strings.Contains(name, ">") {
		return false
	}
	for _, r := range name {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

// extractNameFromLines 从多行中提取股票名称
func (a *Adapter) extractNameFromLines(lines []string, idx int) string {
	for j := idx; j < len(lines) && j < idx+5; j++ {
		if name := extractBetween(lines[j], `title="`, `"`); name != "" && a.isValidStockName(name) {
			return name
		}
	}
	for j := idx + 1; j < len(lines) && j < idx+5; j++ {
		if name := extractBetween(lines[j], `alt="`, `"`); name != "" && a.isValidStockName(name) {
			return name
		}
	}
	for j := idx + 1; j < len(lines) && j < idx+10; j++ {
		nextLine := strings.TrimSpace(lines[j])
		if a.isValidStockName(nextLine) {
			return nextLine
		}
	}
	return ""
}

// ========== Token 生成 ==========

// generateWencaiToken 生成问财token（调用helpers包）
func generateWencaiToken() string {
	return helpers.GenerateWencaiToken()
}

// ========== 数据解析辅助 ==========

// getString 从map获取字符串值
func getString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%.0f", val)
		}
		return fmt.Sprintf("%f", val)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// parseFloat 安全解析浮点数
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

// parseInt64 安全解析整数
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

// formatTradeDate 格式化交易日期
func formatTradeDate(dateInt int) string {
	if dateInt == 0 {
		return time.Now().Format("2006-01-02")
	}
	dateStr := strconv.Itoa(dateInt)
	if len(dateStr) == 8 {
		return dateStr[:4] + "-" + dateStr[4:6] + "-" + dateStr[6:8]
	}
	return dateStr
}

// yuanToCents 将元(float64)转换为分(int64)，四舍五入
func yuanToCents(yuan float64) int64 {
	return int64(yuan*100 + 0.5)
}
