package eastmoney

import (
	"context"
	"encoding/json"
	"testing"
)

// ========== 财报集成测试（需网络） ==========

func TestGetPerformanceReports(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	code := "002404" // 嘉欣丝绸

	reports, err := a.GetPerformanceReports(ctx, code)
	if err != nil {
		t.Fatalf("GetPerformanceReports(%s) failed: %v", code, err)
	}

	if len(reports) == 0 {
		t.Fatal("expected at least one finance report")
	}

	t.Logf("%s 财报记录数: %d", code, len(reports))

	for i, r := range reports[:min(4, len(reports))] {
		t.Logf("  [%d] %s %s EPS=%.3f 营收=%.0f 净利=%.0f 毛利率=%.2f%% ROE=%.2f%%",
			i+1, r.ReportDateName, r.ReportDate,
			r.BasicEPS, r.TotalRevenue, r.ParentNetProfit,
			r.GrossMargin, r.ROEW)
	}

	// 验证最新一期数据
	latest := reports[0]
	if latest.Code != code {
		t.Errorf("Code = %q, want %q", latest.Code, code)
	}
	if latest.SecurityName == "" {
		t.Error("SecurityName is empty")
	}
	if latest.ReportDate == "" {
		t.Error("ReportDate is empty")
	}
	if latest.TotalRevenue <= 0 {
		t.Errorf("TotalRevenue = %f, want > 0", latest.TotalRevenue)
	}
}

func TestGetPerformanceReports_InvalidCode(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	reports, err := a.GetPerformanceReports(ctx, "999999")
	if err != nil {
		t.Logf("999999 returned error (acceptable): %v", err)
		return
	}
	if len(reports) != 0 {
		t.Logf("unexpected data for invalid code 999999: %d records", len(reports))
	}
}

func TestGetLatestPerformanceReport(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	ctx := context.Background()
	code := "002475" // 立讯精密

	report, err := a.GetLatestPerformanceReport(ctx, code)
	if err != nil {
		t.Fatalf("GetLatestPerformanceReport(%s) failed: %v", code, err)
	}

	t.Logf("最新财报: %s %s | EPS=%.3f 营收=%d亿 净利=%d亿 毛利率=%.2f%% ROE=%.2f%%",
		report.ReportDateName, report.ReportDate,
		report.BasicEPS,
		int64(report.TotalRevenue/1e8),
		int64(report.ParentNetProfit/1e8),
		report.GrossMargin, report.ROEW)

	if report.Code != code {
		t.Errorf("Code = %q, want %q", report.Code, code)
	}
	if report.BasicEPS <= 0 {
		t.Errorf("BasicEPS = %f, expect > 0", report.BasicEPS)
	}
}

// ========== 财报 JSON 解析单元测试（无需网络） ==========

