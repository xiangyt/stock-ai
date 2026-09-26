package technical

import (
	"math/rand"
	"testing"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/model"
)

// 测试构造（时间升序：收集期 20 → 放量段 7 → 之后 3 日；倒序后 A 日索引 = 9）
const (
	chipTestCollectDays = 20
	chipTestUpDays      = 7
	chipTestTailDays    = 3 // 放量段之后（更接近今日）的交易日数
	chipTestTotalDays   = chipTestCollectDays + chipTestUpDays + chipTestTailDays
	chipTestAIdx        = chipTestUpDays - 1 + chipTestTailDays // A 日倒序索引 = 9
	chipTestEndIdx      = chipTestTailDays                      // A+up_days-1 倒序索引 = 3
)

// chipTestKline 构造单根测试K线（价格单位：分，成交量单位：股）。
func chipTestKline(date, open, high, low, close int, volume int64) *model.DailyKline {
	return &model.DailyKline{TradeDate: date, Open: open, High: high, Low: low, Close: close, Volume: volume}
}

// chipTestConfig 构造信号配置。
func chipTestConfig(params map[string]any) *indicator.SignalConfig {
	return &indicator.SignalConfig{SignalID: "01006002", Operator: indicator.OpCustom, Params: params}
}

// buildChipKlines 构造一段标准形态（返回倒序 klines）：
//
//	收集期20日（量 1000/1010/1020 循环、价 1000~1002 窄幅）
//	→ A 日（量 12000、价 1120，涨幅 11.9%）
//	→ 放量段7日（量 12000+1000i、价 1120+60i）
//	→ 之后3日（量 5000，判定不覆盖）
func buildChipKlines(mods ...func(k []*model.DailyKline)) []*model.DailyKline {
	oldest := make([]*model.DailyKline, 0, chipTestTotalDays)
	date := 20240101
	appendDay := func(open, high, low, close int, volume int64) {
		oldest = append(oldest, chipTestKline(date, open, high, low, close, volume))
		date++
	}
	for i := 0; i < chipTestCollectDays; i++ {
		c, v := 1000+i%3, int64(1000+(i%3)*10)
		appendDay(c, c+10, c-10, c, v)
	}
	// 放量段涨幅刻意压到 5% 以下（仅 A 日大涨），保证窗口内只有 A 日是有效候选
	for i := 0; i < chipTestUpDays; i++ {
		c, v := 1120+20*i, int64(12000+1000*i)
		appendDay(c-20, c+10, c-30, c, v)
	}
	for i := 0; i < chipTestTailDays; i++ {
		c := 1240 - i*10
		appendDay(c-10, c+10, c-20, c, 5000)
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

// chipCollectIdx 收集期升序第 i 日（0=最早）在倒序序列中的索引。
func chipCollectIdx(i int) int { return chipTestTotalDays - 1 - i }

func TestSignalChipAccumulation_Registration(t *testing.T) {
	p := NewPattern()
	sig, ok := p.Signal["01006002"]
	if !ok {
		t.Fatal("内置信号 01006002 (筹码收集后放量启动) 未注册")
	}
	if sig.Name() != "筹码收集后放量启动" {
		t.Errorf("Name=%q, want 筹码收集后放量启动", sig.Name())
	}
	cfg := sig.DefaultConfig()
	if cfg == nil || cfg.Operator != indicator.OpCustom {
		t.Fatalf("DefaultConfig=%+v", cfg)
	}
	wantDefaults := map[string]float64{
		indicator.ParamKeyDays: 40,
		paramChipMinAgo:        7,
		paramChipCollectDays:   20,
		paramChipTrim:          3,
		paramChipMaxFlatSlope:  1.5,
		paramChipMaxDisp:       0.30,
		paramChipUpDays:        7,
		paramChipBreakVolMult:  2.0,
		paramChipBreakRise:     5,
		paramChipFastRise:      50,
		paramChipMinSlope:      0.5,
		paramChipPeakMult:      7,
		paramChipConfirmRise:   15,
	}
	for key, want := range wantDefaults {
		if got := cfg.GetFloat64(key, -1); got != want {
			t.Errorf("默认参数 %s=%v, want %v", key, got, want)
		}
	}
}

func TestSignalChipAccumulation_Evaluate(t *testing.T) {
	tests := []struct {
		name     string
		klines   []*model.DailyKline
		params   map[string]any
		wantPass bool
		wantMsg  string
	}{
		{
			name:     "标准命中（A距今9日、收集期20日、放量段7日）",
			klines:   buildChipKlines(nil),
			wantPass: true,
		},
		{
			name: "A日涨幅不足",
			klines: buildChipKlines(func(k []*model.DailyKline) {
				for j := chipTestEndIdx; j <= chipTestAIdx; j++ { // 整段压平，避免其它候选因 A 收盘下移而"被动大涨"
					k[j].Close = 1010
				}
			}),
			wantPass: false,
			wantMsg:  "A日涨幅",
		},
		{
			name: "A日量能不足",
			klines: buildChipKlines(func(k []*model.DailyKline) {
				k[chipTestAIdx].Volume = 1500 // 平台均量约 1010，需 >= 2.0×
			}),
			wantPass: false,
			wantMsg:  "A日量",
		},
		{
			name: "放量段末端涨幅不足",
			klines: buildChipKlines(func(k []*model.DailyKline) {
				for j := chipTestEndIdx; j <= chipTestAIdx; j++ {
					k[j].Close = 1120
				}
			}),
			wantPass: false,
			wantMsg:  "相对 A-1 收盘",
		},
		{
			name: "收集期非水平（量能持续抬升）",
			klines: buildChipKlines(func(k []*model.DailyKline) {
				for i := 0; i < chipTestCollectDays; i++ {
					k[chipCollectIdx(i)].Volume = int64(1000 + i*300)
				}
				for j := chipTestEndIdx; j <= chipTestAIdx; j++ {
					k[j].Volume = 60000
				}
			}),
			wantPass: false,
			wantMsg:  "收集期斜率",
		},
		{
			// 交替量能同时抬高斜率与离散度，此处放宽斜率上限以单独验证离散度
			name: "收集期离散度过大",
			klines: buildChipKlines(func(k []*model.DailyKline) {
				for i := 0; i < chipTestCollectDays; i++ {
					if i%2 == 0 {
						k[chipCollectIdx(i)].Volume = 500
					} else {
						k[chipCollectIdx(i)].Volume = 1500
					}
				}
			}),
			params:   map[string]any{paramChipMaxFlatSlope: float64(50)},
			wantPass: false,
			wantMsg:  "收集期离散度",
		},
		{
			name: "放量段峰值不足",
			klines: buildChipKlines(func(k []*model.DailyKline) {
				for j := chipTestEndIdx; j <= chipTestAIdx; j++ {
					k[j].Volume = int64(4000 + (chipTestAIdx-j)*200)
				}
			}),
			wantPass: false,
			wantMsg:  "峰值",
		},
		{
			// 只有 A 日单独放天量、之后回到平台量级：MA10 不再抬升，斜率≈0
			name: "放量段MA10斜率不足（一日游放量）",
			klines: buildChipKlines(func(k []*model.DailyKline) {
				for j := chipTestEndIdx; j < chipTestAIdx; j++ {
					k[j].Volume = 1000
				}
			}),
			wantPass: false,
			wantMsg:  "放量段MA10斜率",
		},
		{
			// A 距今 9 个交易日，低于下限 15（回看窗口会按收集期长度自动放宽，故用下限验证）
			name:     "启动日距今不足下限",
			klines:   buildChipKlines(nil),
			params:   map[string]any{paramChipMinAgo: float64(15)},
			wantPass: false,
			wantMsg:  "启动日距今15~40",
		},
		{
			name:     "K线数据不足",
			klines:   buildChipKlines(nil)[:15],
			wantPass: false,
			wantMsg:  "K线数据不足",
		},
	}

	sig := NewSignalChipAccumulation()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := sig.Evaluate(tt.klines, chipTestConfig(tt.params))
			if tt.wantPass {
				if res.Result != indicator.ResultPassed {
					t.Fatalf("应命中，实际 %v：%s", res.Result, res.Message)
				}
				return
			}
			if res.Result != indicator.ResultRejected {
				t.Fatalf("应拒绝，实际 %v：%s", res.Result, res.Message)
			}
			if tt.wantMsg != "" && !contains(res.Message, tt.wantMsg) {
				t.Errorf("Message=%q, want 包含 %q", res.Message, tt.wantMsg)
			}
		})
	}
}

