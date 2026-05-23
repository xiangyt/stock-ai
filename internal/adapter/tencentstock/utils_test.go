package tencentstock

import (
	"testing"
)

func TestParseFloat(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"1290.20", 1290.20},
		{"-1.59", -1.59},
		{"", 0},
		{"-", 0},
		{"--", 0},
		{"null", 0},
		{"0", 0},
	}
	for _, c := range cases {
		got := parseFloat(c.input)
		if got != c.want {
			t.Errorf("parseFloat(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestParseInt(t *testing.T) {
	cases := []struct {
		input string
		want  int64
	}{
		{"49157", 49157},
		{"0", 0},
		{"", 0},
		{"-", 0},
		{"--", 0},
	}
	for _, c := range cases {
		got := parseInt(c.input)
		if got != c.want {
			t.Errorf("parseInt(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestToCents(t *testing.T) {
	cases := []struct {
		yuan float64
		want int64
	}{
		{0, 0},
		{1290.20, 129020},
		{1.59, 159},
		{0.01, 1},
		{0.005, 1}, // 四舍五入
	}
	for _, c := range cases {
		got := toCents(c.yuan)
		if got != c.want {
			t.Errorf("toCents(%v) = %v, want %v", c.yuan, got, c.want)
		}
	}
}

func TestTruncateDate(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"20260522", "2026-05-22"},
		{"2026-05-22", "2026-05-22"},
		{"20260522161417", "2026052216"}, // >=10 截前10位
	}
	for _, c := range cases {
		got := truncateDate(c.input)
		if got != c.want {
			t.Errorf("truncateDate(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestToTencentCode(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"600519", "sh600519"},
		{"000001", "sz000001"},
		{"300750", "sz300750"},
		{"688981", "sh688981"},
		{"sh600519", "sh600519"},
		{"SZ000001", "sz000001"},
	}
	for _, c := range cases {
		got := toTencentCode(c.input)
		if got != c.want {
			t.Errorf("toTencentCode(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestDetectPrefix(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"600519", "sh"},
		{"688981", "sh"},
		{"000001", "sz"},
		{"300750", "sz"},
		{"002404", "sz"},
	}
	for _, c := range cases {
		got := detectPrefix(c.code)
		if got != c.want {
			t.Errorf("detectPrefix(%q) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestSplitTencentCode(t *testing.T) {
	prefix, symbol := splitTencentCode("sh600519")
	if prefix != "sh" || symbol != "600519" {
		t.Errorf("splitTencentCode(sh600519) = (%q, %q)", prefix, symbol)
	}

	prefix, symbol = splitTencentCode("600519")
	if prefix != "" || symbol != "600519" {
		t.Errorf("splitTencentCode(600519) = (%q, %q)", prefix, symbol)
	}
}

func TestTencentToExchange(t *testing.T) {
	cases := []struct {
		prefix string
		want   string
	}{
		{"sh", "SSE"},
		{"sz", "SZSE"},
		{"bj", "BSE"},
		{"hk", ""},
	}
	for _, c := range cases {
		got := tencentToExchange(c.prefix)
		if got != c.want {
			t.Errorf("tencentToExchange(%q) = %q, want %q", c.prefix, got, c.want)
		}
	}
}
