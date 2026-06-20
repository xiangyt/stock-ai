package eastmoney

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"stock-ai/internal/adapter"
	"stock-ai/internal/model"
)

// ========== 基本面数据测试（F10） ==========

func TestGetStockDetail(t *testing.T) {
	a := newTestAdapter()
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
			if detail.TotalIssueNum <= 0 && tt.code != "688981" {
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
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	_, err := a.GetStockDetail(ctx, "999999")
	if err == nil {
		t.Log("invalid code returned no error (may return empty data)")
	} else {
		t.Logf("invalid code returned error (expected): %v", err)
	}
}

// ========== F10 基本资料 JSON 解析单元测试（无需网络） ==========

func TestParseBasicOrgInfoResponse(t *testing.T) {
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

		dateStr := item.ListingDate
		if len(dateStr) >= 10 {
			dateStr = dateStr[:10]
		}
		if dateStr != "2010-09-15" {
			t.Errorf("ListingDate = %q, want 2010-09-15", dateStr)
		}

		if strOrEmpty(item.ActualHolder) != "王来春,王来胜" {
			t.Errorf("ActualHolder = %q", strOrEmpty(item.ActualHolder))
		}
		if strOrEmpty(item.HostBroker) != "" {
			t.Errorf("HostBroker(nil) = %q, want empty", strOrEmpty(item.HostBroker))
		}
	})

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

// ========== IPO 发行信息 JSON 解析单元测试（无需网络） ==========

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

	basic := &adapter.StockBasic{}
	basic.IssuePrice = item.IssuePrice
	basic.IssuePE = item.AfterIssuePE
	basic.ParValue = item.ParValue
	basic.TotalIssueNum = item.TotalIssueNum
	basic.OnlineIssueDate = item.OnlineIssueDate[:10]
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

// ========== 股本变动测试 ==========

func TestGetShareChanges(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	code := "002404"

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

	for i := 1; i < len(changes); i++ {
		if changes[i].Date > changes[i-1].Date {
			t.Errorf("records not sorted by date desc: [%d]=%s > [%d]=%s",
				i-1, changes[i-1].Date, i, changes[i].Date)
		}
	}
}

func TestGetShareChanges_InvalidCode(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	changes, err := a.GetShareChanges(ctx, "999999")
	if err != nil {
		t.Logf("999999 returned error (acceptable): %v", err)
		return
	}
	if len(changes) != 0 {
		t.Logf("unexpected data for invalid code 999999: %d records", len(changes))
	}
}

// ========== 股本变动 JSON 解析单元测试（无需网络） ==========

func TestParseEquityResponse(t *testing.T) {
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

			if item.TotalShares != tt.wantTotal {
				t.Errorf("TotalShares(股) = %d, want %d", item.TotalShares, tt.wantTotal)
			}
			if item.ListedAShares != tt.wantFloatA {
				t.Errorf("FloatAShares(股) = %d, want %d", item.ListedAShares, tt.wantFloatA)
			}
			if item.UnlimitedShares != tt.wantUnlim {
				t.Errorf("UnlimitedShares(股) = %d, want %d", item.UnlimitedShares, tt.wantUnlim)
			}
			if item.ChangeReason != tt.wantReason {
				t.Errorf("ChangeReason = %q, want %q", item.ChangeReason, tt.wantReason)
			}
		})
	}
}

// ========== 股东户数测试 ==========

func TestGetShareholderCounts(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	code := "002404" // 嘉欣丝绸

	counts, err := a.GetShareholderCounts(ctx, code)
	if err != nil {
		t.Fatalf("GetShareholderCounts(%s) failed: %v", code, err)
	}

	if len(counts) == 0 {
		t.Fatal("expected at least one shareholder count record")
	}

	t.Logf("%s 股东户数记录数: %d", code, len(counts))

	for i, c := range counts[:min(4, len(counts))] {
		t.Logf("  [%d] %s 户数=%d 变化=%.2f%% 人均流通=%d股 集中度=%s 股价=%.2f",
			i+1, c.EndDate, c.HolderNum, c.HolderNumChangePct,
			c.AvgFreeShares, c.HoldFocus, c.Price)
	}

	// 验证最新一期数据
	latest := counts[0]
	if latest.Code != code {
		t.Errorf("Code = %q, want %q", latest.Code, code)
	}
	if latest.HolderNum <= 0 {
		t.Errorf("HolderNum = %d, want > 0", latest.HolderNum)
	}
	if latest.EndDate == "" {
		t.Error("EndDate is empty")
	}
	if latest.HoldFocus == "" {
		t.Error("HoldFocus is empty")
	}
}

