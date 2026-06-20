package quotecache

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================================
//  Mock Collector
// ============================================================================

type mockCollector struct {
	data map[string]*MinuteData
	err  error

	callCount int32
}

func (m *mockCollector) GetIntraday(ctx context.Context, code string) (*MinuteData, error) {
	atomic.AddInt32(&m.callCount, 1)
	if m.err != nil {
		return nil, m.err
	}
	return m.data[code], nil
}

func makeMinuteData(date string, preClose int64, bars ...MinuteBar) *MinuteData {
	return &MinuteData{
		Date:     date,
		PreClose: preClose,
		Bars:     bars,
	}
}

func makeBar(time string, price, volume, amount int64) MinuteBar {
	return MinuteBar{Time: time, Price: price, Volume: volume, Amount: amount}
}

func newTestCache(collector IntradayCollector) QuoteCache {
	if coll, ok := collector.(*mockCollector); ok {
		return New(Config{Collector: CollectorChain{coll}})
	}
	return New(Config{Collector: CollectorChain{collector}})
}

// ============================================================================
//  数据存取
// ============================================================================

func TestGet_CacheHit(t *testing.T) {
	collector := &mockCollector{data: map[string]*MinuteData{
		"000001": makeMinuteData("2026-06-20", 1000,
			makeBar("09:30", 1010, 1000, 1010000),
			makeBar("09:35", 1020, 2000, 2030000),
		),
	}}
	cache := newTestCache(collector)

	data, err := cache.Get(context.Background(), "000001")
	if err != nil {
		t.Fatalf("Get() err: %v", err)
	}
	if data.Code != "000001" {
		t.Errorf("Code = %s, want 000001", data.Code)
	}

	// 2nd call → cache hit
	_, err = cache.Get(context.Background(), "000001")
	if err != nil {
		t.Fatalf("2nd Get() err: %v", err)
	}
	if count := atomic.LoadInt32(&collector.callCount); count != 1 {
		t.Errorf("collector called %d times, want 1", count)
	}
}