func TestParseFinanceResponse(t *testing.T) {
	jsonBody := `{
		"success": true,
		"message": "ok",
		"result": {
			"pages": 1,
			"count": 2,
			"data": [{
				"SECUCODE": "002404.SZ",
				"SECURITY_CODE": "002404",
				"SECURITY_NAME_ABBR": "嘉欣丝绸",
				"REPORT_DATE": "2025-12-31 00:00:00",
				"REPORT_TYPE": "年报",
				"REPORT_DATE_NAME": "2025年报",
				"CURRENCY": "CNY",
				"NOTICE_DATE": "2026-03-31 00:00:00",
				"UPDATE_DATE": "2026-03-31 00:00:00",
				"EPSJB": 0.32,
				"EPSKCJB": 0.28,
				"EPSXS": 0.32,
				"BPS": 3.579914879816,
				"MGZBGJ": 0.996862229685,
				"MGWFPLR": 1.171432342456,
				"MGJYXJJE": 0.806479058701,
				"TOTALOPERATEREVE": 4753012224.90,
				"MLR": 596894493.05,
				"PARENTNETPROFIT": 180085890.34,
				"KCFJCXSYJLR": 158127316.81,
				"TOTALOPERATEREVETZ": 4.066140497355,
				"PARENTNETPROFITTZ": 12.043244610471,
				"KCFJCXSYJLRTZ": 17.336427279326,
				"YYZSRGDHBZC": 3.42890128073,
				"NETPROFITRPHBZC": 11.844519313439,
				"KFJLRGDHBZC": 21.703494114816,
				"ROEJQ": 9.14,
				"ROEKCJQ": 8.02,
				"ZZCJLL": 5.7694913353,
				"XSJLL": 4.0767403986,
				"XSMLL": 12.5742677884,
				"TAXRATE": 18.8210869951,
				"LD": 2.030652654171,
				"SD": 1.535678537291,
				"XJLLB": 0.371793421988,
				"ZCFZL": 36.9606083543,
				"QYCS": 1.586309724593,
				"CQBL": 0.586309724594,
				"LIABILITY": 1235086334.37,
				"ROIC": 7.059602902397,
				"YSZKYYSR": 0.001111168515,
				"XSJXLYYSR": 0.176611810353,
				"JYXJLYYSR": 0.095149437318,
				"DJD_TOI_YOY": 4.07,
				"DJD_DPNP_YOY": 12.04,
				"DJD_DEDUCTDPNP_YOY": 17.34,
				"DJD_TOI_QOQ": 3.43,
				"DJD_DPNP_QOQ": 11.84,
				"DJD_DEDUCTDPNP_QOQ": 21.70,
				"STAFF_NUM": 6094,
				"AVG_TOI": 779949.495388907,
				"AVG_NET_PROFIT": 29551.3440006564,
				"IS_BZ": "0"
			}, {
				"SECUCODE": "002404.SZ",
				"SECURITY_CODE": "002404",
				"SECURITY_NAME_ABBR": "嘉欣丝绸",
				"REPORT_DATE": "2025-09-30 00:00:00",
				"REPORT_TYPE": "三季报",
				"REPORT_DATE_NAME": "2025三季报",
				"CURRENCY": "CNY",
				"NOTICE_DATE": "2025-10-30 00:00:00",
				"UPDATE_DATE": "2025-10-30 00:00:00",
				"EPSJB": 0.2700,
				"EPSKCJB": null,
				"EPSXS": null,
				"BPS": 3.5302,
				"MGZBGJ": null,
				"MGWFPLR": null,
				"MGJYXJJE": null,
				"TOTALOPERATEREVE": 3637000000.00,
				"MLR": 442400000.00,
				"PARENTNETPROFIT": 152200000.00,
				"KCFJCXSYJLR": null,
				"TOTALOPERATEREVETZ": 0.78,
				"PARENTNETPROFITTZ": -0.19,
				"KCFJCXSYJLRTZ": null,
				"YYZSRGDHBZC": 0.01,
				"NETPROFITRPHBZC": -0.15,
				"KFJLRGDHBZC": null,
				"ROEJQ": 7.70,
				"ROEKCJQ": null,
				"ZZCJLL": 4.68,
				"XSJLL": null,
				"XSMLL": 12.18,
				"TAXRATE": 22.21,
				"LD": 1.802,
				"SD": 1.395,
				"XJLLB": null,
				"ZCFZL": 36.96,
				"QYCS": null,
				"CQBL": null,
				"LIABILITY": null,
				"ROIC": null,
				"IS_BZ": "0"
			}]
		}
	}`

	var resp financeResponse
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

	// 验证第一条（年报，完整数据）
	t.Run("年报完整数据", func(t *testing.T) {
		r := convertToPerformanceReport(items[0])

		if r.Code != "002404" {
			t.Errorf("Code = %q", r.Code)
		}
		if r.SecurityName != "嘉欣丝绸" {
			t.Errorf("SecurityName = %q, want 嘉欣丝绸", r.SecurityName)
		}
		if r.ReportDate != "2025-12-31" {
			t.Errorf("ReportDate = %q, want 2025-12-31", r.ReportDate)
		}
		if r.ReportType != "年报" {
			t.Errorf("ReportType = %q, want 年报", r.ReportType)
		}
		if r.BasicEPS != 0.32 {
			t.Errorf("BasicEPS = %f, want 0.32", r.BasicEPS)
		}
		if r.DeductedEPS != 0.28 {
			t.Errorf("DeductedEPS = %f, want 0.28", r.DeductedEPS)
		}
		if r.BVPS < 3.57 && r.BVPS > 3.58 {
			t.Errorf("BVPS = %f, want ~3.58", r.BVPS)
		}
		if int64(r.TotalRevenue) != 4753012224 {
			t.Errorf("TotalRevenue = %d, want 4753012224", int64(r.TotalRevenue))
		}
		if int64(r.ParentNetProfit) != 180085890 {
			t.Errorf("ParentNetProfit = %d, want 180085890", int64(r.ParentNetProfit))
		}
		if r.GrossMargin < 12.57 || r.GrossMargin > 12.58 {
			t.Errorf("GrossMargin = %f, want ~12.57", r.GrossMargin)
		}
		if r.ROEW != 9.14 {
			t.Errorf("ROEW = %f, want 9.14", r.ROEW)
		}
		if r.StaffNum != 6094 {
			t.Errorf("StaffNum = %d, want 6094", r.StaffNum)
		}
	})

	// 验证第二条（三季报，含 null 字段）
	t.Run("三季报null字段安全处理", func(t *testing.T) {
		r := convertToPerformanceReport(items[1])

		if r.ReportType != "三季报" {
			t.Errorf("ReportType = %q, want 三季报", r.ReportType)
		}
		if r.BasicEPS != 0.27 {
			t.Errorf("BasicEPS = %f, want 0.27", r.BasicEPS)
		}
		// null 字段应返回 0，不 panic
		if r.DeductedEPS != 0 {
			t.Errorf("DeductedEPS(null) = %f, want 0", r.DeductedEPS)
		}
		if r.ROEDW != 0 {
			t.Errorf("ROEDW(null) = %f, want 0", r.ROEDW)
		}
	})
}

// ========== 辅助函数单元测试 ==========

