package fundamental

import (
	"testing"

	"stock-ai/internal/model"
)

func TestFloatRatio_GetValue(t *testing.T) {
	tests := []struct {
		name        string
		floatShares int64
		totalShares int64
		want        float64
	}{
		{"全流通", 1_000_000_000, 1_000_000_000, 100},
		{"半流通", 500_000_000, 1_000_000_000, 50},
		{"小流通", 200_000_000, 1_000_000_000, 20},
		{"总股本为0", 500_000_000, 0, 0},
		{"nil snapshot", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &FloatRatio{}
			var snap *model.StockDailySnapshot
			if tt.totalShares != 0 {
				snap = &model.StockDailySnapshot{
					FloatShares: tt.floatShares,
					TotalShares: tt.totalShares,
				}
			}
			got := f.getValue(snap)
			if got != tt.want {
				t.Errorf("getValue = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}

func TestFloatRatio_Register(t *testing.T) {
	ind := NewFloatRatio()
	if ind.ID() != "03006" {
		t.Errorf("ID() = %s, want 03006", ind.ID())
	}
	if ind.Name() != "流通比例" {
		t.Errorf("Name() = %s, want 流通比例", ind.Name())
	}
	if ind.Unit() != "%" {
		t.Errorf("Unit() = %s, want %%", ind.Unit())
	}
	if got := len(ind.BuiltInSignals()); got != 4 {
		t.Errorf("BuiltInSignals() = %d, want 4", got)
	}
	if got := len(ind.CustomSignals()); got != 1 {
		t.Errorf("CustomSignals() = %d, want 1", got)
	}
}
