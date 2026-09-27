package technical

import (
	"strings"
	"testing"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/model"
)

// vsTestConfig 构造信号配置（内置信号 01006003）。
func vsTestConfig(params map[string]any) *indicator.SignalConfig {
	return &indicator.SignalConfig{SignalID: "01006003", Operator: indicator.OpCustom, Params: params}
}

// buildVSKlines 构造"放量上涨后连续跌停缩量"形态（返回倒序 klines）。
//
// 时间升序（价格单位：分，成交量单位：股）：
//
//	3 日填充(900 / 50000)
//	→ A-4(1000 / 100000)
//	→ A-3(1050 / 100000，+5.00%)
//	→ A-2(1100 / 100000，+4.76%)
//	→ A-1(990 / 10000，-10.00%)
//	→ A (891 / 10000，-10.00%)
//	→ tailDays 日填充(891 / 8000)
//
// v1 = 10000、v2 = 100000 → v2/v1 = 10 > 7；两段量差均为 0%。
// tailDays=0 时 A 日即今日（倒序索引 0），tailDays=3 时 A 日距今 3 个交易日。
func buildVSKlines(tailDays int, mods ...func(k []*model.DailyKline)) []*model.DailyKline {
	oldest := make([]*model.DailyKline, 0, 8+tailDays)
	date := 20240101
	appendDay := func(open, high, low, close int, volume int64) {
		oldest = append(oldest, &model.DailyKline{TradeDate: date, Open: open, High: high, Low: low, Close: close, Volume: volume})
		date++
	}
	appendDay(900, 910, 890, 900, 50000) // 前置填充
	appendDay(900, 910, 890, 900, 50000)
	appendDay(900, 910, 890, 900, 50000)
	appendDay(1000, 1010, 990, 1000, 100000)  // A-4
	appendDay(1000, 1060, 1000, 1050, 100000) // A-3
	appendDay(1050, 1110, 1040, 1100, 100000) // A-2
	appendDay(1100, 1110, 985, 990, 10000)    // A-1
	appendDay(990, 995, 885, 891, 10000)      // A
	for i := 0; i < tailDays; i++ {
		appendDay(891, 900, 880, 891, 8000)
	}

	klines := make([]*model.DailyKline, 0, len(oldest))
	for i := len(oldest) - 1; i >= 0; i-- {
		klines = append(klines, oldest[i])
	}
	for _, m := range mods {
		if m != nil {
			m(klines)
		}
	}
	return klines
}

func TestSignalVolumeShrinkLimitDown_Registration(t *testing.T) {
	p := NewPattern()
	sig, ok := p.Signal["01006003"]
	if !ok {
		t.Fatal("内置信号 01006003 (连续跌停缩量) 未注册")
	}
	if sig.Name() != "连续跌停缩量" {
		t.Errorf("Name=%q, want 连续跌停缩量", sig.Name())
	}
	cfg := sig.DefaultConfig()
	if cfg == nil || cfg.Operator != indicator.OpCustom {
		t.Fatalf("DefaultConfig=%+v", cfg)
	}
	wantDefaults := map[string]float64{
		paramVSAgo:         0,
		paramVSDropMin:     9.9,
		paramVSVolDiffMax:  30,
		paramVSRiseMin:     0,
		paramVSVolRatioMin: 7,
	}
	for key, want := range wantDefaults {
		if got := cfg.GetFloat64(key, -1); got != want {
			t.Errorf("默认参数 %s=%v, want %v", key, got, want)
		}
	}
}