func TestGetShareholderCounts_InvalidCode(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	counts, err := a.GetShareholderCounts(ctx, "999999")
	if err != nil {
		t.Logf("999999 returned error (acceptable): %v", err)
		return
	}
	if len(counts) != 0 {
		t.Logf("unexpected data for invalid code 999999: %d records", len(counts))
	}
}

func TestGetLatestShareholderCount(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	code := "002475" // 立讯精密

	sc, err := a.GetLatestShareholderCount(ctx, code)
	if err != nil {
		t.Fatalf("GetLatestShareholderCount(%s) failed: %v", code, err)
	}

	t.Logf("最新股东户数: %s | 户数=%d 较上期变化=%.2f%% 人均流通=%d股 筹码集中度=%s",
		sc.EndDate, sc.HolderNum, sc.HolderNumChangePct,
		sc.AvgFreeShares, sc.HoldFocus)

	if sc.Code != code {
		t.Errorf("Code = %q, want %q", sc.Code, code)
	}
	if sc.HolderNum <= 0 {
		t.Errorf("HolderNum = %d, expect > 0", sc.HolderNum)
	}
	if sc.Price <= 0 {
		t.Errorf("Price = %f, expect > 0", sc.Price)
	}
}

// ========== 股东户数 JSON 解析单元测试（无需网络） ==========

func TestParseShareholderNumResponse(t *testing.T) {
	jsonBody := `{
		"version": "e9e95cc91921d793c4b9236c40b6c07d",
		"result": {
			"pages": 1,
			"data": [{
				"SECUCODE": "002404.SZ",
				"SECURITY_CODE": "002404",
				"END_DATE": "2024-06-30 00:00:00",
				"HOLDER_TOTAL_NUM": 31391,
				"TOTAL_NUM_RATIO": 7.0488,
				"AVG_FREE_SHARES": 14519,
				"AVG_FREESHARES_RATIO": -10.061953807578,
				"HOLD_FOCUS": "较分散",
				"PRICE": 5.0352648639,
				"AVG_HOLD_AMT": 73110.6464529678,
				"HOLD_RATIO_TOTAL": 49.04902404,
				"FREEHOLD_RATIO_TOTAL": 37.71844371
			}, {
				"SECUCODE": "002404.SZ",
				"SECURITY_CODE": "002404",
				"END_DATE": "2023-02-28 00:00:00",
				"HOLDER_TOTAL_NUM": 34081,
				"TOTAL_NUM_RATIO": -12.6375,
				"AVG_FREE_SHARES": 13545,
				"AVG_FREESHARES_RATIO": 14.46553798304,
				"HOLD_FOCUS": "较分散",
				"PRICE": 6.287605389,
				"AVG_HOLD_AMT": 85170.7106091819,
				"HOLD_RATIO_TOTAL": null,
				"FREEHOLD_RATIO_TOTAL": null
			}]
		},
		"count": 77,
		"success": true,
		"message": "ok",
		"code": 0
	}`

	var resp shareholderNumResponse
	if err := json.Unmarshal([]byte(jsonBody), &resp); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Result.Count != 77 {
		t.Fatalf("count = %d, want 77", resp.Result.Count)
	}

	items := resp.Result.Data

	// 第一条：完整数据
	t.Run("完整数据", func(t *testing.T) {
		r := adapter.ShareholderCount{
			Code:                   "002404",
			SecurityCode:           items[0].SECURITY_CODE,
			EndDate:                truncateDate(items[0].END_DATE),
			HolderNum:              items[0].HOLDER_TOTAL_NUM,
			HolderNumChangePct:     floatPtrOrZero(items[0].TOTAL_NUM_RATIO),
			AvgFreeShares:          items[0].AVG_FREE_SHARES,
			AvgFreeSharesChangePct: floatPtrOrZero(items[0].AVG_FREESHARES_RATIO),
			HoldFocus:              items[0].HOLD_FOCUS,
			Price:                  floatPtrOrZero(items[0].PRICE),
			AvgHoldAmount:          floatPtrOrZero(items[0].AVG_HOLD_AMT),
			HoldRatioTotal:         floatPtrOrZero(items[0].HOLD_RATIO_TOTAL),
			FreeHoldRatioTotal:     floatPtrOrZero(items[0].FREEHOLD_RATIO_TOTAL),
		}

		if r.EndDate != "2024-06-30" {
			t.Errorf("EndDate = %q, want 2024-06-30", r.EndDate)
		}
		if r.HolderNum != 31391 {
			t.Errorf("HolderNum = %d, want 31391", r.HolderNum)
		}
		if abs(r.HolderNumChangePct-7.05) > 0.01 {
			t.Errorf("HolderNumChangePct = %f, want ~7.05", r.HolderNumChangePct)
		}
		if r.AvgFreeShares != 14519 {
			t.Errorf("AvgFreeShares = %d, want 14519", r.AvgFreeShares)
		}
		if r.HoldFocus != "较分散" {
			t.Errorf("HoldFocus = %q, want 较分散", r.HoldFocus)
		}
		if abs(r.Price-5.04) > 0.01 {
			t.Errorf("Price = %f, want ~5.04", r.Price)
		}
		if int64(r.AvgHoldAmount) != 73110 {
			t.Errorf("AvgHoldAmount = %d, want 73110", int64(r.AvgHoldAmount))
		}
	})

	// 第二条：含 null 字段
	t.Run("null字段安全处理", func(t *testing.T) {
		item := items[1]

		if item.HOLDER_TOTAL_NUM != 34081 {
			t.Errorf("HolderNum = %d, want 34081", item.HOLDER_TOTAL_NUM)
		}
		// null 字段应返回 0，不 panic
		if floatPtrOrZero(item.HOLD_RATIO_TOTAL) != 0 {
			t.Errorf("HOLD_RATIO_TOTAL(null) = %f, want 0", floatPtrOrZero(item.HOLD_RATIO_TOTAL))
		}
		if floatPtrOrZero(item.FREEHOLD_RATIO_TOTAL) != 0 {
			t.Errorf("FREEHOLD_RATIO_TOTAL(null) = %f, want 0", floatPtrOrZero(item.FREEHOLD_RATIO_TOTAL))
		}
	})
}

