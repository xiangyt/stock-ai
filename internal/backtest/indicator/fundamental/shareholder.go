package fundamental

import (
	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
)

// ============================================================================
//  股东相关数值型指标（共享数据源 ShareholderCount）
//  ID: 03007~03010
// ============================================================================

// ============================================================================
//  FreeHoldRatio — 十大流通股东持股比例 (数值型, %)
//  ID: 03007 = CatCodeFundamental("03") + IndFreeHoldRatioSeq("007")
// ============================================================================

type FreeHoldRatio struct{ indicator.BaseIndicator }

var freeHoldRatioDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于20%", Alias: "分散", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 20},
	{Seq: "02", Desc: "20%~50%", Alias: "适中", Operator: indicator.OpBetween, MinThreshold: 20, MaxThreshold: 50},
	{Seq: "03", Desc: "大于50%", Alias: "集中", Operator: indicator.OpGT, MinThreshold: 50, MaxThreshold: 0},
}

func NewFreeHoldRatio() *FreeHoldRatio {
	i := &FreeHoldRatio{BaseIndicator: indicator.BaseIndicator{
		Seq: IndFreeHoldRatioSeq, NameStr: "十大流通股东持股比例",
		CategoryVal: indicator.CatFundamental, Desc: "十大流通股东持股合计占流通股本的比例", UnitStr: "%",
	}}
	ops := signalutil.NumberOpsByUnit(i.UnitStr)
	name := i.NameStr
	sigs := signalutil.BuildRangeSignals(freeHoldRatioDefs, name, ops, func(bs indicator.BaseSignal) indicator.Signal {
		return &freeHoldRatioSignal{BaseSignal: bs}
	})
	i.SetBuiltInSignals(sigs)
	cs := indicator.NewBaseSignal("01", name, "自定义十大流通股东持股筛选", indicator.ValNumber, ops,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 50.0}})
	i.SetCustomSignals([]indicator.Signal{&freeHoldRatioSignal{BaseSignal: cs}})
	return i
}

func (i *FreeHoldRatio) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}
	sc, err := stock.GetShareholderCount()
	if err != nil || sc == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID,
			Message: indicator.DataEmptyError("free_hold_ratio").Error()}
	}
	value := sc.FreeHoldRatioTotal
	for _, cfg := range configs {
		if s, ok := i.Signal[cfg.SignalID]; ok {
			if ss, ok := s.(*freeHoldRatioSignal); ok {
				if res := ss.Evaluate(value, cfg); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: cfg.SignalID,
			Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

type freeHoldRatioSignal struct{ indicator.BaseSignal }

func (s *freeHoldRatioSignal) Evaluate(value float64, cfg *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := cfg.SignalID
	if !cfg.IsCustom() {
		cfg = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "十大流通股东持股比例", "%.2f%%", "%.0f%%", sId, cfg)
}

// ============================================================================
//  HoldRatio — 十大股东持股比例 (数值型, %)
//  ID: 03008 = CatCodeFundamental("03") + IndHoldRatioSeq("008")
// ============================================================================

type HoldRatio struct{ indicator.BaseIndicator }

var holdRatioDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于20%", Alias: "分散", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 20},
	{Seq: "02", Desc: "20%~50%", Alias: "适中", Operator: indicator.OpBetween, MinThreshold: 20, MaxThreshold: 50},
	{Seq: "03", Desc: "大于50%", Alias: "集中", Operator: indicator.OpGT, MinThreshold: 50, MaxThreshold: 0},
}

func NewHoldRatio() *HoldRatio {
	i := &HoldRatio{BaseIndicator: indicator.BaseIndicator{
		Seq: IndHoldRatioSeq, NameStr: "十大股东持股比例",
		CategoryVal: indicator.CatFundamental, Desc: "十大股东持股合计占总股本的比例", UnitStr: "%",
	}}
	ops := signalutil.NumberOpsByUnit(i.UnitStr)
	name := i.NameStr
	sigs := signalutil.BuildRangeSignals(holdRatioDefs, name, ops, func(bs indicator.BaseSignal) indicator.Signal {
		return &holdRatioSignal{BaseSignal: bs}
	})
	i.SetBuiltInSignals(sigs)
	cs := indicator.NewBaseSignal("01", name, "自定义十大股东持股筛选", indicator.ValNumber, ops,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 50.0}})
	i.SetCustomSignals([]indicator.Signal{&holdRatioSignal{BaseSignal: cs}})
	return i
}

