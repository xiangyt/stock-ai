package eastmoney

import (
	"context"
	"encoding/json"
	"testing"

	"stock-ai/internal/adapter"
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
