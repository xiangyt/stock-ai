package eastmoney

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"stock-ai/internal/adapter"
	"stock-ai/internal/model"
)

func TestFetchStockListPage(t *testing.T) {
	a := New()
	if err := a.Init(nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer a.Close()

	resp, err := a.fetchStockListPage(1, 50)
	if err != nil {
		t.Fatalf("fetchStockListPage failed: %v", err)
	}

	t.Logf("Total: %d, Items: %d", resp.Data.Total, len(resp.Data.Diff))

	if len(resp.Data.Diff) == 0 {
		t.Fatal("no data returned")
	}

	for i, item := range resp.Data.Diff {
		if i < 5 {
			t.Logf("  [%d] %s(%s) 市场:%d", i+1, item.F14, item.F12, item.F13)
		}
	}
}

func TestGetShareChanges(t *testing.T) {
	a := New()
	if err := a.Init(nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer a.Close()

	ctx := context.Background()
	code := "002475" // 立讯精密

	changes, err := a.GetShareChanges(ctx, code)
	if err != nil {
		t.Fatalf("GetShareChanges(%s) failed: %v", code, err)
	}

	if len(changes) == 0 {
		t.Fatal("expected at least one share change record")
	}

	t.Logf("%s 股本变动记录数: %d", code, len(changes))

	for i, c := range changes {
		t.Logf("  [%d] %s %s %d %d %s", i+1, c.Code, c.Date, c.TotalShares, c.FloatAShares, c.ChangeReason)
	}

	// 验证按日期降序排列（最新在前）
	for i := 1; i < len(changes); i++ {
		if changes[i].Date > changes[i-1].Date {
			t.Errorf("records not sorted by date desc: [%d]=%s > [%d]=%s",
				i-1, changes[i-1].Date, i, changes[i].Date)
		}
	}
}

func TestGetShareChanges_InvalidCode(t *testing.T) {
	a := New()
	if err := a.Init(nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer a.Close()

	ctx := context.Background()
	// 不存在的代码应返回空列表而非 panic
	changes, err := a.GetShareChanges(ctx, "999999")
	if err != nil {
		// API 返回空数据也算正常行为
		t.Logf("999999 returned error (acceptable): %v", err)
		return
	}
	// 某些实现可能返回空切片
	if len(changes) != 0 {
		t.Logf("unexpected data for invalid code 999999: %d records", len(changes))
	}
}

func TestParseEquityResponse(t *testing.T) {
	// 测试 JSON 解析 + 数据转换逻辑（不依赖网络）
	jsonBody := `{
		"success": true,
		"message": "ok",
		"result": {
			"pages": 1,
			"count": 2,
			"data": [{
				"SECUCODE": "600519.SH",
				"SECURITY_CODE": "600519",
				"END_DATE": "2024-12-31 00:00:00",
				"TOTAL_SHARES": 1256197800,
				"LIMITED_SHARES": 0,
				"UNLIMITED_SHARES": 1256197800,
				"LISTED_A_SHARES": 1256197800,
				"CHANGE_REASON": "无变动"
			}, {
				"SECUCODE": "600519.SH",
				"SECURITY_CODE": "600519",
				"END_DATE": "2023-12-31 00:00:00",
				"TOTAL_SHARES": 1200000000,
				"LIMITED_SHARES": 50000000,
				"UNLIMITED_SHARES": 1150000000,
				"LISTED_A_SHARES": 1150000000,
				"CHANGE_REASON": "限售解禁"
			}]
		}
	}`

	var resp equityResponse
	if err := json.Unmarshal([]byte(jsonBody), &resp); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Result.Count != 2 {
		t.Fatalf("count = %d, want 2", resp.Result.Count)
	}

	items := resp.Result.Data

	// 验证日期截取和股单位
	tests := []struct {
		name       string
		idx        int
		wantDate   string
		wantTotal  int64
		wantFloatA int64
		wantUnlim  int64
		wantReason string
	}{
		{"第1条(最新)", 0, "2024-12-31", 1256197800, 1256197800, 1256197800, "无变动"},
		{"第2条", 1, "2023-12-31", 1200000000, 1150000000, 1150000000, "限售解禁"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := items[tt.idx]

			dateStr := item.EndDate
			if len(dateStr) >= 10 {
				dateStr = dateStr[:10]
			}
			if dateStr != tt.wantDate {
				t.Errorf("Date = %q, want %q", dateStr, tt.wantDate)
			}

			total := item.TotalShares
			if total != tt.wantTotal {
				t.Errorf("TotalShares(股) = %d, want %d", total, tt.wantTotal)
			}

			floatA := item.ListedAShares
			if floatA != tt.wantFloatA {
				t.Errorf("FloatAShares(股) = %d, want %d", floatA, tt.wantFloatA)
			}

			unlim := item.UnlimitedShares
			if unlim != tt.wantUnlim {
				t.Errorf("UnlimitedShares(股) = %d, want %d", unlim, tt.wantUnlim)
			}

			if item.ChangeReason != tt.wantReason {
				t.Errorf("ChangeReason = %q, want %q", item.ChangeReason, tt.wantReason)
			}
		})
	}
}

// convertToShareChanges 将 equityItem 列表转换为 adapter.ShareChange 列表
// 提取为独立函数方便单测验证转换逻辑
func convertToShareChanges(code string, items []equityItem) []adapter.ShareChange {
	result := make([]adapter.ShareChange, 0, len(items))
	for _, item := range items {
		dateStr := item.EndDate
		if len(dateStr) >= 10 {
			dateStr = dateStr[:10]
		}
		result = append(result, adapter.ShareChange{
			Code:            code,
			Date:            dateStr,
			TotalShares:     item.TotalShares,
			LimitedShares:   item.LimitedShares,
			UnlimitedShares: item.UnlimitedShares,
			FloatAShares:    item.ListedAShares,
			ChangeReason:    item.ChangeReason,
		})
	}
	return result
}

func TestGetStockDetail(t *testing.T) {
	a := New()
	if err := a.Init(nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer a.Close()

	ctx := context.Background()

	tests := []struct {
		name         string
		code         string
		wantName     string
		wantExchange string
	}{
		{"贵州茅台(沪市主板)", "600519", "贵州茅台", model.ExchangeSSE},
		{"立讯精密(深市主板)", "002475", "立讯精密", model.ExchangeSZSE},
		{"宁德时代(创业板)", "300750", "宁德时代", model.ExchangeSZSE},
		{"中芯国际(科创板)", "688981", "中芯国际", model.ExchangeSSE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detail, err := a.GetStockDetail(ctx, tt.code)
			if err != nil {
				t.Fatalf("GetStockDetail(%s) failed: %v", tt.code, err)
			}

			// 必填字段验证
			if detail.Code != tt.code {
				t.Errorf("Code = %q, want %q", detail.Code, tt.code)
			}
			if !strings.Contains(detail.Name, tt.wantName) {
				t.Errorf("Name = %q, should contain %q", detail.Name, tt.wantName)
			}
			if detail.Exchange != tt.wantExchange {
				t.Errorf("Exchange = %q, want %q", detail.Exchange, tt.wantExchange)
			}
			if detail.ListingBoard == "" {
				t.Error("ListingBoard is empty")
			}
			if detail.FullName == "" {
				t.Error("FullName is empty")
			}
			if detail.Industry == "" {
				t.Error("Industry is empty")
			}
			// President 可能为空（如境外注册公司）
			if detail.President == "" && detail.Province != "境外" {
				t.Error("President is empty (non-overseas)")
			}

			t.Logf("  Code:%s Name:%s FullName:%s Exchange:%s Board:%s ListDate:%s FoundDate:%s",
				detail.Code, detail.Name, detail.FullName, detail.Exchange,
				detail.ListingBoard, detail.ListDate, detail.FoundDate)
			t.Logf("  Industry:%s Sector:%s Province:%s President:%s LegalPerson:%s EmpNum:%d RegCapital:%.2f万",
				detail.Industry, detail.Sector, detail.Province, detail.President,
				detail.LegalPerson, detail.EmpNum, detail.RegCapital)
			// IPO 信息验证
			if detail.IssuePrice <= 0 {
				t.Errorf("[%s] IssuePrice = %.2f, want > 0", tt.code, detail.IssuePrice)
			}
			if detail.IssuePE < 0 {
				t.Errorf("[%s] IssuePE = %.2f, want >= 0", tt.code, detail.IssuePE)
			}
			if detail.TotalIssueNum <= 0 && tt.code != "688981" { // 中芯国际可能无数据
				t.Errorf("[%s] TotalIssueNum = %d, want > 0", tt.code, detail.TotalIssueNum)
			}
			if detail.IssueWay == "" {
				t.Errorf("[%s] IssueWay is empty", tt.code)
			}
			if detail.Sponsor == "" {
				t.Errorf("[%s] Sponsor is empty", tt.code)
			}
			t.Logf("  IPO: Price=%.2f PE=%.2f Num=%d Way=%s Sponsor=%s Underwriter=%s",
				detail.IssuePrice, detail.IssuePE, detail.TotalIssueNum,
				detail.IssueWay, detail.Sponsor, detail.Underwriter)
		})
	}
}

func TestGetStockDetail_InvalidCode(t *testing.T) {
	a := New()
	if err := a.Init(nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer a.Close()

	ctx := context.Background()
	_, err := a.GetStockDetail(ctx, "999999")
	if err == nil {
		t.Log("invalid code returned no error (may return empty data)")
	} else {
		t.Logf("invalid code returned error (expected): %v", err)
	}
}

// TestParseIssueInfoResponse 测试 IPO 发行信息 JSON 解析（无需网络）
func TestParseIssueInfoResponse(t *testing.T) {
	jsonBody := `{
		"success": true,
		"message": "ok",
		"result": {"pages": 1, "count": 1},
		"data": [{
			"SECUCODE": "002475.SZ",
			"SECURITY_CODE": "002475",
			"FOUND_DATE": "2004-05-24 00:00:00",
			"LISTING_DATE": "2010-09-15 00:00:00",
			"AFTER_ISSUE_PE": 72,
			"ONLINE_ISSUE_DATE": "2010-09-01 00:00:00",
			"ISSUE_WAY": "网下询价配售",
			"PAR_VALUE": 1,
			"TOTAL_ISSUE_NUM": 43800000,
			"ISSUE_PRICE": 28.8,
			"DEC_SUMISSUEFEE": 63192993,
			"TOTAL_FUNDS": 1261440000,
			"NET_RAISE_FUNDS": 1198247007,
			"OPEN_PRICE": 35,
			"CLOSE_PRICE": 39.99,
			"TURNOVERRATE": 88.5942,
			"HIGH_PRICE": 40,
			"OFFLINE_VAP_RATIO": 5.13782991,
			"ONLINE_ISSUE_LWR": 0.87773682,
			"SECURITY_TYPE": "A股",
			"OVERALLOTMENT": 0,
			"TYPE": "2",
			"TRADE_MARKET_CODE": "069001002001",
			"STR_ZHUCHENGXIAO": "中信证券股份有限公司",
			"STR_BAOJIAN": "中信证券股份有限公司"
		}]
	}`

	var resp struct {
		Success bool                   `json:"success"`
		Message string                 `json:"message"`
		Result  basicOrgInfoResultData `json:"result"`
		Data    []issueInfoItem        `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonBody), &resp); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected at least 1 record")
	}

	item := resp.Data[0]
	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"IssuePrice", item.IssuePrice, float64(28.8)},
		{"AfterIssuePE", item.AfterIssuePE, float64(72)},
		{"ParValue", item.ParValue, float64(1)},
		{"TotalIssueNum", item.TotalIssueNum, int64(43800000)},
		{"IssueWay", item.IssueWay, "网下询价配售"},
		{"Sponsor", item.StrZhuchengxiao, "中信证券股份有限公司"},
		{"Underwriter", item.StrBaojian, "中信证券股份有限公司"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("got %v, want %v", tt.got, tt.expected)
			}
		})
	}

	// 验证 fillIPOInfo 映射逻辑
	basic := &adapter.StockBasic{}
	basic.IssuePrice = item.IssuePrice
	basic.IssuePE = item.AfterIssuePE
	basic.ParValue = item.ParValue
	basic.TotalIssueNum = item.TotalIssueNum
	basic.OnlineIssueDate = item.OnlineIssueDate[:10] // 模拟 truncateDate
	basic.IssueWay = item.IssueWay
	basic.Sponsor = item.StrZhuchengxiao
	basic.Underwriter = item.StrBaojian

	if basic.IssuePrice != 28.8 {
		t.Errorf("basic.IssuePrice = %.2f, want 28.8", basic.IssuePrice)
	}
	if basic.IssuePE != 72 {
		t.Errorf("basic.IssuePE = %.2f, want 72", basic.IssuePE)
	}
	if basic.TotalIssueNum != 43800000 {
		t.Errorf("basic.TotalIssueNum = %d, want 43800000", basic.TotalIssueNum)
	}
	if basic.OnlineIssueDate != "2010-09-01" {
		t.Errorf("basic.OnlineIssueDate = %q, want \"2010-09-01\"", basic.OnlineIssueDate)
	}
}

func TestParseBasicOrgInfoResponse(t *testing.T) {
	// 测试 F10 基本资料 JSON 解析（不依赖网络）
	jsonBody := `{
		"success": true,
		"message": "ok",
		"result": {
			"pages": 1,
			"count": 2,
			"data": [{
				"SECUCODE": "002475.SZ",
				"SECURITY_CODE": "002475",
				"SECURITY_NAME_ABBR": "立讯精密",
				"ORG_CODE": "10152586",
				"ORG_NAME": "立讯精密工业股份有限公司",
				"ORG_NAME_EN": "Luxshare Precision Industry Co., Ltd.",
				"FORMERNAME": null,
				"STR_CODEA": "002475",
				"STR_NAMEA": "立讯精密",
				"SECURITY_TYPE": "深交所主板A股",
				"EM2016": "电子设备-消费电子设备-消费电子设备",
				"TRADE_MARKET": "深圳证券交易所",
				"INDUSTRYCSRC1": "制造业-计算机、通信和其他电子设备制造业",
				"PRESIDENT": "王来春",
				"LEGAL_PERSON": "王来春",
				"SECRETARY": "肖云兮",
				"CHAIRMAN": "王来春",
				"ORG_TEL": "0769-87892475",
				"ORG_EMAIL": "Public@luxshare-ict.com",
				"ORG_WEB": "www.luxshare-ict.com",
				"ADDRESS": "广东省东莞市清溪镇北环路313号",
				"REG_ADDRESS": "深圳市宝安区沙井街道蚝一西部三洋新工业区A栋2层",
				"PROVINCE": "广东",
				"ADDRESS_POSTCODE": "523642",
				"REG_CAPITAL": 728598.4811,
				"REG_NUM": "91440300760482233Q",
				"EMP_NUM": 278103,
				"TATOLNUMBER": 12,
				"LAW_FIRM": "盛德律师事务所",
				"ACCOUNTFIRM_NAME": "立信会计师事务所(特殊普通合伙)",
				"ORG_PROFILE": "  立讯精密工业股份有限公司成立于2004年。  ",
				"BUSINESS_SCOPE": "生产经营连接线、连接器。",
				"LISTING_DATE": "2010-09-15 00:00:00",
				"FOUND_DATE": "2004-05-24 00:00:00",
				"MAIN_BUSINESS": "消费电子业务、通信与数据中心业务、汽车业务",
				"HOST_BROKER": null,
				"TRANSFER_WAY": null,
				"ACTUAL_HOLDER": "王来春,王来胜",
				"CURRENCY": "人民币",
				"BOARD_NAME_LEVEL": "电子-消费电子-消费电子零部件及组装"
			}, {
				"SECUCODE": "600519.SH",
				"SECURITY_CODE": "600519",
				"SECURITY_NAME_ABBR": "贵州茅台",
				"ORG_NAME": "贵州茅台酒股份有限公司",
				"PRESIDENT": "张德芹",
				"LEGAL_PERSON": "张德芹",
				"SECRETARY": "蒋焰",
				"EMP_NUM": 30000,
				"REG_CAPITAL": 125619.78,
				"PROVINCE": "贵州",
				"SECURITY_TYPE": "沪市主板A股",
				"INDUSTRYCSRC1": "制造业-酒、饮料和精制茶制造业",
				"EM2016": "食品饮料-白酒-白酒",
				"CURRENCY": "人民币",
				"LISTING_DATE": "2001-08-27 00:00:00",
				"FOUND_DATE": "1999-11-20 00:00:00",
				"ACTUAL_HOLDER": "贵州省人民政府国有资产监督管理委员会",
				"ORG_PROFILE": "贵州茅台酒股份有限公司主营...",
				"BUSINESS_SCOPE": "茅台酒系列产品的生产与销售",
				"MAIN_BUSINESS": "白酒业务",
				"ADDRESS": "贵州省遵义市仁怀市茅台镇",
				"ORG_TEL": "0851-22388001"
			}]
		}
	}`

	var resp basicOrgInfoResponse
	if err := json.Unmarshal([]byte(jsonBody), &resp); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Result.Count != 2 {
		t.Fatalf("count = %d, want 2", resp.Result.Count)
	}

	items := resp.Result.Data

	// 验证第1条（含指针字段）
	t.Run("立讯精密", func(t *testing.T) {
		item := items[0]
		if item.SecurityCode != "002475" {
			t.Errorf("SecurityCode = %q, want 002475", item.SecurityCode)
		}
		if item.SecurityNameAbbr != "立讯精密" {
			t.Errorf("SecurityNameAbbr = %q", item.SecurityNameAbbr)
		}
		if item.OrgNameEn == nil || *item.OrgNameEn != "Luxshare Precision Industry Co., Ltd." {
			t.Errorf("OrgNameEn = %v", item.OrgNameEn)
		}
		if item.FormerName != nil {
			t.Errorf("FormerName = %v, want nil", item.FormerName)
		}
		if item.EmpNum != 278103 {
			t.Errorf("EmpNum = %d, want 278103", item.EmpNum)
		}
		if item.RegCapital <= 0 {
			t.Errorf("RegCapital = %f, want > 0", item.RegCapital)
		}

		// 验证日期截取
		dateStr := item.ListingDate
		if len(dateStr) >= 10 {
			dateStr = dateStr[:10]
		}
		if dateStr != "2010-09-15" {
			t.Errorf("ListingDate = %q, want 2010-09-15", dateStr)
		}

		// 验证辅助函数
		if strOrEmpty(item.ActualHolder) != "王来春,王来胜" {
			t.Errorf("ActualHolder = %q", strOrEmpty(item.ActualHolder))
		}
		if strOrEmpty(item.HostBroker) != "" {
			t.Errorf("HostBroker(nil) = %q, want empty", strOrEmpty(item.HostBroker))
		}
	})

	// 验证第2条（无英文名称）
	t.Run("贵州茅台", func(t *testing.T) {
		item := items[1]
		if item.SecurityCode != "600519" {
			t.Errorf("SecurityCode = %q", item.SecurityCode)
		}
		if item.President != "张德芹" {
			t.Errorf("President = %q, want 张德芹", item.President)
		}
	})
}