// TestCheckChipAccum_FastRise 验证强势直通：末端涨幅达到 fast_rise 时直接命中，
// 跳过收集期平整度/离散度与放量段量能校验（这些都刻意构造为不合格）。
func TestCheckChipAccum_FastRise(t *testing.T) {
	// 收集期剧烈震荡（离散度必然超标）+ 放量段量能压平（MA10 斜率≈0、峰值不足）
	mk := func(endClose int) []*model.DailyKline {
		return buildChipKlines(func(k []*model.DailyKline) {
			for i := 0; i < chipTestCollectDays; i++ {
				if i%2 == 0 {
					k[chipCollectIdx(i)].Volume = 200
				} else {
					k[chipCollectIdx(i)].Volume = 3000
				}
			}
			for j := chipTestEndIdx; j < chipTestAIdx; j++ { // A 日之后量能压平（不含 A 日）
				k[j].Volume = 6000
			}
			k[chipTestEndIdx].Close = endClose // 只抬高 A+6 收盘，制造末端涨幅
		})
	}

	p := parseChipParams(chipTestConfig(nil))
	for _, c := range []struct {
		name     string
		endClose int
		want     bool
	}{
		{"末端涨幅50.05%→直通命中", 1502, true}, // Close(A-1)=1001
		{"末端涨幅20.08%→不直通", 1202, false}, // 走完整校验，被离散度/峰值/斜率判死
	} {
		klines := mk(c.endClose)
		ok, _, msg := checkChipAccumAt(klines, chipVolumePrefix(klines), chipTestAIdx, p)
		if ok != c.want {
			t.Errorf("%s: ok=%v want %v, msg=%q", c.name, ok, c.want, msg)
		}
	}

	// fast_rise=0 关闭直通：同一形态应回到完整校验并被拒绝
	off := parseChipParams(chipTestConfig(map[string]any{paramChipFastRise: float64(0)}))
	klines := mk(1502)
	if ok, _, msg := checkChipAccumAt(klines, chipVolumePrefix(klines), chipTestAIdx, off); ok {
		t.Errorf("fast_rise=0 应关闭直通，仍命中：%s", msg)
	}
}