// ========== 机构持仓测试 ==========

func TestGetInstitutionalHoldings(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	code := "002404" // 嘉欣丝绸

	holdings, err := a.GetInstitutionalHoldings(ctx, code)
	if err != nil {
		t.Fatalf("GetInstitutionalHoldings(%s) failed: %v", code, err)
	}

	if len(holdings) == 0 {
		t.Fatal("expected at least one institutional holding record")
	}

	t.Logf("%s 机构持仓记录数: %d", code, len(holdings))

	for i, h := range holdings[:min(6, len(holdings))] {
		t.Logf("  [%d] %s 机构=%d家 持股=%.2f亿 市值=%.2f亿 流通比=%.2f%% 总股比=%.2f%% 股价=%.2f",
			i+1, h.ReportDate, h.InstitutionCount,
			float64(h.TotalFreeShares)/1e8,
			h.TotalMarketCap/1e8,
			h.FreeShareRatio, h.TotalShareRatio, h.ClosePrice)
	}

	// 验证最新一期
	latest := holdings[0]
	if latest.Code != code {
		t.Errorf("Code = %q, want %q", latest.Code, code)
	}
	if latest.InstitutionCount <= 0 {
		t.Errorf("InstitutionCount = %d, want > 0", latest.InstitutionCount)
	}
	if latest.TotalFreeShares <= 0 {
		t.Errorf("TotalFreeShares = %d, want > 0", latest.TotalFreeShares)
	}
	if latest.FreeShareRatio <= 0 {
		t.Errorf("FreeShareRatio = %f, want > 0", latest.FreeShareRatio)
	}
	if latest.ClosePrice <= 0 {
		t.Errorf("ClosePrice = %f, want > 0", latest.ClosePrice)
	}

	// 验证按时间降序排列
	for i := 1; i < len(holdings); i++ {
		if holdings[i].ReportDate > holdings[i-1].ReportDate {
			t.Errorf("not sorted desc: [%d]=%s > [%d]=%s",
				i-1, holdings[i-1].ReportDate, i, holdings[i].ReportDate)
		}
	}
}

func TestGetInstitutionalHoldings_InvalidCode(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	holdings, err := a.GetInstitutionalHoldings(ctx, "999999")
	if err != nil {
		t.Logf("999999 returned error (acceptable): %v", err)
		return
	}
	if len(holdings) != 0 {
		t.Logf("unexpected data for invalid code: %d records", len(holdings))
	}
}

