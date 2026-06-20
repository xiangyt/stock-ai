package monitor

import (
	"fmt"
	"testing"

	"stock-ai/internal/model"
)

// ============================================================================
//  TestCheckDailyChange — 当日涨幅监控（6 档阈值）
// ============================================================================

func TestCheckDailyChange(t *testing.T) {
	checker := NewAlertChecker()

	tests := []struct {
		name      string
		params    string
		changePct float64
		wantLen   int
		wantLabel string
	}{
		{
			name:      "涨停触发",
			params:    `{"surge_big":9,"surge_small":5,"limit_up":9.8,"limit_down":-9.8,"drop_small":-5,"drop_big":-9,"surge_big_enabled":true,"surge_small_enabled":true,"limit_up_enabled":true,"limit_down_enabled":true,"drop_small_enabled":true,"drop_big_enabled":true}`,
			changePct: 10.0,
			wantLen:   1,
			wantLabel: "涨停",
		},
		{
			name:      "大涨触发",
			params:    `{"surge_big":9,"surge_small":5,"limit_up":9.8,"limit_down":-9.8,"drop_small":-5,"drop_big":-9,"surge_big_enabled":true,"surge_small_enabled":true,"limit_up_enabled":true,"limit_down_enabled":true,"drop_small_enabled":true,"drop_big_enabled":true}`,
			changePct: 9.5,
			wantLen:   1,
			wantLabel: "大涨",
		},
		{
			name:      "跌停触发",
			params:    `{"surge_big":9,"surge_small":5,"limit_up":9.8,"limit_down":-9.8,"drop_small":-5,"drop_big":-9,"surge_big_enabled":true,"surge_small_enabled":true,"limit_up_enabled":true,"limit_down_enabled":true,"drop_small_enabled":true,"drop_big_enabled":true}`,
			changePct: -10.0,
			wantLen:   1,
			wantLabel: "跌停",
		},
		{
			name:      "大跌触发",
			params:    `{"surge_big":9,"surge_small":5,"limit_up":9.8,"limit_down":-9.8,"drop_small":-5,"drop_big":-9,"surge_big_enabled":true,"surge_small_enabled":true,"limit_up_enabled":true,"limit_down_enabled":true,"drop_small_enabled":true,"drop_big_enabled":true}`,
			changePct: -9.5,
			wantLen:   1,
			wantLabel: "大跌",
		},
		{
			name:      "涨幅不足不触发",
			params:    `{"surge_big":9,"surge_small":5,"limit_up":9.8,"limit_down":-9.8,"drop_small":-5,"drop_big":-9,"surge_big_enabled":true,"surge_small_enabled":true,"limit_up_enabled":true,"limit_down_enabled":true,"drop_small_enabled":true,"drop_big_enabled":true}`,
			changePct: 2.0,
			wantLen:   0,
		},
		{
			name:      "小涨关闭不触发",
			params:    `{"surge_big":9,"surge_small":5,"limit_up":9.8,"limit_down":-9.8,"drop_small":-5,"drop_big":-9,"surge_big_enabled":false,"surge_small_enabled":false,"limit_up_enabled":false,"limit_down_enabled":false,"drop_small_enabled":false,"drop_big_enabled":false}`,
			changePct: 9.5,
			wantLen:   0,
		},
		{
			name:      "涨停优先级高于大涨",
			params:    `{"surge_big":9,"surge_small":5,"limit_up":9.8,"limit_down":-9.8,"drop_small":-5,"drop_big":-9,"surge_big_enabled":true,"surge_small_enabled":true,"limit_up_enabled":true,"limit_down_enabled":true,"drop_small_enabled":true,"drop_big_enabled":true}`,
			changePct: 9.9,
			wantLen:   1,
			wantLabel: "涨停",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := makeRule(model.RuleTypeDailyChange, tt.params)
			data := makeQuoteData("000001", tt.changePct, 10000000)
			alerts := checker.Check(rule, data)

			if len(alerts) != tt.wantLen {
				t.Errorf("告警数量 = %d, want %d", len(alerts), tt.wantLen)
				return
			}
			if tt.wantLen > 0 && alerts[0].Label != tt.wantLabel {
				t.Errorf("告警标签 = %q, want %q", alerts[0].Label, tt.wantLabel)
			}
		})
	}
}

// ============================================================================
//  TestCheckRapidMove — 急拉急跌监控
// ============================================================================

