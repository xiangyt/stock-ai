package tencentstock

import (
	"context"
	"testing"
)

func TestGetShareholderCounts(t *testing.T) {
	a := newTestAdapter(t)
	counts, err := a.GetShareholderCounts(context.Background(), "002404")
	if err != nil {
		t.Fatalf("GetShareholderCounts 失败: %v", err)
	}
	if len(counts) == 0 {
		t.Fatal("股东户数列表为空")
	}
	t.Logf("股东户数: 共 %d 条，最新截止日=%s 户数=%d",
		len(counts), counts[0].EndDate, counts[0].HolderNum)
}

func TestGetLatestShareholderCount(t *testing.T) {
	a := newTestAdapter(t)
	count, err := a.GetLatestShareholderCount(context.Background(), "002404")
	if err != nil {
		t.Fatalf("GetLatestShareholderCount 失败: %v", err)
	}
	if count == nil {
		t.Fatal("最新股东户数为空")
	}
	t.Logf("最新股东户数: 截止日=%s 户数=%d", count.EndDate, count.HolderNum)
}

func TestGetShareChanges(t *testing.T) {
	a := newTestAdapter(t)
	changes, err := a.GetShareChanges(context.Background(), "002404")
	if err != nil {
		t.Fatalf("GetShareChanges 失败: %v", err)
	}
	if len(changes) == 0 {
		t.Fatal("股本变动列表为空")
	}
	t.Logf("股本变动: 共 %d 条", len(changes))
}

func TestGetInstitutionalHoldings(t *testing.T) {
	a := newTestAdapter(t)
	holdings, err := a.GetInstitutionalHoldings(context.Background(), "002404")
	if err != nil {
		t.Fatalf("GetInstitutionalHoldings 失败: %v", err)
	}
	if len(holdings) == 0 {
		t.Fatal("机构持仓列表为空")
	}
	t.Logf("机构持仓: 共 %d 条", len(holdings))
}