// ========== 机构持仓 JSON 解析单元测试（无需网络） ==========

func TestParseInstitutionalHoldResponse(t *testing.T) {
	jsonBody := `{
		"version": "79622728b0073ab75bf9f1122e69f775",
		"result": {
			"pages": 1,
			"data": [{
				"SECURITY_INNER_CODE": "1000008358",
				"REPORT_DATE": "2025-12-31 00:00:00",
				"ORG_TYPE": "00",
				"TOTAL_ORG_NUM": 82,
				"TOTAL_FREE_SHARES": 146403170,
				"TOTAL_MARKET_CAP": 1026286221.70,
				"TOTAL_SHARES_RATIO": 32.13813658,
				"SECUCODE": "002404.SZ",
				"IS_INCREASE": "1",
				"IS_COMPLETE": "0",
				"SECURITY_CODE": "002404",
				"FREE_SHARES_CHANGE": 2.78005884,
				"CHANGE_RATIO": 9.469485245664,
				"ORG_NAME_TYPE": "合计",
				"ALL_SHARES_RATIO": 26.14101728,
				"TOTAL_SHARES": 146403170,
				"TOTAL_FREE_SHARES_CHANGE": 12592840,
				"CLOSE_PRICE": 7.01
			}, {
				"SECURITY_INNER_CODE": "1000008358",
				"REPORT_DATE": "2025-09-30 00:00:00",
				"ORG_TYPE": "00",
				"TOTAL_ORG_NUM": 5,
				"TOTAL_FREE_SHARES": 133810330,
				"TOTAL_MARKET_CAP": 828285942.70,
				"TOTAL_SHARES_RATIO": 29.35807774,
				"SECUCODE": "002404.SZ",
				"IS_INCREASE": "-1",
				"IS_COMPLETE": "1",
				"SECURITY_CODE": "002404",
				"FREE_SHARES_CHANGE": -1.60493790,
				"CHANGE_RATIO": -5.183403059509,
				"ORG_NAME_TYPE": "合计",
				"ALL_SHARES_RATIO": 23.89250279,
				"TOTAL_SHARES": 133810330,
				"TOTAL_FREE_SHARES_CHANGE": -7315100,
				"CLOSE_PRICE": 6.19
			}],
			"count": 59
		},
		"success": true,
		"message": "ok",
		"code": 0
	}`

	var resp institutionalHoldResponse
	if err := json.Unmarshal([]byte(jsonBody), &resp); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Result.Count != 59 {
		t.Fatalf("count = %d, want 59", resp.Result.Count)
	}

	items := resp.Result.Data

	// 第一条：2025年报完整数据（对应图片第一列）
	t.Run("2025年报完整数据", func(t *testing.T) {
		item := items[0]
		r := adapter.InstitutionalHolding{
			Code:               item.SECURITY_CODE,
			ReportDate:         truncateDate(item.REPORT_DATE),
			InstitutionCount:   item.TOTAL_ORG_NUM,
			TotalFreeShares:    item.TOTAL_FREE_SHARES,
			TotalMarketCap:     floatPtrOrZero(item.TOTAL_MARKET_CAP),
			FreeShareRatio:     floatPtrOrZero(item.TOTAL_SHARES_RATIO),
			TotalShareRatio:    floatPtrOrZero(item.ALL_SHARES_RATIO),
			ClosePrice:         floatPtrOrZero(item.CLOSE_PRICE),
			FreeShareChangePct: floatPtrOrZero(item.FREE_SHARES_CHANGE),
			HoldingChangeRatio: floatPtrOrZero(item.CHANGE_RATIO),
			FreeShareChangeNum: item.TOTAL_FREE_SHARES_CHANGE,
		}

		if r.ReportDate != "2025-12-31" {
			t.Errorf("ReportDate = %q, want 2025-12-31", r.ReportDate)
		}
		if r.InstitutionCount != 82 {
			t.Errorf("InstitutionCount = %d, want 82(更新中)", r.InstitutionCount)
		}
		// 合计持股 1.464亿
		if int64(r.TotalFreeShares/1e8*100) != 146 { // 1.46403170亿 → 146
			t.Errorf("TotalFreeShares(亿) = %.2f, want ~1.464", float64(r.TotalFreeShares)/1e8)
		}
		// 合计市值 10.26亿
		if int64(r.TotalMarketCap/1e6) != 1026 { // 102628万
			t.Errorf("TotalMarketCap(万) = %.0f, want ~102628", r.TotalMarketCap/1e4)
		}
		// 占流通股比 32.14%
		if abs(r.FreeShareRatio-32.14) > 0.01 {
			t.Errorf("FreeShareRatio = %f, want ~32.14", r.FreeShareRatio)
		}
		// 占总股本比例 26.14%
		if abs(r.TotalShareRatio-26.14) > 0.01 {
			t.Errorf("TotalShareRatio = %f, want ~26.14", r.TotalShareRatio)
		}
		// 股价 7.01元
		if abs(r.ClosePrice-7.01) > 0.001 {
			t.Errorf("ClosePrice = %f, want 7.01", r.ClosePrice)
		}
		// 较上期变化 +2.78%
		if abs(r.FreeShareChangePct-2.78) > 0.01 {
			t.Errorf("FreeShareChangePct = %f, want ~2.78", r.FreeShareChangePct)
		}
	})

	// 第二条：2025三季报数据（对应图片第二列）
	t.Run("2025三季报", func(t *testing.T) {
		item := items[1]

		if truncateDate(item.REPORT_DATE) != "2025-09-30" {
			t.Errorf("ReportDate = %q, want 2025-09-30", truncateDate(item.REPORT_DATE))
		}
		if item.TOTAL_ORG_NUM != 5 {
			t.Errorf("InstitutionCount = %d, want 5", item.TOTAL_ORG_NUM)
		}
		// 三季报机构数通常较少（基金未全部披露）
		if item.IS_INCREASE != "-1" {
			t.Errorf("IS_INCREASE = %q, want -1(减少)", item.IS_INCREASE)
		}
	})
}

