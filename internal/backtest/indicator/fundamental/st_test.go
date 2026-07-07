package fundamental

import (
	"testing"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

func TestIsStStock(t *testing.T) {
	tests := []struct {
		name      string
		stockName string
		isSt      bool
	}{
		{"普通股", "平安银行", false},
		{"ST股", "ST平安", true},
		{"*ST股", "*ST平安", true},
		{"空名称", "", false},
		{"含st小写", "test", false}, // 中文名称不含小写st
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStStock(tt.stockName)
			if got != tt.isSt {
				t.Errorf("isStStock(%q) = %v, want %v", tt.stockName, got, tt.isSt)
			}
		})
	}
}

// mockStockForSt 模拟StockSource用于ST测试
type mockStockForSt struct {
	name string
}

func (m *mockStockForSt) GetCode() string                                           { return "000001" }
func (m *mockStockForSt) GetName() string                                            { return m.name }
func (m *mockStockForSt) Price() float64                                             { return 10.0 }
func (m *mockStockForSt) GetDailyKline() ([]*model.DailyKline, error)                { return nil, nil }
func (m *mockStockForSt) GetWeeklyKline() ([]*model.WeeklyKline, error)              { return nil, nil }
func (m *mockStockForSt) GetMonthlyKline() ([]*model.MonthlyKline, error)            { return nil, nil }
func (m *mockStockForSt) GetYearlyKline() ([]*model.YearlyKline, error)              { return nil, nil }
func (m *mockStockForSt) GetDailySnapshot() (*model.StockDailySnapshot, error)       { return nil, nil }
func (m *mockStockForSt) GetPerformanceReport() ([]*model.PerformanceReport, error)  { return nil, nil }
func (m *mockStockForSt) GetShareholderCount() (*model.ShareholderCount, error)      { return nil, nil }
func (m *mockStockForSt) GetDetail() (*model.Stock, error) {
	return &model.Stock{Name: m.name}, nil
}

func TestIsSt_Evaluate_STStock(t *testing.T) {
	ind := NewIsSt()

	stock := &mockStockForSt{name: "*ST平安"}

	// 模拟引擎传入的SignalConfig（带SignalID）
	configs := []*indicator.SignalConfig{
		{
			SignalID: "03011001", // 内置: 是ST股
			Operator: indicator.OpEQ,
			Params:   map[string]any{indicator.ParamKeyThreshold: "st"},
		},
	}
	result := ind.Evaluate(stock, configs)

	if result.Result != indicator.ResultPassed {
		t.Errorf("ST股票应判定为是ST股，实际: %s", result.Message)
	}
}

func TestIsSt_Evaluate_NormalStock(t *testing.T) {
	ind := NewIsSt()

	stock := &mockStockForSt{name: "平安银行"}

	// 模拟引擎传入的SignalConfig（带SignalID）
	configs := []*indicator.SignalConfig{
		{
			SignalID: "03011002", // 内置: 非ST股
			Operator: indicator.OpEQ,
			Params:   map[string]any{indicator.ParamKeyThreshold: "normal"},
		},
	}
	result := ind.Evaluate(stock, configs)

	if result.Result != indicator.ResultPassed {
		t.Errorf("非ST股票应判定为非ST股，实际: %s", result.Message)
	}
}

func TestStSignalID(t *testing.T) {
	ind := NewIsSt()

	// 验证所有内置信号已正确注册
	expectedIDs := []string{
		"03011001", // 内置: 是ST股
		"03011002", // 内置: 非ST股
	}
	for _, id := range expectedIDs {
		if s, ok := ind.Signal[id]; !ok {
			t.Errorf("预期信号 %s 未注册", id)
		} else {
			t.Logf("信号 %s: %s", id, s.Description())
		}
	}

	// 验证无自定义信号
	customSigs := ind.CustomSignals()
	if len(customSigs) != 0 {
		t.Errorf("自定义信号应为空，实际 %d 个", len(customSigs))
	}

	// 验证仅2个内置信号
	if len(ind.Signal) != 2 {
		t.Errorf("Signal映射应为2个，实际 %d 个", len(ind.Signal))
	}
}

func TestStSignal_EnumEval(t *testing.T) {
	tests := []struct {
		name      string
		stVal     string
		operator  indicator.CompareOperator
		threshold string
		wantPass  bool
	}{
		{"是ST-匹配", "st", indicator.OpEQ, "st", true},
		{"是ST-不匹配", "normal", indicator.OpEQ, "st", false},
		{"非ST-匹配", "normal", indicator.OpEQ, "normal", true},
		{"非ST-不匹配", "st", indicator.OpEQ, "normal", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &indicator.SignalConfig{
				SignalID: "test",
				Operator: tt.operator,
				Params:   map[string]any{indicator.ParamKeyThreshold: tt.threshold},
			}
			result := signalutil.EvalEnumOp(tt.stVal, "ST状态", "test", cfg)
			if (result.Result == indicator.ResultPassed) != tt.wantPass {
				t.Errorf("EvalEnumOp(%q, %v, %q) = %v, want pass=%v",
					tt.stVal, tt.operator, tt.threshold, result.Result, tt.wantPass)
			}
		})
	}
}
