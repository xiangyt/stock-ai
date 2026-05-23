package tencentstock

import (
	"context"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// newRateTestAdapter 创建带自定义限速器的适配器
func newRateTestAdapter(t *testing.T, rps float64, burst int) *Adapter {
	t.Helper()
	a := New()
	a.limiter = rate.NewLimiter(rate.Limit(rps), burst)
	if err := a.Init(nil); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	return a
}

// ========== 紧限速：3rps burst=1，10次请求应受明显限速 ==========

func TestGetTodayData_RateLimitingTight(t *testing.T) {
	// 3rps, burst=1 → 10次请求中 9次要等令牌，理论耗时 ≥ 9/3 = 3s
	a := newRateTestAdapter(t, 3, 1)

	codes := []string{
		"000001", "000002", "000638", "002404", "600519",
		"600036", "000333", "002415", "600900", "000858",
	}

	ctx := context.Background()
	start := time.Now()
	ok, fail := 0, 0

	for _, code := range codes {
		_, err := a.GetTodayData(ctx, code)
		if err != nil {
			fail++
			t.Logf("  [FAIL] %s: %v", code, err)
		} else {
			ok++
		}
	}

	elapsed := time.Since(start)
	t.Logf("紧限速结果: %d ok / %d fail, 总耗时 %v (3rps burst=1)", ok, fail, elapsed)

	// 限速器在 doGet 入口消耗 token，即使 API 返回错误也已限速
	expectedMin := 2500 * time.Millisecond // 9次排队 / 3rps = 3s，宽松到 2.5s
	if elapsed < expectedMin && len(codes) > 1 {
		t.Errorf("耗时过短 %v < %v，限速器可能未生效", elapsed, expectedMin)
	}
}

// ========== 大 burst：5次 < burst=10，应瞬间完成 ==========

func TestGetTodayData_RateLimitingBurstSmallBatch(t *testing.T) {
	a := newRateTestAdapter(t, 2, 10)

	codes := []string{"000001", "600519", "002404", "000638", "000333"}

	ctx := context.Background()
	start := time.Now()
	ok, fail := 0, 0

	for _, code := range codes {
		_, err := a.GetTodayData(ctx, code)
		if err != nil {
			fail++
		} else {
			ok++
		}
	}

	elapsed := time.Since(start)
	t.Logf("大Burst结果: %d ok / %d fail, 总耗时 %v (2rps burst=10)", ok, fail, elapsed)
}

// ========== 默认限速器：10rps burst=50 ==========

func TestGetTodayData_RateLimitingDefault(t *testing.T) {
	a := newTestAdapter(t)

	codes := []string{"000001", "600519", "002404", "000638", "000333"}

	ctx := context.Background()
	start := time.Now()
	ok, fail := 0, 0

	for _, code := range codes {
		_, err := a.GetTodayData(ctx, code)
		if err != nil {
			fail++
		} else {
			ok++
		}
	}

	elapsed := time.Since(start)
	t.Logf("默认限速器: %d ok / %d fail, 总耗时 %v (10rps burst=50)", ok, fail, elapsed)
}

// ========== 超 burst 触发排队：5rps burst=3，发8次 ==========

func TestGetTodayData_RateLimitingExceedBurst(t *testing.T) {
	a := newRateTestAdapter(t, 5, 3)

	codes := []string{
		"000001", "000002", "000638", "002404",
		"600519", "600036", "000333", "002415",
	}

	ctx := context.Background()
	start := time.Now()
	ok, fail := 0, 0

	for _, code := range codes {
		_, err := a.GetTodayData(ctx, code)
		if err != nil {
			fail++
		} else {
			ok++
		}
	}

	elapsed := time.Since(start)
	afterBurst := len(codes) - 3 // burst=3
	expectedMin := time.Duration(afterBurst) * (time.Second / 5)

	t.Logf("超Burst结果: %d ok / %d fail, 总耗时 %v (5rps burst=3, 超%d次预期≥%v)",
		ok, fail, elapsed, afterBurst, expectedMin)

	minRequired := expectedMin / 2 // 宽松 50%
	if elapsed < minRequired && len(codes) > 1 {
		t.Errorf("耗时过短 %v < %v，限速器可能未生效", elapsed, minRequired)
	}
}

// ========== 纯限速器测试：验证 limiter.Wait 本身 ==========

func TestRateLimiter_WaitTiming(t *testing.T) {
	a := newRateTestAdapter(t, 5, 1)

	ctx := context.Background()
	n := 6
	start := time.Now()

	for i := 0; i < n; i++ {
		if err := a.limiter.Wait(ctx); err != nil {
			t.Fatalf("limiter.Wait 失败: %v", err)
		}
	}

	elapsed := time.Since(start)
	t.Logf("纯限速器: %d 次 Wait, 耗时 %v (5rps burst=1, 预期≥%v)", n, elapsed, time.Duration(n-1)*(time.Second/5))

	if elapsed < 800*time.Millisecond {
		t.Errorf("纯限速器耗时过短 %v < 800ms", elapsed)
	}
}
