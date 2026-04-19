package eastmoney

import (
	"context"
	"encoding/json"
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
