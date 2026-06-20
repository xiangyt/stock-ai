package tencentstock

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"stock-ai/internal/adapter"
)

// ============================================================================
//  parseMinuteBar 测试
// ============================================================================

func TestParseMinuteBar_Normal(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    adapter.IntradayBar
		wantErr bool
	}{
		{
			name: "正常4字段",
			line: "0930 14.61 6 8766.00",
			want: adapter.IntradayBar{
				Time:   "09:30",
				Price:  1461,   // 14.61×100
				Volume: 600,    // 6手×100
				Amount: 876600, // 8766.00元×100
			},
		},
		{
			name: "开盘首分钟大量",
			line: "0930 14.61 165 240653.00",
			want: adapter.IntradayBar{
				Time:   "09:30",
				Price:  1461,
				Volume: 16500,
				Amount: 24065300,
			},
		},
		{
			name: "价格含小数点",
			line: "1030 28.50 100 285000.00",
			want: adapter.IntradayBar{
				Time:   "10:30",
				Price:  2850,
				Volume: 10000,
				Amount: 28500000,
			},
		},
		{
			name: "仅3字段无成交额",
			line: "1100 15.20 50",
			want: adapter.IntradayBar{
				Time:   "11:00",
				Price:  1520,
				Volume: 5000,
				Amount: 0,
			},
		},
		{
			name: "下午时段",
			line: "1305 14.80 30 44400.00",
			want: adapter.IntradayBar{
				Time:   "13:05",
				Price:  1480,
				Volume: 3000,
				Amount: 4440000,
			},
		},
		{
			name:    "空行",
			line:    "",
			wantErr: true,
		},
		{
			name:    "仅1字段",
			line:    "0930",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMinuteBar(tt.line)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for line %q", tt.line)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMinuteBar(%q) err: %v", tt.line, err)
			}
			if got.Time != tt.want.Time {
				t.Errorf("Time = %q, want %q", got.Time, tt.want.Time)
			}
			if got.Price != tt.want.Price {
				t.Errorf("Price = %d, want %d", got.Price, tt.want.Price)
			}
			if got.Volume != tt.want.Volume {
				t.Errorf("Volume = %d, want %d", got.Volume, tt.want.Volume)
			}
			if got.Amount != tt.want.Amount {
				t.Errorf("Amount = %d, want %d", got.Amount, tt.want.Amount)
			}
		})
	}
}

// ============================================================================
//  parseIntradayResponse 测试
// ============================================================================

func TestParseIntradayResponse(t *testing.T) {
	// Build a qt array with all 47 fields
	qtArr := makeQT47()
	qtArr[1] = "测试股"
	qtArr[4] = "14.50"
	qtJSON, _ := json.Marshal(qtArr)
	body := fmt.Sprintf(
		`min_data_sz300484={"code":0,"msg":"","data":{"sz300484":{"qt":{"sz300484":%s},"data":{"data":["0930 14.61 6 8766.00","0931 14.58 165 240653.00","0932 14.55 300 436500.00"]}}}}`,
		string(qtJSON),
	)

	result, err := parseIntradayResponse(body, "300484", "sz300484")
	if err != nil {
		t.Fatalf("parseIntradayResponse err: %v", err)
	}
	if result.Code != "300484" {
		t.Errorf("Code = %s, want 300484", result.Code)
	}
	if result.Name != "测试股" {
		t.Errorf("Name = %s, want 测试股", result.Name)
	}
	if result.PreClose != 1450 {
		t.Errorf("PreClose = %d, want 1450 (14.50元)", result.PreClose)
	}
	if result.Current != 1481 {
		t.Errorf("Current = %d, want 1481", result.Current)
	}
	if result.Open != 1455 {
		t.Errorf("Open = %d, want 1455", result.Open)
	}
	if result.High != 1485 {
		t.Errorf("High = %d, want 1485", result.High)
	}
	if result.Low != 1440 {
		t.Errorf("Low = %d, want 1440", result.Low)
	}
	if len(result.Bars) != 3 {
		t.Fatalf("len(Bars) = %d, want 3", len(result.Bars))
	}
	if result.Bars[0].Time != "09:30" {
		t.Errorf("Bars[0].Time = %s, want 09:30", result.Bars[0].Time)
	}
}

