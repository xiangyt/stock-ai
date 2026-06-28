package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ============================================================================
//  东方财富自选股 / 分组管理 API
//
//  基础 URL: https://myfavor.eastmoney.com/v4/webouter/ag
//  appkey:    e9166c7e9cdfad3aa3fd7d93b757e9b1（东财自选页专用 key）
//  Cookie:    需要登录态，由调用方传入
// ============================================================================

const (
	favorBaseURL  = "https://myfavor.eastmoney.com/v4/webouter"
	favorAppKey   = "e9166c7e9cdfad3aa3fd7d93b757e9b1"
	favorReferer  = "https://quote.eastmoney.com/zixuan/?from=home"
	favorPathList = "/ggdefstkindexinfos" // 获取分组列表
	favorPathAg   = "/ag"                 // 新建分组
	favorPathDg   = "/dg"                 // 删除分组
	favorPathAs   = "/as"                 // 加入自选
	favorPathDs   = "/dslot"              // 移出自选
)

// ============================================================================
//  公共响应结构
// ============================================================================

type favorResponse struct {
	State   int             `json:"state"`   // 0=成功
	Message string          `json:"message"` // 状态消息
	Data    json.RawMessage `json:"data"`    // 具体数据（类型依接口而异）
}

// CreateGroupResponse 新建分组响应。
type CreateGroupResponse struct {
	GID         string `json:"gid"`          // 分组 ID
	GName       string `json:"gname"`        // 分组名称
	FromClient  string `json:"fromclient"`   // 客户端来源 "web"
	GroupSource string `json:"group_source"` // 分组来源
}

// GroupInfo 分组信息（列表项）。
type GroupInfo struct {
	GID        string `json:"gid"`        // 分组 ID
	GName      string `json:"gname"`      // 分组名称
	FromClient string `json:"fromclient"` // 客户端来源
	Ver        int    `json:"ver"`        // 版本号
	Full       bool   `json:"full"`       // 是否满员
}

// listGroupsData 分组列表 API 响应 data 字段结构。
type listGroupsData struct {
	GInfoList []GroupInfo `json:"ginfolist"`
}

// doFavorRequest 发起自选股 API 通用请求。
// path 为 API 路径（如 favorPathAg / favorPathDg）。
// cookie 必须包含登录态（由调用方传入）。
func (a *Adapter) doFavorRequest(ctx context.Context, cookie string, path string, params url.Values) (*favorResponse, error) {
	params.Set("appkey", favorAppKey)
	params.Set("_", fmt.Sprintf("%d", nowMillis()))

	reqURL := favorBaseURL + path + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建自选请求失败: %w", err)
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Referer", favorReferer)
	req.Header.Set("Cookie", cookie)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("自选请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取自选响应失败: %w", err)
	}

	// 响应可能是 JSONP（含 jQuery callback 包装），也可能是纯 JSON。
	// 统一处理：去掉 JSONP 包装后再解析。
	raw := stripJSONP(string(body))
	if raw == "" {
		return nil, fmt.Errorf("自选响应为空")
	}

	var favResp favorResponse
	if err := json.Unmarshal([]byte(raw), &favResp); err != nil {
		return nil, fmt.Errorf("解析自选响应失败: %w, body=%s", err, raw)
	}

	if favResp.State != 0 {
		return nil, fmt.Errorf("自选API失败: %s (state=%d)", favResp.Message, favResp.State)
	}

	return &favResp, nil
}

// stripJSONP 去除 JSONP 回调包装，返回纯 JSON 字符串。
// 例如: "jQuery123({...})" → "{...}"
func stripJSONP(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "jQuery") || strings.HasPrefix(raw, "jsonp") {
		// 找到第一个 '(' 和最后一个 ')' 之间的内容
		start := strings.IndexByte(raw, '(')
		end := strings.LastIndexByte(raw, ')')
		if start >= 0 && end > start {
			return strings.TrimSpace(raw[start+1 : end])
		}
	}
	return raw
}

// nowMillis 返回当前 Unix 毫秒时间戳，用于 _ 参数防缓存。
func nowMillis() int64 {
	return time.Now().UnixMilli()
}

// ============================================================================
//  新建分组 — gn=分组名称，cookie 作为入参传入
//
//  返回新建分组的 gid（字符串型 ID）。
// ============================================================================

