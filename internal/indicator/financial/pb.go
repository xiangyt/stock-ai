package financial

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  PB — 市净率 (数值型)
//  ID: 04002 = CatCodeFinancial("04") + IndPBSeq("002")
//  数据源: StockDailySnapshot.PB (float64, 单位: 倍)
// ============================================================================

type PB struct {
	indicator.BaseIndicator
}

var pbDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "0~1", Operator: indicator.OpBetween, MinThreshold: 0, MaxThreshold: 1},
	{Seq: "02", Desc: "1~2", Operator: indicator.OpBetween, MinThreshold: 1, MaxThreshold: 2},
	{Seq: "03", Desc: "2~5", Operator: indicator.OpBetween, MinThreshold: 2, MaxThreshold: 5},
	{Seq: "04", Desc: "大于5", Operator: indicator.OpGT, MinThreshold: 5, MaxThreshold: 0},
}

func NewPB() *PB {
	pb := &PB{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndPBSeq,
			NameStr:     "市净率",
			CategoryVal: indicator.CatFinancial,
			Desc:        "市净率(PB)",
			UnitStr:     "倍",
		},
	}

	numberOps := signalutil.NumberOpsByUnit(pb.UnitStr)

	builtInSigs := signalutil.BuildRangeSignals(pbDefs, "市净率", numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &pbSignal{bs}
	})
	pb.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", "市净率", "自定义市净率筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpBetween,
			Params: map[string]any{indicator.ParamKeyMin: 1.0, indicator.ParamKeyMax: 5.0}})
	pb.SetCustomSignals([]indicator.Signal{&pbSignal{cs1}})
	return pb
}

func (pb *PB) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("PB").Error()}
	}
	value := pb.getValue(snap)
	for _, v := range configs {
		if s, ok := pb.Signal[v.SignalID]; ok {
			if ss, ok := s.(*pbSignal); ok {
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

func (pb *PB) getValue(snap *model.StockDailySnapshot) float64 {
	if snap != nil && snap.PB != 0 {
		return snap.PB
	}
	return 0
}

type pbSignal struct {
	indicator.BaseSignal
}

func (s *pbSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "市净率", "%.2f", "%.0f", sId, config)
}
