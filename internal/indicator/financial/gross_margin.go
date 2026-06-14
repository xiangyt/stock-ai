package financial

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  GrossMargin — 毛利率(%) (数值型)
//  ID: 04006 = CatCodeFinancial("04") + IndGrossMarginSeq("006")
//  数据源: StockDailySnapshot.GrossMargin (float64, 单位: %)
//  公式: (营业收入 - 营业成本) / 营业收入 × 100%
// ============================================================================

type GrossMargin struct {
	indicator.BaseIndicator
}

var grossMarginDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于20%", Alias: "低毛利", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 20},
	{Seq: "02", Desc: "20%~50%", Alias: "中毛利", Operator: indicator.OpBetween, MinThreshold: 20, MaxThreshold: 50},
	{Seq: "03", Desc: "50%~70%", Alias: "高毛利", Operator: indicator.OpBetween, MinThreshold: 50, MaxThreshold: 70},
	{Seq: "04", Desc: "大于70%", Alias: "超高毛利", Operator: indicator.OpGT, MinThreshold: 70, MaxThreshold: 0},
}

func NewGrossMargin() *GrossMargin {
	gm := &GrossMargin{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndGrossMarginSeq,
			NameStr:     "毛利率",
			CategoryVal: indicator.CatFinancial,
			Desc:        "毛利率 = (营业收入 - 营业成本) / 营业收入 × 100%",
			UnitStr:     "%",
		},
	}

	numberOps := signalutil.NumberOpsByUnit(gm.UnitStr)

	indicatorName := "毛利率"
	builtInSigs := signalutil.BuildRangeSignals(grossMarginDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &grossMarginSignal{BaseSignal: bs}
	})
	gm.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", indicatorName, "自定义毛利率筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 40.0}})
	gm.SetCustomSignals([]indicator.Signal{&grossMarginSignal{BaseSignal: cs1}})
	return gm
}

func (g *GrossMargin) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("GrossMargin").Error()}
	}
	value := g.getValue(snap)
	for _, v := range configs {
		if s, ok := g.Signal[v.SignalID]; ok {
			if ss, ok := s.(*grossMarginSignal); ok {
				if res := ss.Evaluate(value, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: v.SignalID, Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

func (g *GrossMargin) getValue(snap *model.StockDailySnapshot) float64 {
	if snap != nil && snap.GrossMargin != 0 {
		return snap.GrossMargin
	}
	return 0
}

type grossMarginSignal struct {
	indicator.BaseSignal
}

func (s *grossMarginSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "毛利率", "%.2f%%", "%.0f", sId, config)
}