func TestCheckRapidMove(t *testing.T) {
	checker := NewAlertChecker()

	// makeBars 生成模拟分时数据：价格从 start 等差变化到 end，共 n 根 bar
	makeBars := func(n int, start, end float64) []model.MinuteBar {
		bars := make([]model.MinuteBar, n)
		step := (end - start) / float64(n-1)
		for i := 0; i < n; i++ {
			tm := 10*60 + 25 + i // 10:25 + i
			h, m := tm/60, tm%60
			bars[i] = model.MinuteBar{
				Time:  fmt.Sprintf("%02d:%02d", h, m),
				Price: start + step*float64(i),
			}
		}
		return bars
	}

	tests := []struct {
		name       string
		params     string
		minutes    []model.MinuteBar
		preClose   float64
		wantLen    int
		wantLabels []string
	}{
		{
			name:       "窗口内急拉→触发",
			params:     `{"minutes":5,"amplitude_pct":3,"up_enabled":true,"down_enabled":true}`,
			minutes:    makeBars(6, 10.00, 10.35), // 10.00→10.35, 相对昨收10.00: highPct=3.5%, lowPct=0%, amplitude=3.5%
			preClose:   10.00,
			wantLen:    1,
			wantLabels: []string{"急拉"},
		},
		{
			name:       "窗口内急跌→触发",
			params:     `{"minutes":5,"amplitude_pct":3,"up_enabled":true,"down_enabled":true}`,
			minutes:    makeBars(6, 10.00, 9.65), // 10.00→9.65, 相对昨收10.00: highPct=0%, lowPct=-3.5%, amplitude=3.5%
			preClose:   10.00,
			wantLen:    1,
			wantLabels: []string{"急跌"},
		},
		{
			name:       "仅启用上涨→跌不触发",
			params:     `{"minutes":5,"amplitude_pct":3,"up_enabled":true,"down_enabled":false}`,
			minutes:    makeBars(6, 10.00, 9.65),
			preClose:   10.00,
			wantLen:    0,
		},
		{
			name:       "仅启用下跌→涨不触发",
			params:     `{"minutes":5,"amplitude_pct":3,"up_enabled":false,"down_enabled":true}`,
			minutes:    makeBars(6, 10.00, 10.35),
			preClose:   10.00,
			wantLen:    0,
		},
		{
			name:       "幅度不足→不触发",
			params:     `{"minutes":5,"amplitude_pct":5,"up_enabled":true,"down_enabled":true}`,
			minutes:    makeBars(6, 10.00, 10.30), // amplitude=3.0% < 5.0%
			preClose:   10.00,
			wantLen:    0,
		},
		{
			name:       "方向全关→不触发",
			params:     `{"minutes":5,"amplitude_pct":3,"up_enabled":false,"down_enabled":false}`,
			minutes:    makeBars(6, 10.00, 10.35),
			preClose:   10.00,
			wantLen:    0,
		},
		{
			name:       "分时数据→窗口内急拉触发",
			params:     `{"minutes":5,"amplitude_pct":2,"up_enabled":true,"down_enabled":true}`,
			minutes: []model.MinuteBar{
				{Time: "10:25", Price: 10.00},
				{Time: "10:26", Price: 10.05},
				{Time: "10:27", Price: 10.10},
				{Time: "10:28", Price: 10.15},
				{Time: "10:29", Price: 10.20},
				{Time: "10:30", Price: 10.30},
			},
			preClose:   10.00,
			wantLen:    1,
			wantLabels: []string{"急拉"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := makeRule(model.RuleTypeRapidMove, tt.params)
			data := makeQuoteDataWithPreClose("000001", 0, tt.preClose, 10000000)
			data.Minutes = tt.minutes
			data.Price = tt.minutes[len(tt.minutes)-1].Price
			alerts := checker.Check(rule, data)

			if len(alerts) != tt.wantLen {
				t.Errorf("告警数量 = %d, want %d", len(alerts), tt.wantLen)
				return
			}
			for i, alert := range alerts {
				if i < len(tt.wantLabels) && alert.Label != tt.wantLabels[i] {
					t.Errorf("告警[%d] 标签 = %q, want %q", i, alert.Label, tt.wantLabels[i])
				}
			}
		})
	}
}

// ============================================================================
//  TestCheckVolumeRatio — 量比异动监控
// ============================================================================

func TestCheckVolumeRatio(t *testing.T) {
	t.Skip("volume_ratio 需要 DB 环境，跳过单元测试")
}

// ============================================================================
//  TestMultipleRules — 多规则分别检查
// ============================================================================

func TestMultipleRules(t *testing.T) {
	checker := NewAlertChecker()

	rule1 := makeRule(model.RuleTypeDailyChange, `{"surge_big":9,"surge_small":5,"limit_up":9.8,"limit_down":-9.8,"drop_small":-5,"drop_big":-9,"surge_big_enabled":true,"surge_small_enabled":true,"limit_up_enabled":true,"limit_down_enabled":true,"drop_small_enabled":true,"drop_big_enabled":true}`)
	rule2 := makeRule(model.RuleTypeRapidMove, `{"minutes":5,"amplitude_pct":3,"up_enabled":true,"down_enabled":true}`)

	data := makeQuoteData("000001", 10.0, 10000000)
	data.Minutes = []model.MinuteBar{
		{Time: "14:55", Price: 10.00},
		{Time: "14:56", Price: 10.10},
		{Time: "14:57", Price: 10.20},
		{Time: "14:58", Price: 10.30},
		{Time: "14:59", Price: 10.40},
		{Time: "15:00", Price: 10.50},
	}
	data.Price = 10.50

	alerts1 := checker.Check(rule1, data)
	alerts2 := checker.Check(rule2, data)

	t.Logf("daily_change 触发 %d 条告警:", len(alerts1))
	for _, a := range alerts1 {
		t.Logf("  - [%s] %s: %+.2f%%", a.RuleType, a.Label, a.ChangePct)
	}
	t.Logf("rapid_move 触发 %d 条告警:", len(alerts2))
	for _, a := range alerts2 {
		t.Logf("  - [%s] %s: amplitude=%.2f%%", a.RuleType, a.Label, a.ChangePct)
	}
	if len(alerts1) < 1 || len(alerts2) < 1 {
		t.Error("涨停应触发 daily_change 和 rapid_move 两条规则")
	}
}