func TestGet_CollectorError(t *testing.T) {
	collector := &mockCollector{err: context.DeadlineExceeded}
	cache := newTestCache(collector)

	_, err := cache.Get(context.Background(), "000001")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ============================================================================
//  日线计算
// ============================================================================

func TestDaily_ComputeFromBars(t *testing.T) {
	md := makeMinuteData("2026-06-20", 1000,
		makeBar("09:30", 1010, 100, 101000),
		makeBar("09:35", 1020, 200, 203000),
		makeBar("09:40", 990, 300, 297000),
	)

	cd := &CachedQuoteData{Code: "000001", Intraday: md}
	daily := cd.Daily()

	if daily == nil {
		t.Fatal("Daily() returned nil")
	}
	if daily.Open != 1010 {
		t.Errorf("Open = %d, want 1010", daily.Open)
	}
	if daily.Close != 990 {
		t.Errorf("Close = %d, want 990", daily.Close)
	}
	if daily.High != 1020 {
		t.Errorf("High = %d, want 1020", daily.High)
	}
	if daily.Low != 990 {
		t.Errorf("Low = %d, want 990", daily.Low)
	}
	if daily.Volume != 300 {
		t.Errorf("Volume = %d, want 300", daily.Volume)
	}
	if daily.Amount != 297000 {
		t.Errorf("Amount = %d, want 297000", daily.Amount)
	}
	if daily.Change != -10 {
		t.Errorf("Change = %d, want -10", daily.Change)
	}
	if daily.ChangePct != -1.0 {
		t.Errorf("ChangePct = %f, want -1.0", daily.ChangePct)
	}
}

func TestDaily_CachedAfterCompute(t *testing.T) {
	md := makeMinuteData("2026-06-20", 1000,
		makeBar("09:30", 1010, 100, 101000),
	)
	cd := &CachedQuoteData{Code: "000001", Intraday: md}

	d1 := cd.Daily()
	d2 := cd.Daily()

	if d1 != d2 {
		t.Error("Daily() returned different pointers on 2nd call (cache not working)")
	}
}

func TestDaily_Invalidate(t *testing.T) {
	md := makeMinuteData("2026-06-20", 1000,
		makeBar("09:30", 1010, 100, 101000),
	)
	cd := &CachedQuoteData{Code: "000001", Intraday: md}

	d1 := cd.Daily()
	cd.Invalidate()
	d2 := cd.Daily()

	if d1 == d2 {
		t.Error("Daily() returned same pointer after Invalidate")
	}
}

func TestDaily_NilIntraday(t *testing.T) {
	cd := &CachedQuoteData{Code: "000001"}
	if cd.Daily() != nil {
		t.Error("Daily() with nil Intraday should return nil")
	}
}

// ============================================================================
//  订阅推送
// ============================================================================

func TestSubscribe_SingleCode_PushOnRefresh(t *testing.T) {
	collector := &mockCollector{data: map[string]*MinuteData{
		"000001": makeMinuteData("2026-06-20", 1000,
			makeBar("09:30", 1010, 100, 101000),
		),
	}}
	cache := newTestCache(collector)

	ch, cancel := cache.Subscribe([]string{"000001"})
	defer cancel()

	// Trigger refresh
	_, err := cache.Get(context.Background(), "000001")
	if err != nil {
		t.Fatalf("Get() err: %v", err)
	}

	select {
	case evt := <-ch:
		if evt.Code != "000001" {
			t.Errorf("Code = %s, want 000001", evt.Code)
		}
		if evt.Data == nil {
			t.Fatal("Data is nil")
		}
		if evt.Data.Price != 10.10 {
			t.Errorf("Price = %f, want 10.10", evt.Data.Price)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for QuoteEvent")
	}
}

func TestSubscribe_OnlyReceivesSubscribed(t *testing.T) {
	collector := &mockCollector{data: map[string]*MinuteData{
		"000001": makeMinuteData("2026-06-20", 1000, makeBar("09:30", 1010, 100, 101000)),
		"000002": makeMinuteData("2026-06-20", 500, makeBar("09:30", 510, 200, 102000)),
	}}
	cache := newTestCache(collector)

	ch, cancel := cache.Subscribe([]string{"000001"})
	defer cancel()

	// Refresh 000002 only
	_, err := cache.Get(context.Background(), "000002")
	if err != nil {
		t.Fatalf("Get() err: %v", err)
	}

	select {
	case evt := <-ch:
		t.Errorf("received unexpected event for code %s", evt.Code)
	case <-time.After(100 * time.Millisecond):
		// Expected: no event
	}
}

func TestSubscribe_Unsubscribe_ChannelClosed(t *testing.T) {
	cache := New(Config{Collector: CollectorChain{&mockCollector{}}})
	ch, cancel := cache.Subscribe([]string{"000001"})
	cancel()

	_, ok := <-ch
	if ok {
		t.Error("channel should be closed after unsubscribe")
	}
}

func TestSubscribe_AfterUnsubscribe_NoPanic(t *testing.T) {
	collector := &mockCollector{data: map[string]*MinuteData{
		"000001": makeMinuteData("2026-06-20", 1000, makeBar("09:30", 1010, 100, 101000)),
	}}
	cache := newTestCache(collector)

	ch, cancel := cache.Subscribe([]string{"000001"})
	cancel()
	// Drain closed channel
	for range ch {
	}

	// Should not panic
	_, err := cache.Get(context.Background(), "000001")
	if err != nil {
		t.Fatalf("Get() after unsubscribe err: %v", err)
	}
}

// ============================================================================
//  反向索引
// ============================================================================

func TestCodeToSubs_Cleanup(t *testing.T) {
	cache := New(Config{Collector: CollectorChain{&mockCollector{}}}).(*quoteCacheImpl)

	_, cancel := cache.Subscribe([]string{"000001", "000002"})
	cancel()

	cache.subMu.RLock()
	defer cache.subMu.RUnlock()

	if _, ok := cache.codeToSubs["000001"]; ok {
		t.Error("codeToSubs should not have 000001 after unsubscribe")
	}
	if _, ok := cache.codeToSubs["000002"]; ok {
		t.Error("codeToSubs should not have 000002 after unsubscribe")
	}
}

// ============================================================================
//  优先级管理
// ============================================================================

func TestSetPriority(t *testing.T) {
	cache := New(Config{Collector: CollectorChain{&mockCollector{}}})

	cache.SetPriority([]string{"000001"}, PriorityHigh)
	cache.SetPriority([]string{"000002"}, PriorityLow)

	impl := cache.(*quoteCacheImpl)
	impl.mu.RLock()
	defer impl.mu.RUnlock()

	if impl.priority["000001"] != PriorityHigh {
		t.Error("000001 should be PriorityHigh")
	}
	if impl.priority["000002"] != PriorityLow {
		t.Error("000002 should be PriorityLow")
	}
}

// ============================================================================
//  生命周期
// ============================================================================

func TestStart_Stop(t *testing.T) {
	cache := New(Config{Collector: CollectorChain{&mockCollector{}}})
	cache.Start()
	cache.Stop()
	// Should not panic
}

// ============================================================================
//  OnQuoteReady hook
// ============================================================================

func TestOnQuoteReady_HookCalled(t *testing.T) {
	var called bool
	collector := &mockCollector{data: map[string]*MinuteData{
		"000001": makeMinuteData("2026-06-20", 1000, makeBar("09:30", 1010, 100, 101000)),
	}}
	cache := New(Config{
		Collector: CollectorChain{collector},
		OnQuoteReady: func(data *CachedQuoteData) {
			called = true
			data.Name = "测试股"
		},
	})

	data, err := cache.Get(context.Background(), "000001")
	if err != nil {
		t.Fatalf("Get() err: %v", err)
	}
	if !called {
		t.Error("OnQuoteReady not called")
	}
	if data.Name != "测试股" {
		t.Errorf("Name = %s, want 测试股", data.Name)
	}
}

// ============================================================================
//  Stats
// ============================================================================

func TestStats_HitMissCount(t *testing.T) {
	collector := &mockCollector{data: map[string]*MinuteData{
		"000001": makeMinuteData("2026-06-20", 1000, makeBar("09:30", 1010, 100, 101000)),
	}}
	cache := newTestCache(collector)

	cache.Get(context.Background(), "000001") // miss → fetch
	cache.Get(context.Background(), "000001") // hit

	stats := cache.Stats()
	if stats.HitCount != 1 {
		t.Errorf("HitCount = %d, want 1", stats.HitCount)
	}
	if stats.MissCount != 1 {
		t.Errorf("MissCount = %d, want 1", stats.MissCount)
	}
	if stats.Size != 1 {
		t.Errorf("Size = %d, want 1", stats.Size)
	}
}

// ============================================================================
//  Priority TTL
// ============================================================================

func TestPriority_TTL(t *testing.T) {
	if PriorityHigh.TTL() >= PriorityNormal.TTL() {
		t.Error("High TTL should be less than Normal TTL")
	}
	if PriorityNormal.TTL() >= PriorityLow.TTL() {
		t.Error("Normal TTL should be less than Low TTL")
	}
}
