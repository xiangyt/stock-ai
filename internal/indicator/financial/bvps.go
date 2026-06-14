package financial

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  BVPS — 每股净资产(元) (数值型)
//  ID: 04008 = CatCodeFinancial("04") + IndBVPSSeq("008")
//  数据源: StockDailySnapshot.BVPS (float64, 单位: 元)
//  公式: 归属母公司股东权益 / 总股本
// ============================================================================

type BVPS struct {
	indicator.BaseIndicator
}

var bvpsDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于3", Alias: "偏低", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 3},
	{Seq: "02", Desc: "3~5", Alias: "中等", Operator: indicator.OpBetween, MinThreshold: 3, MaxThreshold: 5},
	{Seq: "03", Desc: "大于5", Alias: "偏高", Operator: indicator.OpBetween, MinThreshold: 5, MaxThreshold: 0},
	// {Seq: "04", Desc: "破净股", Operator: indicator.OpGT, MinThreshold: 5, MaxThreshold: 0},
}

func NewBVPS() *BVPS {
	bvps := &BVPS{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndBVPSSeq,
			NameStr:     "每股净资产",
			CategoryVal: indicator.CatFinancial,
			Desc:        "每股净资产(BVPS) = 归属母公司股东权益 / 总股本",
			UnitStr:     "元",
		},
	}

	numberOps := signalutil.NumberOpsByUnit(bvps.UnitStr)

	indicatorName := "每股净资产"
	builtInSigs := signalutil.BuildRangeSignals(bvpsDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &bvpsSignal{BaseSignal: bs}
	})
	bvps.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", indicatorName, "自定义每股净资产筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 5.0}})
	bvps.SetCustomSignals([]indicator.Signal{&bvpsSignal{BaseSignal: cs1}})
	return bvps
}

func (b *BVPS) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("BVPS").Error()}
	}
	value := b.getValue(snap)
	for _, v := range configs {
		if s, ok := b.Signal[v.SignalID]; ok {
			if ss, ok := s.(*bvpsSignal); ok {
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

func (b *BVPS) getValue(snap *model.StockDailySnapshot) float64 {
	if snap != nil && snap.BVPS != 0 {
		return snap.BVPS
	}
	return 0
}

type bvpsSignal struct {
	indicator.BaseSignal
}

func (s *bvpsSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "每股净资产", "%.2f", "%.0f", sId, config)
}
