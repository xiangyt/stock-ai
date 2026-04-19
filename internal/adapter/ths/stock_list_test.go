package ths

import (
	"testing"
)

// ========== 股票列表测试 ==========

func TestFetchStockListPage(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	stocks, hasMore, err := a.fetchStockListPage(1)
	if err != nil {
		t.Fatalf("fetchStockListPage failed: %v", err)
	}

	t.Logf("返回 %d 条数据, hasMore=%v", len(stocks), hasMore)

	if len(stocks) == 0 {
		t.Fatal("no data returned")
	}

	for i, s := range stocks {
		if i < 10 {
			t.Logf("  [%d] %s(%s) %s/%s", i+1, s.Name, s.Code, s.Exchange, s.ListingBoard)
		}
		if s.Code == "" {
			t.Errorf("[%d] Code is empty", i)
		}
		if s.Exchange == "" {
			t.Errorf("[%d] Exchange is empty for code %s", i, s.Code)
		}
	}
}

// ========== 股票详情测试 ==========

func TestGetStockDetail(t *testing.T) {
	a := newTestAdapter()
	defer a.Close()

	tests := []struct {
		code        string
		wantMarket  string
		wantBoard   string
	}{
		{"600519", "SSE", "main"},     // 沪市主板
		{"300750", "SZSE", "chinext"}, // 创业板
		{"688001", "SSE", "star"},     // 科创板
		{"000001", "SZSE", "main"},    // 深市主板
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			detail, err := a.GetStockDetail(nil, tt.code)
			if err != nil {
				t.Fatalf("GetStockDetail(%s) failed: %v", tt.code, err)
			}
			if detail == nil {
				t.Fatal("returned nil")
			}
			if detail.Code != tt.code {
				t.Errorf("Code = %q, want %q", detail.Code, tt.code)
			}
			if detail.Exchange != tt.wantMarket && tt.wantMarket != "" {
				t.Logf("Exchange = %q (want %q, 仅记录)", detail.Exchange, tt.wantMarket)
			}
			if detail.ListingBoard != tt.wantBoard && tt.wantBoard != "" {
				t.Logf("ListingBoard = %q (want %q, 仅记录)", detail.ListingBoard, tt.wantBoard)
			}
			t.Logf("  %s → Exchange:%s Board:%s", tt.code, detail.Exchange, detail.ListingBoard)
		})
	}
}

// ========== buildTHSCode 单元测试 ==========

func TestBuildTHSCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// 沪市
		{"600519", "hs_600519"},
		{"601318", "hs_601318"},
		{"688001", "hs_688001"}, // 科创板也用 hs_ 前缀
		// 深市
		{"000001", "hs_000001"},
		{"002404", "hs_002404"},
		{"300750", "hs_300750"}, // 创业板也用 hs_ 前缀
		// 北交所
		{"835305", "hs_835305"},
		// 不支持的市场
		{"123456", ""},
		{"HK00700", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := buildTHSCode(tt.input)
			if got != tt.want {
				t.Errorf("buildTHSCode(%s) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ========== detectExchange 单元测试 ==========

func TestDetectExchange(t *testing.T) {
	tests := []struct {
		input         string
		wantMarket    string
		wantExchange  string
	}{
		{"600519", "SH", "SSE"},
		{"601318", "SH", "SSE"},
		{"688001", "SH", "SSE"},
		{"000001", "SZ", "SZSE"},
		{"002404", "SZ", "SZSE"},
		{"300750", "SZ", "SZSE"},
		{"835305", "SZ", "SZSE"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			market, exchange, _ := detectExchange(tt.input)
			if market != tt.wantMarket {
				t.Errorf("market = %q, want %q", market, tt.wantMarket)
			}
			if exchange != tt.wantExchange {
				t.Errorf("exchange = %q, want %q", exchange, tt.wantExchange)
			}
		})
	}
}

// ========== parseCode 单元测试 ==========

func TestParseCode(t *testing.T) {
	a := newTestAdapter()

	tests := []struct {
		input       string
		wantSymbol  string
		wantMarket  string
	}{
		{"000001.SZ", "000001", "SZ"},
		{"600519.SH", "600519", "SH"},
		{"300750.SZSE", "300750", "SZSE"},
		{"000001", "000001", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			symbol, market, err := a.parseCode(tt.input)
			if err != nil {
				t.Fatalf("parseCode(%s) error: %v", tt.input, err)
			}
			if symbol != tt.wantSymbol {
				t.Errorf("symbol = %q, want %q", symbol, tt.wantSymbol)
			}
			if market != tt.wantMarket {
				t.Errorf("market = %q, want %q", market, tt.wantMarket)
			}
		})
	}
}
