package eastmoney

import (
	"testing"
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
			t.Logf("  [%d] %s(%s) 市场:%d 行业:%s 地区:%s",
				i+1, item.F14, item.F12,
				item.F13, item.F116, item.F117,
			)
		}
	}
}