func (i *HoldRatio) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}
	sc, err := stock.GetShareholderCount()
	if err != nil || sc == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID,
			Message: indicator.DataEmptyError("hold_ratio").Error()}
	}
	value := sc.HoldRatioTotal
	for _, cfg := range configs {
		if s, ok := i.Signal[cfg.SignalID]; ok {
			if ss, ok := s.(*holdRatioSignal); ok {
				if res := ss.Evaluate(value, cfg); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: cfg.SignalID,
			Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

type holdRatioSignal struct{ indicator.BaseSignal }

func (s *holdRatioSignal) Evaluate(value float64, cfg *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := cfg.SignalID
	if !cfg.IsCustom() {
		cfg = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "十大股东持股比例", "%.2f%%", "%.0f%%", sId, cfg)
}

// ============================================================================
//  HolderNum — 股东户数 (数值型, 户)
//  ID: 03009 = CatCodeFundamental("03") + IndHolderNumSeq("009")
// ============================================================================

type HolderNum struct{ indicator.BaseIndicator }

var holderNumDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于1万户", Alias: "集中", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 10000},
	{Seq: "02", Desc: "1万~5万户", Alias: "一般", Operator: indicator.OpBetween, MinThreshold: 10000, MaxThreshold: 50000},
	{Seq: "03", Desc: "大于5万户", Alias: "分散", Operator: indicator.OpGT, MinThreshold: 50000, MaxThreshold: 0},
}

func NewHolderNum() *HolderNum {
	i := &HolderNum{BaseIndicator: indicator.BaseIndicator{
		Seq: IndHolderNumSeq, NameStr: "股东户数",
		CategoryVal: indicator.CatFundamental, Desc: "报告期末股东总户数", UnitStr: "户",
	}}
	ops := signalutil.NumberOpsByUnit(i.UnitStr)
	name := i.NameStr
	sigs := signalutil.BuildRangeSignals(holderNumDefs, name, ops, func(bs indicator.BaseSignal) indicator.Signal {
		return &holderNumSignal{BaseSignal: bs}
	})
	i.SetBuiltInSignals(sigs)
	cs := indicator.NewBaseSignal("01", name, "自定义股东户数筛选", indicator.ValNumber, ops,
		&indicator.SignalConfig{Operator: indicator.OpLT, Params: map[string]any{indicator.ParamKeyThreshold: 50000.0}})
	i.SetCustomSignals([]indicator.Signal{&holderNumSignal{BaseSignal: cs}})
	return i
}

func (i *HolderNum) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}
	sc, err := stock.GetShareholderCount()
	if err != nil || sc == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID,
			Message: indicator.DataEmptyError("holder_num").Error()}
	}
	value := float64(sc.HolderNum)
	for _, cfg := range configs {
		if s, ok := i.Signal[cfg.SignalID]; ok {
			if ss, ok := s.(*holderNumSignal); ok {
				if res := ss.Evaluate(value, cfg); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: cfg.SignalID,
			Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

type holderNumSignal struct{ indicator.BaseSignal }

func (s *holderNumSignal) Evaluate(value float64, cfg *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := cfg.SignalID
	if !cfg.IsCustom() {
		cfg = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "股东户数", "%.0f户", "%.0f户", sId, cfg)
}

// ============================================================================
//  AvgHoldShares — 户均持股数 (数值型, 股)
//  ID: 03010 = CatCodeFundamental("03") + IndAvgHoldSharesSeq("010")
// ============================================================================

type AvgHoldShares struct{ indicator.BaseIndicator }

var avgHoldSharesDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于5000股", Alias: "分散", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 5000},
	{Seq: "02", Desc: "5000~10000股", Alias: "一般", Operator: indicator.OpBetween, MinThreshold: 5000, MaxThreshold: 10000},
	{Seq: "03", Desc: "大于10000股", Alias: "集中", Operator: indicator.OpGT, MinThreshold: 10000, MaxThreshold: 0},
}

func NewAvgHoldShares() *AvgHoldShares {
	i := &AvgHoldShares{BaseIndicator: indicator.BaseIndicator{
		Seq: IndAvgHoldSharesSeq, NameStr: "户均持股数",
		CategoryVal: indicator.CatFundamental, Desc: "流通A股 / 股东户数", UnitStr: "股",
	}}
	ops := signalutil.NumberOpsByUnit(i.UnitStr)
	name := i.NameStr
	sigs := signalutil.BuildRangeSignals(avgHoldSharesDefs, name, ops, func(bs indicator.BaseSignal) indicator.Signal {
		return &avgHoldSharesSignal{BaseSignal: bs}
	})
	i.SetBuiltInSignals(sigs)
	cs := indicator.NewBaseSignal("01", name, "自定义户均持股数筛选", indicator.ValNumber, ops,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 10000.0}})
	i.SetCustomSignals([]indicator.Signal{&avgHoldSharesSignal{BaseSignal: cs}})
	return i
}

func (i *AvgHoldShares) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}
	sc, err := stock.GetShareholderCount()
	if err != nil || sc == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID,
			Message: indicator.DataEmptyError("avg_hold_shares").Error()}
	}
	value := float64(sc.AvgFreeShares)
	for _, cfg := range configs {
		if s, ok := i.Signal[cfg.SignalID]; ok {
			if ss, ok := s.(*avgHoldSharesSignal); ok {
				if res := ss.Evaluate(value, cfg); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: cfg.SignalID,
			Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

type avgHoldSharesSignal struct{ indicator.BaseSignal }

func (s *avgHoldSharesSignal) Evaluate(value float64, cfg *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := cfg.SignalID
	if !cfg.IsCustom() {
		cfg = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "户均持股数", "%.0f股", "%.0f股", sId, cfg)
}