// CreateGroup 新建自选分组。
// cookie 为登录态 Cookie 字符串（必填）。
// groupName 为分组名称（必填）。
func (a *Adapter) CreateGroup(ctx context.Context, cookie, groupName string) (*CreateGroupResponse, error) {
	if cookie == "" {
		return nil, fmt.Errorf("cookie 不能为空")
	}
	if groupName == "" {
		return nil, fmt.Errorf("分组名称不能为空")
	}

	params := url.Values{
		"gn": {groupName},
	}

	favResp, err := a.doFavorRequest(ctx, cookie, favorPathAg, params)
	if err != nil {
		return nil, err
	}

	var data CreateGroupResponse
	if err := json.Unmarshal(favResp.Data, &data); err != nil {
		return nil, fmt.Errorf("解析新建分组响应数据失败: %w", err)
	}

	return &data, nil
}

// ============================================================================
//  获取分组列表 — 无额外参数，cookie 作为入参传入
// ============================================================================

// ListGroups 获取用户的所有自选分组列表。
// cookie 为登录态 Cookie 字符串（必填）。
func (a *Adapter) ListGroups(ctx context.Context, cookie string) ([]GroupInfo, error) {
	if cookie == "" {
		return nil, fmt.Errorf("cookie 不能为空")
	}

	favResp, err := a.doFavorRequest(ctx, cookie, favorPathList, url.Values{})
	if err != nil {
		return nil, err
	}

	var data listGroupsData
	if err := json.Unmarshal(favResp.Data, &data); err != nil {
		return nil, fmt.Errorf("解析分组列表响应数据失败: %w", err)
	}

	return data.GInfoList, nil
}

// ============================================================================
//  删除分组 — g=分组ID，cookie 作为入参传入
// ============================================================================

// DeleteGroup 删除自选分组。
// cookie 为登录态 Cookie 字符串（必填）。
// gid 为分组 ID（字符串，"12" 等）。
func (a *Adapter) DeleteGroup(ctx context.Context, cookie, gid string) error {
	if cookie == "" {
		return fmt.Errorf("cookie 不能为空")
	}
	if gid == "" {
		return fmt.Errorf("分组ID不能为空")
	}

	params := url.Values{
		"g": {gid},
	}

	_, err := a.doFavorRequest(ctx, cookie, favorPathDg, params)
	return err
}

// ============================================================================
//  加入自选 — g=分组ID, sc=市场前缀$股票代码
//  市场前缀: 688/689 → "1", 其余 → "0"
// ============================================================================

// AddToGroup 将股票加入自选分组。
// cookie 为登录态 Cookie 字符串（必填）。
// gid 为分组 ID。
// stockCode 为 6 位股票代码（如 "688060"、"000001"）。
func (a *Adapter) AddToGroup(ctx context.Context, cookie, gid, stockCode string) error {
	if cookie == "" {
		return fmt.Errorf("cookie 不能为空")
	}
	if gid == "" {
		return fmt.Errorf("分组ID不能为空")
	}
	if stockCode == "" {
		return fmt.Errorf("股票代码不能为空")
	}

	params := url.Values{
		"g":  {gid},
		"sc": {formatStockCode(stockCode)},
	}

	_, err := a.doFavorRequest(ctx, cookie, favorPathAs, params)
	return err
}

// ============================================================================
//  移出自选 — g=分组ID, scs=市场前缀$股票代码（formatStockCode 格式）
// ============================================================================

// RemoveFromGroup 将股票从自选分组中移除。
// cookie 为登录态 Cookie 字符串（必填）。
// gid 为分组 ID。
// stockCode 为 6 位股票代码（自动格式化市场前缀）。
func (a *Adapter) RemoveFromGroup(ctx context.Context, cookie, gid, stockCode string) error {
	if cookie == "" {
		return fmt.Errorf("cookie 不能为空")
	}
	if gid == "" {
		return fmt.Errorf("分组ID不能为空")
	}
	if stockCode == "" {
		return fmt.Errorf("股票代码不能为空")
	}

	params := url.Values{
		"g":   {gid},
		"scs": {formatStockCode(stockCode)},
	}

	_, err := a.doFavorRequest(ctx, cookie, favorPathDs, params)
	return err
}

// formatStockCode 将 6 位股票代码转为东财自选 API 格式: 市场前缀$代码。
// 60xxxx（沪市主板）/ 688/689（科创板）→ "1$XXXXXX"，其余 → "0$XXXXXX"。
func formatStockCode(code string) string {
	if len(code) >= 3 && (code[:2] == "60" || code[:3] == "688" || code[:3] == "689") {
		return "1$" + code
	}
	return "0$" + code
}