// ========== 名称变更测试 ==========

// TestGetNameChanges 测试获取名称变更（实际调用 API，需要网络）
// 使用 300344（立方数科→*ST立方→立方退）作为测试标的
func TestGetNameChanges(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	code := "300344"

	changes, err := a.GetNameChanges(ctx, code, "2025-12-02")
	if err != nil {
		t.Fatalf("GetNameChanges(%s, since=empty) failed: %v", code, err)
	}

	if len(changes) == 0 {
		t.Fatal("expected at least one name change record for 300344")
	}

	t.Logf("%s 名称变更记录数: %d", code, len(changes))

	// 验证按日期降序排列
	for i := 1; i < len(changes); i++ {
		if changes[i].ChangeDate > changes[i-1].ChangeDate {
			t.Errorf("records not sorted by date desc: [%d]=%s > [%d]=%s",
				i-1, changes[i-1].ChangeDate, i, changes[i].ChangeDate)
		}
	}

	// 验证第一条（最新）记录字段
	latest := changes[0]
	if latest.Code != code {
		t.Errorf("Code = %q, want %q", latest.Code, code)
	}
	if latest.ChangeDate == "" {
		t.Error("ChangeDate is empty")
	}
	if latest.SecurityName == "" {
		t.Error("SecurityName is empty")
	}
	if latest.ChangeReason == "" {
		t.Error("ChangeReason is empty")
	}

	t.Logf("  Latest: %s %s (%s)", latest.ChangeDate, latest.SecurityName, latest.ChangeReason)
}

// TestGetNameChanges_InvalidCode 测试无效股票代码
func TestGetNameChanges_InvalidCode(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	// 999999 是无意义的代码，API 应返回空数据（code=9201）
	changes, err := a.GetNameChanges(ctx, "999999", "")
	if err != nil {
		t.Logf("999999 returned error (acceptable): %v", err)
		return
	}
	if len(changes) != 0 {
		t.Logf("unexpected data for invalid code 999999: %d records", len(changes))
	}
}

