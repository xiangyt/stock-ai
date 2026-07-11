package technical

import (
	"testing"

	"stock-ai/internal/backtest/indicator"
)

func TestSignalMaSeqCrossMA120_Registration(t *testing.T) {
	sig := NewSignalMaSeqCrossMA120("20")
	if sig == nil {
		t.Fatal("NewSignalMaSeqCrossMA120 返回 nil")
	}
	if sig.Name() != "均线依次上穿半年线" {
		t.Errorf("Name=%q", sig.Name())
	}

	cfg := sig.DefaultConfig()
	if cfg.Operator != indicator.OpCustom {
		t.Errorf("Operator=%v", cfg.Operator)
	}
	start := int(cfg.GetFloat64(indicator.ParamKeyLookbackStart, 0))
	end := int(cfg.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	if start != 25 {
		t.Errorf("lookback_start=%d, want 25", start)
	}
	if end != 2 {
		t.Errorf("lookback_end=%d, want 2", end)
	}
}

func TestMa_AllSignals_Registered(t *testing.T) {
	ma := NewMa()

	if sig, ok := ma.Signal["01001020"]; !ok {
		t.Errorf("内置信号 01001020 (均线依次上穿半年线) 未注册")
	} else {
		t.Logf("内置信号: %s (%s)", sig.Name(), sig.Description())
	}

	customFound := false
	for _, s := range ma.CustomSignals() {
		if s.SignalType() == "均线依次上穿半年线" {
			customFound = true
			cfg := s.DefaultConfig()
			start := int(cfg.GetFloat64(indicator.ParamKeyLookbackStart, 0))
			end := int(cfg.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
			if start != 25 || end != 2 {
				t.Errorf("自定义信号默认窗口: start=%d, end=%d, want 25,2", start, end)
			}
			break
		}
	}
	if !customFound {
		t.Error("自定义信号'均线依次上穿半年线'未找到")
	}
}