func TestSignalVolumeShrinkLimitDown_Evaluate(t *testing.T) {
	tests := []struct {
		name     string
		klines   []*model.DailyKline
		params   map[string]any
		wantPass bool
		wantMsg  string
	}{
		{
			name:     "标准命中（A=今日，跌幅10%、v2/v1=10）",
			klines:   buildVSKlines(0, nil),
			wantPass: true,
		},
		{
			name: "A日跌幅不足",
			klines: buildVSKlines(0, func(k []*model.DailyKline) {
				k[0].Close = 900 // (900-990)/990 = -9.09%
			}),
			wantPass: false,
			wantMsg:  "A日",
		},
		{
			name: "A-1跌幅不足",
			klines: buildVSKlines(0, func(k []*model.DailyKline) {
				k[1].Close = 1000 // (1000-1100)/1100 = -9.09%
			}),
			wantPass: false,
			wantMsg:  "A-1日",
		},
		{
			name: "跌停段量差过大",
			klines: buildVSKlines(0, func(k []*model.DailyKline) {
				k[0].Volume = 20000 // (20000-10000)/10000 = 100%
			}),
			wantPass: false,
			wantMsg:  "跌停段量差",
		},
		{
			// A-2 涨幅归零（与 A-3 同价），同步调整 A-1/A 收盘保证跌停段仍达标
			name: "A-2涨幅为0（不大于0%）",
			klines: buildVSKlines(0, func(k []*model.DailyKline) {
				k[2].Close = 1050
				k[1].Close = 940 // (940-1050)/1050 = -10.48%
				k[0].Close = 845 // (845-940)/940 = -10.11%
			}),
			wantPass: false,
			wantMsg:  "A-2日",
		},
		{
			name: "A-3涨幅为0（不大于0%）",
			klines: buildVSKlines(0, func(k []*model.DailyKline) {
				k[3].Close = 1000 // (1000-1000)/1000 = 0%
			}),
			wantPass: false,
			wantMsg:  "A-3日",
		},
		{
			name: "上涨段量差过大",
			klines: buildVSKlines(0, func(k []*model.DailyKline) {
				k[2].Volume = 200000 // (200000-100000)/100000 = 100%
			}),
			wantPass: false,
			wantMsg:  "上涨段量差",
		},
		{
			name: "量比 v2/v1 不足",
			klines: buildVSKlines(0, func(k []*model.DailyKline) {
				k[0].Volume = 20000
				k[1].Volume = 20000 // v1=20000 → v2/v1=5 < 7
			}),
			wantPass: false,
			wantMsg:  "量比",
		},
		{
			name:     "K线数据不足（需5根）",
			klines:   buildVSKlines(0, nil)[:4],
			wantPass: false,
			wantMsg:  "K线数据不足",
		},
		{
			name: "跌停段成交量为0",
			klines: buildVSKlines(0, func(k []*model.DailyKline) {
				k[0].Volume = 0
			}),
			wantPass: false,
			wantMsg:  "跌停段成交量为 0",
		},
		{
			name:     "A日距今3个交易日命中",
			klines:   buildVSKlines(3, nil),
			params:   map[string]any{paramVSAgo: float64(3)},
			wantPass: true,
		},
		{
			name:     "形态在3天前但A日取今日时不命中",
			klines:   buildVSKlines(3, nil),
			wantPass: false,
			wantMsg:  "A日",
		},
		{
			name:     "放宽跌幅下限后命中（跌幅10% → 下限8%）",
			klines:   buildVSKlines(0, nil),
			params:   map[string]any{paramVSDropMin: float64(8)},
			wantPass: true,
		},
		{
			name: "量差上限放宽到150%后命中",
			klines: buildVSKlines(0, func(k []*model.DailyKline) {
				k[0].Volume = 20000 // 量差 100%
			}),
			params:   map[string]any{paramVSVolDiffMax: float64(150), paramVSVolRatioMin: float64(6)},
			wantPass: true,
		},
	}

	s := NewSignalVolumeShrinkLimitDown()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := s.Evaluate(tt.klines, vsTestConfig(tt.params))
			if tt.wantPass {
				if res.Result != indicator.ResultPassed {
					t.Fatalf("want passed, got %v: %s", res.Result, res.Message)
				}
				return
			}
			if res.Result != indicator.ResultRejected {
				t.Fatalf("want rejected, got %v: %s", res.Result, res.Message)
			}
			if tt.wantMsg != "" && !strings.Contains(res.Message, tt.wantMsg) {
				t.Errorf("Message=%q, want contains %q", res.Message, tt.wantMsg)
			}
		})
	}
}

// TestSignalVolumeShrinkLimitDown_PatternDispatch 验证 Pattern.Evaluate 能分发到该信号。
func TestSignalVolumeShrinkLimitDown_PatternDispatch(t *testing.T) {
	p := NewPattern()
	res := p.Evaluate(&vsTestStock{klines: buildVSKlines(0, nil)}, []*indicator.SignalConfig{
		{SignalID: "01006003", Operator: indicator.OpCustom},
	})
	if res.Result != indicator.ResultPassed {
		t.Fatalf("want passed, got %v: %s", res.Result, res.Message)
	}
}

// vsTestStock 仅供分发测试使用的 StockSource 实现（仅 GetDailyKline 返回真实数据）。
type vsTestStock struct {
	klines []*model.DailyKline
}

func (s *vsTestStock) GetCode() string { return "000001" }
func (s *vsTestStock) GetName() string { return "测试" }

func (s *vsTestStock) GetDailyKline() ([]*model.DailyKline, error)     { return s.klines, nil }
func (s *vsTestStock) GetWeeklyKline() ([]*model.WeeklyKline, error)   { return nil, nil }
func (s *vsTestStock) GetMonthlyKline() ([]*model.MonthlyKline, error) { return nil, nil }
func (s *vsTestStock) GetYearlyKline() ([]*model.YearlyKline, error)   { return nil, nil }
func (s *vsTestStock) GetDailySnapshot() (*model.StockDailySnapshot, error) {
	return nil, nil
}
func (s *vsTestStock) GetPerformanceReport() ([]*model.PerformanceReport, error) { return nil, nil }
func (s *vsTestStock) GetShareholderCount() (*model.ShareholderCount, error)     { return nil, nil }
func (s *vsTestStock) GetDetail() (*model.Stock, error)                          { return nil, nil }

// BenchmarkSignalVolumeShrinkLimitDown 基准测试：形态判定为 O(1)，验证无隐藏的窗口扫描开销。
func BenchmarkSignalVolumeShrinkLimitDown(b *testing.B) {
	klines := buildVSKlines(0, nil)
	s := NewSignalVolumeShrinkLimitDown()
	cfg := vsTestConfig(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Evaluate(klines, cfg)
	}
}