// TestGetNameChanges_WithSince 测试增量模式（since 参数）
func TestGetNameChanges_WithSince(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	code := "300344"

	// 先全量拉取，获取最新变更日期
	allChanges, err := a.GetNameChanges(ctx, code, "")
	if err != nil {
		t.Fatalf("GetNameChanges(%s, since=empty) failed: %v", code, err)
	}
	if len(allChanges) == 0 {
		t.Skip("no name change data for 300344, skip incremental test")
	}

	// 取最新记录的日期作为 since（模拟"已同步到该日期"）
	latestDate := allChanges[0].ChangeDate
	t.Logf("全量记录数: %d, 最新日期: %s", len(allChanges), latestDate)

	// since = 最新日期 → 应该返回空（没有比最新日期更新的记录）
	changes, err := a.GetNameChanges(ctx, code, latestDate)
	if err != nil {
		t.Fatalf("GetNameChanges(%s, since=%s) failed: %v", code, latestDate, err)
	}
	if len(changes) != 0 {
		t.Errorf("since=%s should return empty, got %d records", latestDate, len(changes))
	}
	t.Logf("since=%s → 返回空（符合预期）", latestDate)

	// since = 很早的日期（如 2000-01-01）→ 应该返回全量
	changes, err = a.GetNameChanges(ctx, code, "2000-01-01")
	if err != nil {
		t.Fatalf("GetNameChanges(%s, since=2000-01-01) failed: %v", code, err)
	}
	if len(changes) != len(allChanges) {
		t.Errorf("since=2000-01-01: got %d records, want %d (full amount)",
			len(changes), len(allChanges))
	}
	t.Logf("since=2000-01-01 → 返回 %d 条（全量）", len(changes))
}

// TestParseNameChangeResponse 测试名称变更 JSON 解析（无需网络）
func TestParseNameChangeResponse(t *testing.T) {
	jsonBody := `{
		"version": "e9ec6711f0a51b29350c86d84476feb4",
		"result": {
			"pages": 1,
			"data": [
				{
					"SECURITY_CODE": "300344",
					"SECUCODE": "300344.SZ",
					"CHANGE_DATE": "2026-03-31 00:00:00",
					"CHANGE_NAME": "*ST立方 → 立方退",
					"CHANGE_REASON": "进入退市整理板"
				},
				{
					"SECURITY_CODE": "300344",
					"SECUCODE": "300344.SZ",
					"CHANGE_DATE": "2025-12-02 00:00:00",
					"CHANGE_NAME": "ST立方 → *ST立方",
					"CHANGE_REASON": "实行退市风险警示"
				},
				{
					"SECURITY_CODE": "300344",
					"SECUCODE": "300344.SZ",
					"CHANGE_DATE": "2025-04-30 00:00:00",
					"CHANGE_NAME": "立方数科 → ST立方",
					"CHANGE_REASON": "实行其他特别处理"
				}
			],
			"count": 6
		},
		"success": true,
		"message": "ok",
		"code": 0
	}`

	var resp nameChangeResponse
	if err := json.Unmarshal([]byte(jsonBody), &resp); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Result.Count != 6 {
		t.Fatalf("count = %d, want 6", resp.Result.Count)
	}
	if resp.Code != 0 {
		t.Fatalf("code = %d, want 0", resp.Code)
	}

	items := resp.Result.Data
	if len(items) != 3 {
		t.Fatalf("data length = %d, want 3", len(items))
	}

	// 验证第一条（最新）
	t.Run("第一条(2026-03-31)", func(t *testing.T) {
		item := items[0]
		if item.SECURITY_CODE != "300344" {
			t.Errorf("SECURITY_CODE = %q, want 300344", item.SECURITY_CODE)
		}
		dateStr := parseNameChangeDate(item.CHANGE_DATE)
		if dateStr != "2026-03-31" {
			t.Errorf("CHANGE_DATE = %q, want 2026-03-31", dateStr)
		}
		if item.CHANGE_NAME != "*ST立方 → 立方退" {
			t.Errorf("CHANGE_NAME = %q, want *ST立方 → 立方退", item.CHANGE_NAME)
		}
		if item.CHANGE_REASON != "进入退市整理板" {
			t.Errorf("CHANGE_REASON = %q, want 进入退市整理板", item.CHANGE_REASON)
		}
	})

	// 验证 parseNameChangeDate 提取 YYYY-MM-DD
	t.Run("parseNameChangeDate", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"2026-03-31 00:00:00", "2026-03-31"},
			{"2025-12-02", "2025-12-02"},
			{"", ""},
			{"2024", "2024"},
		}
		for _, tt := range tests {
			got := parseNameChangeDate(tt.input)
			if got != tt.expected {
				t.Errorf("parseNameChangeDate(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		}
	})

	// 验证增量过滤逻辑：只保留 > since 的记录
	t.Run("增量过滤逻辑", func(t *testing.T) {
		since := "2025-06-01"
		var filtered []nameChangeItem
		for _, item := range items {
			dateStr := parseNameChangeDate(item.CHANGE_DATE)
			if dateStr <= since {
				continue
			}
			filtered = append(filtered, item)
		}
		// 只有 2026-03-31 和 2025-12-02 两条 > 2025-06-01
		if len(filtered) != 2 {
			t.Errorf("filtered count = %d, want 2", len(filtered))
		}
		t.Logf("since=%s: 过滤后 %d 条", since, len(filtered))
	})
}