func TestParseChipParams_Clamp(t *testing.T) {
	p := parseChipParams(chipTestConfig(map[string]any{
		paramChipCollectDays: float64(5),
		paramChipTrim:        float64(10),
		paramChipUpDays:      float64(1),
	}))
	if p.CollectDays != chipMinCollectDays {
		t.Errorf("CollectDays=%d, want %d", p.CollectDays, chipMinCollectDays)
	}
	if p.TrimCount != 3 { // (10-3)/2
		t.Errorf("TrimCount=%d, want 3", p.TrimCount)
	}
	if p.UpDays != chipMinUpDays {
		t.Errorf("UpDays=%d, want %d", p.UpDays, chipMinUpDays)
	}
}

// TestChipTrimExtremes 验证去极值保留的是中间样本，且 x 为原始时间序号。
func TestChipTrimExtremes(t *testing.T) {
	ys := []float64{5, 1, 9, 3, 7, 2, 8}
	xs, kept := chipTrimExtremes(ys, 1) // 去掉 1 和 9
	if len(kept) != 5 {
		t.Fatalf("kept=%v, want 5 个", kept)
	}
	for _, v := range kept {
		if v == 1 || v == 9 {
			t.Errorf("极值未被剔除：%v", kept)
		}
	}
	// x 必须是原始序号且升序
	for i := 1; i < len(xs); i++ {
		if xs[i] <= xs[i-1] {
			t.Errorf("x 非升序：%v", xs)
		}
	}
}

// TestChipLinearFit 验证最小二乘拟合：y = 2x + 1。
func TestChipLinearFit(t *testing.T) {
	xs := []float64{0, 1, 2, 3, 4}
	ys := []float64{1, 3, 5, 7, 9}
	a, b := linearFitFloat(xs, ys)
	if mathAbs(a-2) > 1e-9 || mathAbs(b-1) > 1e-9 {
		t.Errorf("fit: a=%v b=%v, want a=2 b=1", a, b)
	}
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// buildChipRandomKlines 生成 n 根倒序随机K线。
func buildChipRandomKlines(n int) []*model.DailyKline {
	r := rand.New(rand.NewSource(42))
	klines := make([]*model.DailyKline, 0, n)
	for i := 0; i < n; i++ {
		c := 1000 + r.Intn(500)
		klines = append(klines, chipTestKline(20240101+i, c-10, c+20, c-20, c, int64(1000+r.Intn(20000))))
	}
	for i, j := 0, len(klines)-1; i < j; i, j = i+1, j-1 { // 转倒序
		klines[i], klines[j] = klines[j], klines[i]
	}
	return klines
}

func BenchmarkSignalChipAccumulation_Evaluate(b *testing.B) {
	klines := buildChipRandomKlines(250)
	sig := NewSignalChipAccumulation()
	cfg := chipTestConfig(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sig.Evaluate(klines, cfg)
	}
}
