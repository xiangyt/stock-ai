package tencentstock

import (
	"context"
	"testing"
)

func TestGetTodayData(t *testing.T) {
	a := newTestAdapter(t)

	snap, err := a.GetTodayData(context.Background(), "000638")
	if err != nil {
		t.Fatalf("GetTodayData 失败: %v", err)
	}
	if snap == nil || snap.Close == 0 {
		t.Fatal("快照数据为空或价格为零")
	}
	t.Logf("嘉欣丝绸快照: 日期=%s 收盘=%.2f 元 涨跌幅=%.2f%%",
		snap.Date, float64(snap.Close)/100, snap.ChangePct)
}

func TestGetThisWeekData(t *testing.T) {
	a := newTestAdapter(t)
	snap, err := a.GetThisWeekData(context.Background(), "002404")
	if err != nil {
		t.Fatalf("GetThisWeekData 失败: %v", err)
	}
	if snap == nil {
		t.Fatal("本周期数据为空")
	}
	t.Logf("本周期: 日期=%s 收盘=%.2f", snap.Date, float64(snap.Close)/100)
}

func TestGetThisMonthData(t *testing.T) {
	a := newTestAdapter(t)
	snap, err := a.GetThisMonthData(context.Background(), "002404")
	if err != nil {
		t.Fatalf("GetThisMonthData 失败: %v", err)
	}
	if snap == nil {
		t.Fatal("本月数据为空")
	}
	t.Logf("本月: 日期=%s 收盘=%.2f", snap.Date, float64(snap.Close)/100)
}

func TestGetThisQuarterData(t *testing.T) {
	a := newTestAdapter(t)
	snap, err := a.GetThisQuarterData(context.Background(), "002404")
	if err != nil {
		t.Fatalf("GetThisQuarterData 失败: %v", err)
	}
	if snap == nil {
		t.Fatal("本季数据为空")
	}
	t.Logf("本季: 日期=%s 收盘=%.2f", snap.Date, float64(snap.Close)/100)
}

func TestGetThisYearData(t *testing.T) {
	a := newTestAdapter(t)
	snap, err := a.GetThisYearData(context.Background(), "002404")
	if err != nil {
		t.Fatalf("GetThisYearData 失败: %v", err)
	}
	if snap == nil {
		t.Fatal("本年数据为空")
	}
	t.Logf("本年: 日期=%s 收盘=%.2f", snap.Date, float64(snap.Close)/100)
}
