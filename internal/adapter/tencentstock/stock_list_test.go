package tencentstock

import (
	"context"
	"testing"
)

func TestGetStockList(t *testing.T) {
	a := newTestAdapter(t)

	list, err := a.GetStockList(context.Background())
	if err != nil {
		t.Fatalf("GetStockList 失败: %v", err)
	}
	if len(list) < 100 {
		t.Fatalf("股票列表太少: %d 只", len(list))
	}
	t.Logf("股票列表: 共 %d 只", len(list))
}

func TestGetStockDetail(t *testing.T) {
	a := newTestAdapter(t)
	detail, err := a.GetStockDetail(context.Background(), "002404")
	if err != nil {
		t.Fatalf("GetStockDetail 失败: %v", err)
	}
	if detail == nil {
		t.Fatal("股票详情为空")
	}
	t.Logf("股票详情: 代码=%s 交易所=%s", detail.Code, detail.Exchange)
}