func TestParseIntradayResponse_InvalidJSON(t *testing.T) {
	_, err := parseIntradayResponse("min_data=invalid", "300484", "sz300484")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseIntradayResponse_MissingCode(t *testing.T) {
	body := `min_data_sz300484={"code":0,"msg":"","data":{"sz000001":{"data":{"data":["0930 10.00 1 1000.00"]}}}}`
	_, err := parseIntradayResponse(body, "300484", "sz300484")
	if err == nil {
		t.Fatal("expected error for missing code in response")
	}
}

func TestParseIntradayResponse_EmptyBars(t *testing.T) {
	body := `min_data_sz300484={"code":0,"msg":"","data":{"sz300484":{"qt":{"sz300484":["","","","","14.50","","","","","","","","","","","","","","","","","","","","","","","","","","","","","","","","","","","","","","","","","","",""]},"data":{"data":[]}}}}`
	_, err := parseIntradayResponse(body, "300484", "sz300484")
	if err == nil {
		t.Fatal("expected error for empty bars")
	}
}

// ============================================================================
//  parseQtFields 测试
// ============================================================================

func makeQT47() []string {
	arr := make([]string, 47)
	arr[0] = "2026-06-20 15:00|SZ_close_收盘"
	arr[1] = "测试股"
	arr[2] = "300484"
	arr[3] = "14.81"    // current
	arr[4] = "14.50"    // pre-close
	arr[5] = "14.55"    // open
	arr[6] = "35878"    // volume(手)
	arr[7] = "18900"    // outer
	arr[8] = "16978"    // inner
	arr[9] = "14.81"    // ask1
	arr[10] = "78"
	arr[11] = "14.82"   // ask2
	arr[12] = "202"
	arr[13] = "14.83"   // ask3
	arr[14] = "150"
	arr[15] = "14.84"   // ask4
	arr[16] = "69"
	arr[17] = "14.85"   // ask5
	arr[18] = "47"
	arr[19] = "14.80"   // bid1
	arr[20] = "61"
	arr[21] = "14.79"   // bid2
	arr[22] = "40"
	arr[23] = "14.78"   // bid3
	arr[24] = "91"
	arr[25] = "14.77"   // bid4
	arr[26] = "49"
	arr[27] = "14.76"   // bid5
	arr[28] = "36"
	arr[30] = "20260620150000"
	arr[31] = "0.31"    // change
	arr[32] = "2.14"    // change_pct
	arr[33] = "14.85"   // high
	arr[34] = "14.40"   // low
	arr[37] = "5248.17" // amount(万元)
	arr[38] = "2.16"    // turnover
	arr[39] = "86.76"   // PE
	arr[43] = "3.10"    // amplitude
	arr[44] = "24.64"   // float market cap
	arr[45] = "30.68"   // total market cap
	arr[46] = "4.65"    // PB
	return arr
}

func TestParseQtFields_Full(t *testing.T) {
	qt := map[string][]string{"sz300484": makeQT47()}
	result := parseQtFields(qt, "", "sz300484", "300484")

	if result.Code != "300484" {
		t.Errorf("Code = %s, want 300484", result.Code)
	}
	if result.Name != "测试股" {
		t.Errorf("Name = %s, want 测试股", result.Name)
	}
	if result.Current != 1481 {
		t.Errorf("Current = %d, want 1481 (14.81元)", result.Current)
	}
	if result.PreClose != 1450 {
		t.Errorf("PreClose = %d, want 1450 (14.50元)", result.PreClose)
	}
	if result.Open != 1455 {
		t.Errorf("Open = %d, want 1455 (14.55元)", result.Open)
	}
	if result.High != 1485 {
		t.Errorf("High = %d, want 1485 (14.85元)", result.High)
	}
	if result.Low != 1440 {
		t.Errorf("Low = %d, want 1440 (14.40元)", result.Low)
	}
	if result.Volume != 3587800 {
		t.Errorf("Volume = %d, want 3587800 (35878手×100)", result.Volume)
	}
	if result.Change != 31 {
		t.Errorf("Change = %d, want 31", result.Change)
	}
	if result.ChangePct != 2.14 {
		t.Errorf("ChangePct = %f, want 2.14", result.ChangePct)
	}
	if result.Turnover != 2.16 {
		t.Errorf("Turnover = %f, want 2.16", result.Turnover)
	}
	if result.Pe != 86.76 {
		t.Errorf("Pe = %f, want 86.76", result.Pe)
	}
	if result.Pb != 4.65 {
		t.Errorf("Pb = %f, want 4.65", result.Pb)
	}
	if result.MarketCap != 30.68 {
		t.Errorf("MarketCap = %f, want 30.68", result.MarketCap)
	}
	if result.FloatMarketCap != 24.64 {
		t.Errorf("FloatMarketCap = %f, want 24.64", result.FloatMarketCap)
	}
	if result.Amplitude != 3.10 {
		t.Errorf("Amplitude = %f, want 3.10", result.Amplitude)
	}
	if result.Date != "2026-06-20" {
		t.Errorf("Date = %s, want 2026-06-20", result.Date)
	}

	// 验证五档盘口
	d := result.Depth
	if d == nil {
		t.Fatal("Depth is nil")
	}
	if d.Ask1Price != 1481 {
		t.Errorf("Ask1Price = %d, want 1481", d.Ask1Price)
	}
	if d.Ask1Volume != 7800 {
		t.Errorf("Ask1Volume = %d, want 7800", d.Ask1Volume)
	}
	if d.Ask5Price != 1485 {
		t.Errorf("Ask5Price = %d, want 1485", d.Ask5Price)
	}
	if d.Ask5Volume != 4700 {
		t.Errorf("Ask5Volume = %d, want 4700", d.Ask5Volume)
	}
	if d.Bid1Price != 1480 {
		t.Errorf("Bid1Price = %d, want 1480", d.Bid1Price)
	}
	if d.Bid1Volume != 6100 {
		t.Errorf("Bid1Volume = %d, want 6100", d.Bid1Volume)
	}
	if d.Bid5Price != 1476 {
		t.Errorf("Bid5Price = %d, want 1476", d.Bid5Price)
	}
	if d.Bid5Volume != 3600 {
		t.Errorf("Bid5Volume = %d, want 3600", d.Bid5Volume)
	}
}

func TestParseQtFields_Minimal(t *testing.T) {
	// Widget with insufficient qt data
	qt := map[string][]string{"sz300484": {"", "迷你股", "", "", "14.50"}}
	result := parseQtFields(qt, "", "sz300484", "300484")
	if result.Name != "迷你股" {
		t.Errorf("Name = %s, want 迷你股", result.Name)
	}
	if result.PreClose != 1450 {
		t.Errorf("PreClose = %d, want 1450", result.PreClose)
	}
}

func TestParseQtFields_MissingQt(t *testing.T) {
	qt := map[string][]string{}
	result := parseQtFields(qt, "", "sz300484", "300484")
	if result.Code != "300484" {
		t.Errorf("Code = %s, want 300484", result.Code)
	}
	if result.Name != "" {
		t.Errorf("Name = %s, want empty", result.Name)
	}
}

// ============================================================================
//  GetIntraday 集成测试
// ============================================================================

func TestGetIntraday(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	data, err := a.GetIntraday(ctx, "300484")
	if err != nil {
		t.Fatalf("GetIntraday 失败: %v", err)
	}
	if data == nil {
		t.Fatal("GetIntraday 返回 nil")
	}
	if data.Code != "300484" {
		t.Errorf("Code = %s, want 300484", data.Code)
	}
	if data.Name == "" {
		t.Error("Name 为空")
	}
	if data.PreClose <= 0 {
		t.Errorf("PreClose = %d, 应 > 0", data.PreClose)
	}
	if len(data.Bars) == 0 {
		t.Fatal("Bars 为空")
	}

	first := data.Bars[0]
	if first.Time == "" {
		t.Error("首根 bar 时间为空")
	}
	if first.Price <= 0 {
		t.Errorf("首根 bar 价格 = %d, 应 > 0", first.Price)
	}

	last := data.Bars[len(data.Bars)-1]

	t.Logf("%s (%s) %s 分时: %d bars, 昨收=%.2f 今开=%.2f 最高=%.2f 最低=%.2f 当前=%.2f 换手=%.2f%% PE=%.2f PB=%.2f 市值=%.2f亿",
		data.Code, data.Name, data.Date, len(data.Bars),
		float64(data.PreClose)/100, float64(data.Open)/100,
		float64(data.High)/100, float64(data.Low)/100,
		float64(data.Current)/100, data.Turnover, data.Pe, data.Pb, data.MarketCap)
	t.Logf("  首=%.2f(%s) 末=%.2f(%s) 涨跌=%.2f(%+.2f%%) 振幅=%.2f%%",
		float64(first.Price)/100, first.Time,
		float64(last.Price)/100, last.Time,
		float64(data.Change)/100, data.ChangePct, data.Amplitude)
}

func TestGetIntraday_BarsAreAscending(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	data, err := a.GetIntraday(ctx, "002404")
	if err != nil {
		t.Skipf("GetIntraday 失败(非交易日可能无数据): %v", err)
	}
	if len(data.Bars) < 2 {
		t.Skip("分时 bar 不足 2 根，跳过排序校验")
	}

	for i := 1; i < len(data.Bars); i++ {
		if data.Bars[i].Time <= data.Bars[i-1].Time {
			t.Errorf("Bar[%d].Time=%s <= Bar[%d].Time=%s, 时间乱序",
				i, data.Bars[i].Time, i-1, data.Bars[i-1].Time)
		}
	}
}

func TestGetIntraday_VolumeIsCumulative(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	data, err := a.GetIntraday(ctx, "002404")
	if err != nil {
		t.Skipf("GetIntraday 失败: %v", err)
	}
	if len(data.Bars) < 2 {
		t.Skip("分时 bar 不足 2 根，跳过累计校验")
	}

	for i := 1; i < len(data.Bars); i++ {
		if data.Bars[i].Volume < data.Bars[i-1].Volume {
			t.Errorf("Bar[%d].Volume=%d < Bar[%d].Volume=%d, 成交量应为累计值",
				i, data.Bars[i].Volume, i-1, data.Bars[i-1].Volume)
		}
	}
}
