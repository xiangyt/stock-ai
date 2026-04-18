package eastmoney

import (
	"testing"
)

// ========== 股票列表测试 ==========

func TestFetchStockListPage(t *testing.T) {
	a := newTestAdapter()
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