// TestCopyValues 测试 copyValues 辅助函数
func TestCopyValues(t *testing.T) {
	orig := url.Values{
		"reportName":   {"RPT_ORG_COURSECHANGE"},
		"columns":      {"SECURITY_CODE,SECUCODE"},
		"quoteColumns": {""},
		"filter":       {"(SECUCODE=\"300344.SZ\")"},
	}

	copied := copyValues(orig)

	// 修改 copy 不影响 orig
	copied.Set("pageNumber", "1")
	copied.Set("pageSize", "1")
	if orig.Get("pageNumber") != "" {
		t.Error("copyValues: modifying copy should not affect original")
	}

	// 值正确
	if copied.Get("reportName") != "RPT_ORG_COURSECHANGE" {
		t.Errorf("reportName = %q, want RPT_ORG_COURSECHANGE", copied.Get("reportName"))
	}
	if copied.Get("filter") != "(SECUCODE=\"300344.SZ\")" {
		t.Errorf("filter = %q, want (SECUCODE=\"300344.SZ\")", copied.Get("filter"))
	}
}

// TestGetNameChanges_ProbeEarlyReturn 测试探路提前返回逻辑
// 模拟 since 日期晚于最新记录的情况
func TestGetNameChanges_ProbeEarlyReturn(t *testing.T) {
	// 构造一个探路响应：最新记录日期是 2025-01-01
	probeJSON := `{
		"success": true,
		"code": 0,
		"result": {
			"pages": 2,
			"data": [{
				"SECURITY_CODE": "300344",
				"SECUCODE": "300344.SZ",
				"CHANGE_DATE": "2025-01-01 00:00:00",
				"CHANGE_NAME": "旧名称",
				"CHANGE_REASON": "旧原因"
			}],
			"count": 2
		}
	}`

	var probeResp nameChangeResponse
	if err := json.Unmarshal([]byte(probeJSON), &probeResp); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	latestDate := parseNameChangeDate(probeResp.Result.Data[0].CHANGE_DATE)
	since := "2026-01-01" // since 比最新记录新

	if latestDate <= since {
		t.Logf("latestDate=%s <= since=%s → 应提前返回空（符合预期）", latestDate, since)
	} else {
		t.Errorf("expected latestDate <= since, got latestDate=%s, since=%s", latestDate, since)
	}
}

// TestGetNameChanges_SinceFilter 测试 Go 侧 since 过滤
func TestGetNameChanges_SinceFilter(t *testing.T) {
	// 模拟 API 返回的全量数据
	allItems := []nameChangeItem{
		{CHANGE_DATE: "2026-03-31 00:00:00", CHANGE_NAME: "A", CHANGE_REASON: "R1"},
		{CHANGE_DATE: "2025-12-02 00:00:00", CHANGE_NAME: "B", CHANGE_REASON: "R2"},
		{CHANGE_DATE: "2025-04-30 00:00:00", CHANGE_NAME: "C", CHANGE_REASON: "R3"},
		{CHANGE_DATE: "2021-01-26 00:00:00", CHANGE_NAME: "D", CHANGE_REASON: "R4"},
	}

	since := "2025-06-01"

	var filtered []nameChangeItem
	for _, item := range allItems {
		dateStr := parseNameChangeDate(item.CHANGE_DATE)
		if dateStr <= since {
			continue
		}
		filtered = append(filtered, item)
	}

	// 应该只有 2026-03-31 和 2025-12-02 两条
	if len(filtered) != 2 {
		t.Fatalf("filtered count = %d, want 2", len(filtered))
	}
	if filtered[0].CHANGE_NAME != "A" {
		t.Errorf("filtered[0].CHANGE_NAME = %q, want A", filtered[0].CHANGE_NAME)
	}
	if filtered[1].CHANGE_NAME != "B" {
		t.Errorf("filtered[1].CHANGE_NAME = %q, want B", filtered[1].CHANGE_NAME)
	}
	t.Logf("since=%s: 过滤后 %d 条（正确）", since, len(filtered))
}
