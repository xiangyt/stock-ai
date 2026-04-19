package ths

import "testing"

// ========== 辅助函数单元测试 ==========

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"10.5", 10.5},
		{"0", 0},
		{"1500.25", 1500.25},
		{"", 0},
		{"-", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseFloat(tt.input)
			if got != tt.want {
				t.Errorf("parseFloat(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseInt64(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"123456", 123456},
		{"0", 0},
		{"-1", -1},
		{"9876543210", int64(9876543210)},
		{"", 0},
		{"-", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseInt64(tt.input)
			if got != tt.want {
				t.Errorf("parseInt64(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatTradeDate(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{20260418, "2026-04-18"},
		{20251231, "2025-12-31"},
		{20240101, "2024-01-01"},
		{0, ""}, // 0 返回当天日期，不做精确匹配
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := formatTradeDate(tt.input)
			if tt.input != 0 && got != tt.want {
				t.Errorf("formatTradeDate(%d) = %q, want %q", tt.input, got, tt.want)
			}
			if tt.input == 0 && got == "" {
				t.Error("formatTradeDate(0) should return today's date")
			}
		})
	}
}

func TestYuanToCents(t *testing.T) {
	tests := []struct {
		input float64
		want  int64
	}{
		{10.50, 1050},
		{0, 0},
		{1.005, 1},     // 四舍五入
		{1.006, 1},     // 四舍五入
		{1500.55, 150056},
		{0.01, 1},
		{99.99, 9999},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := yuanToCents(tt.input)
			if got != tt.want {
				t.Errorf("yuanToCents(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractBetween(t *testing.T) {
	tests := []struct {
		s        string
		left     string
		right    string
		expected string
	}{
		{"<a>hello</a>", "<a>", "</a>", "hello"},
		{"stockCode\">000001</a>", "stockCode\">", "</a>", "000001"},
		{"no match here", "start", "end", ""},
		{"prefix_value_suffix", "prefix_", "_suffix", "value"},
	}

	for _, tt := range tests {
		t.Run(tt.s[:min(len(tt.s), 10)], func(t *testing.T) {
			got := extractBetween(tt.s, tt.left, tt.right)
			if got != tt.expected {
				t.Errorf("extractBetween(%q,%q,%q) = %q, want %q",
					tt.s, tt.left, tt.right, got, tt.expected)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	tests := []struct {
		m       map[string]interface{}
		key     string
		want    string
	}{
		{map[string]interface{}{"k": "val"}, "k", "val"},
		{map[string]interface{}{"k": float64(42)}, "k", "42"},
		{map[string]interface{}{"k": 42}, "k", "42"},
		{map[string]interface{}{"k": int64(42)}, "k", "42"},
		{map[string]interface{}{"other": "val"}, "k", ""},
		{map[string]interface{}{"k": nil}, "k", ""},
	}

	for i, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := getString(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("[%d] getString(%q) = %q, want %q", i, tt.key, got, tt.want)
			}
		})
	}
}

// min 辅助函数（Go < 1.21 兼容）
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