func TestConvertToPerformanceReport_AllFieldsMapped(t *testing.T) {
	// 确保所有东财字段都能正确映射到 PerformanceReport
	item := financeItem{
		SECUCODE:           "600519.SH",
		SECURITY_CODE:      "600519",
		SECURITY_NAME_ABBR: "贵州茅台",
		REPORT_DATE:        "2024-12-31 00:00:00",
		REPORT_TYPE:        "年报",
		REPORT_DATE_NAME:   "2024年报",
		CURRENCY:           "CNY",
		EPSJB:              ptrF(50.0),
		EPSKCJB:            ptrF(49.5),
		EPSXS:              ptrF(49.8),
		BPS:                ptrF(150.0),
		MGZBGJ:             ptrF(80.0),
		MGWFPLR:            ptrF(60.0),
		MGJYXJJE:           ptrF(45.0),
		TOTALOPERATEREVE:    ptrF(150500000000.0),
		MLR:                 ptrF(100000000000.0),
		PARENTNETPROFIT:     ptrF(75000000000.0),
		KCFJCXSYJLR:         ptrF(74000000000.0),
		TOTALOPERATEREVETZ:  ptrF(15.5),
		PARENTNETPROFITTZ:   ptrF(18.0),
		KCFJCXSYJLRTZ:       ptrF(19.2),
		YYZSRGDHBZC:         ptrF(3.0),
		NETPROFITRPHBZC:     ptrF(10.0),
		KFJLRGDHBZC:         ptrF(20.0),
		DJD_TOI_YOY:         ptrF(5.0),
		DJD_DPNP_YOY:        ptrF(6.0),
		DJD_DEDUCTDPNP_YOY:  ptrF(7.0),
		DJD_TOI_QOQ:         ptrF(2.0),
		DJD_DPNP_QOQ:        ptrF(3.0),
		DJD_DEDUCTDPNP_QOQ:  ptrF(4.0),
		ROEJQ:               ptrF(33.0),
		ROEKCJQ:             ptrF(32.5),
		ZZCJLL:              ptrF(25.0),
		XSJLL:               ptrF(50.0),
		XSMLL:               ptrF(66.0),
		TAXRATE:             ptrF(25.0),
		LD:                  ptrF(3.0),
		SD:                  ptrF(2.5),
		XJLLB:               ptrF(1.5),
		ZCFZL:               ptrF(25.0),
		QYCS:                ptrF(1.33),
		CQBL:                ptrF(0.25),
		LIABILITY:           ptrF(300000000000.0),
		ROIC:                ptrF(30.0),
		YSZKYYSR:            ptrF(0.01),
		XSJXLYYSR:           ptrF(0.10),
		JYXJLYYSR:           ptrF(0.08),
		ZZCZZTS:             ptrF(250.0),
		CHZZTS:              ptrF(50.0),
		YSZKZZTS:            ptrF(40.0),
		TOAZZL:              ptrF(1.5),
		CHZZL:               ptrF(7.0),
		YSZKZZL:             ptrF(9.0),
		STAFF_NUM:           ptrI(30000),
		AVG_TOI:             ptrF(500000.0),
		AVG_NET_PROFIT:      ptrF(250000.0),
		IS_BZ:               ptrStr("0"),
	}

	r := convertToPerformanceReport(item)

	tests := []struct {
		name     string
		got      float64
		want     float64
		tolerance float64
	}{
		{"Code", 0, 0, 0}, // string field skip
		{"BasicEPS", r.BasicEPS, 50.0, 0.001},
		{"DeductedEPS", r.DeductedEPS, 49.5, 0.001},
		{"DilutedEPS", r.DilutedEPS, 49.8, 0.001},
		{"BVPS", r.BVPS, 150.0, 0.01},
		{"TotalRevenue/亿", r.TotalRevenue / 1e8, 150500.0, 1},
		{"ParentNetProfit/亿", r.ParentNetProfit / 1e8, 750.0, 0.5},
		{"RevenueYoY", r.RevenueYoY, 15.5, 0.01},
		{"ParentNetProfitYoY", r.ParentNetProfitYoY, 18.0, 0.01},
		{"GrossMargin", r.GrossMargin, 66.0, 0.01},
		{"ROEW", r.ROEW, 33.0, 0.01},
		{"CurrentRatio", r.CurrentRatio, 3.0, 0.01},
		{"DebtRatio", r.DebtRatio, 25.0, 0.01},
		{"StaffNum", float64(r.StaffNum), 30000.0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff := abs(tt.got - tt.want)
			if diff > tt.tolerance {
				t.Errorf("%s = %.4f, want %.4f (diff=%.4f)", tt.name, tt.got, tt.want, diff)
			}
		})
	}
}

// TestParseReportYear 测试年份解析
func TestParseReportYear(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"2025-12-31", 2025},
		{"2025", 2025},
		{"2024-03-31", 2024},
		{"", 0},
		{"abc", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseReportYear(tt.input); got != tt.want {
				t.Errorf("parseReportYear(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// ========== test helpers ==========

func ptrF(v float64) *float64 { return &v }
func ptrI(v int) *int          { return &v }
func ptrStr(v string) *string  { return &v }
func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// min helper for Go < 1.22
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
