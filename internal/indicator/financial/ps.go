package financial

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  PSTTM — 市销率(TTM) (数值型)
//  ID: 04003 = CatCodeFinancial("04") + IndPSTTMSeq("003")
//  数据源: StockDailySnapshot.PSTTM (float64, 单位: 倍)
//  公式: 总市值 / 近12个月营业总收入
// ============================================================================

type PSTTM struct {
	indicator.BaseIndicator
}

var psDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "0~1", Alias: "低估值", Operator: indicator.OpBetween, MinThreshold: 0, MaxThreshold: 1},
	{Seq: "02", Desc: "1~3", Alias: "合理估值", Operator: indicator.OpBetween, MinThreshold: 1, MaxThreshold: 3},
	{Seq: "03", Desc: "3~10", Alias: "偏高估值", Operator: indicator.OpBetween, MinThreshold: 3, MaxThreshold: 10},
	{Seq: "04", Desc: "大于10", Alias: "高估值", Operator: indicator.OpGT, MinThreshold: 10, MaxThreshold: 0},
}

func NewPSTTM() *PSTTM {
	ps := &PSTTM{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndPSTTMSeq,
			NameStr:     "市销率(TTM)",
			CategoryVal: indicator.CatFinancial,
			Desc:        "市销率(TTM) = 总市值 / 近12个月营业总收入",
			UnitStr:     "倍",
		},
	}

	numberOps := signalutil.NumberOpsByUnit(ps.UnitStr)

	indicatorName := "市销率(TTM)"
	builtInSigs := signalutil.BuildRangeSignals(psDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &psTTMSignal{BaseSignal: bs}
	})
	ps.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", indicatorName, "自定义市销率(TTM)筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpLT, Params: map[string]any{indicator.ParamKeyThreshold: 5.0}})
	ps.SetCustomSignals([]indicator.Signal{&psTTMSignal{BaseSignal: cs1}})
	return ps
}

func (ps *PSTTM) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("PSTTM").Error()}
	}
	value := ps.getValue(snap)
	for _, v := range configs {
		if s, ok := ps.Signal[v.SignalID]; ok {
			if ss, ok := s.(*psTTMSignal); ok {
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

func (ps *PSTTM) getValue(snap *model.StockDailySnapshot) float64 {
	if snap != nil && snap.PSTTM != 0 {
		return snap.PSTTM
	}
	return 0
}

type psTTMSignal struct {
	indicator.BaseSignal
}

func (s *psTTMSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "市销率(TTM)", "%.2f", "%.0f", sId, config)
}
