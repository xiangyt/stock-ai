package financial

import (
	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  NetMargin — 净利率(%) (数值型)
//  ID: 04007 = CatCodeFinancial("04") + IndNetMarginSeq("007")
//  数据源: StockDailySnapshot.NetMargin (float64, 单位: %)
//  公式: 归属母公司净利润 / 营业总收入 × 100%
// ============================================================================

type NetMargin struct {
	indicator.BaseIndicator
}

var netMarginDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于0", Alias: "亏损", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 0},
	{Seq: "02", Desc: "0~10%", Alias: "偏低", Operator: indicator.OpBetween, MinThreshold: 0, MaxThreshold: 10},
	{Seq: "03", Desc: "10%~30%", Alias: "良好", Operator: indicator.OpBetween, MinThreshold: 10, MaxThreshold: 30},
	{Seq: "04", Desc: "大于30%", Alias: "优秀", Operator: indicator.OpGT, MinThreshold: 30, MaxThreshold: 0},
}

func NewNetMargin() *NetMargin {
	nm := &NetMargin{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndNetMarginSeq,
			NameStr:     "净利率",
			CategoryVal: indicator.CatFinancial,
			Desc:        "净利率 = 归属母公司净利润 / 营业总收入 × 100%",
			UnitStr:     "%",
		},
	}

	numberOps := signalutil.NumberOpsByUnitMin(nm.UnitStr, -99)

	indicatorName := "净利率"
	builtInSigs := signalutil.BuildRangeSignals(netMarginDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &netMarginSignal{BaseSignal: bs}
	})
	nm.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", indicatorName, "自定义净利率筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 15.0}})
	nm.SetCustomSignals([]indicator.Signal{&netMarginSignal{BaseSignal: cs1}})
	return nm
}

func (n *NetMargin) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("NetMargin").Error()}
	}
	value := n.getValue(snap)
	for _, v := range configs {
		if s, ok := n.Signal[v.SignalID]; ok {
			if ss, ok := s.(*netMarginSignal); ok {
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

func (n *NetMargin) getValue(snap *model.StockDailySnapshot) float64 {
	if snap != nil && snap.NetMargin != 0 {
		return snap.NetMargin
	}
	return 0
}

type netMarginSignal struct {
	indicator.BaseSignal
}

func (s *netMarginSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "净利率", "%.2f%%", "%.0f", sId, config)
}
