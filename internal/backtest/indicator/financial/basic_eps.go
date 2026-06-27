package financial

import (
	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  BasicEPS — 基本每股收益(元) (数值型)
//  ID: 04009 = CatCodeFinancial("04") + IndBasicEPSSeq("009")
//  数据源: StockDailySnapshot.BasicEPS (float64, 单位: 元)
//  公式: 归属母公司净利润 / 普通股加权平均股数
// ============================================================================

type BasicEPS struct {
	indicator.BaseIndicator
}

var basicEPSDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于0", Alias: "亏损", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 0},
	{Seq: "02", Desc: "0~0.5", Alias: "偏低", Operator: indicator.OpBetween, MinThreshold: 0, MaxThreshold: 0.5},
	{Seq: "03", Desc: "0.5~1", Alias: "中档", Operator: indicator.OpBetween, MinThreshold: 0.5, MaxThreshold: 1},
	{Seq: "04", Desc: "大于1", Alias: "优秀", Operator: indicator.OpGT, MinThreshold: 1, MaxThreshold: 0},
}

func NewBasicEPS() *BasicEPS {
	eps := &BasicEPS{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndBasicEPSSeq,
			NameStr:     "基本每股收益",
			CategoryVal: indicator.CatFinancial,
			Desc:        "基本每股收益(EPS) = 归属母公司净利润 / 普通股加权平均股数",
			UnitStr:     "元",
		},
	}

	numberOps := signalutil.NumberOpsByUnitMin(eps.UnitStr, -99)

	indicatorName := "基本每股收益"
	builtInSigs := signalutil.BuildRangeSignals(basicEPSDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &basicEPSSignal{BaseSignal: bs}
	})
	eps.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", indicatorName, "自定义基本每股收益筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 0.5}})
	eps.SetCustomSignals([]indicator.Signal{&basicEPSSignal{BaseSignal: cs1}})
	return eps
}

func (e *BasicEPS) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("BasicEPS").Error()}
	}
	value := e.getValue(snap)
	for _, v := range configs {
		if s, ok := e.Signal[v.SignalID]; ok {
			if ss, ok := s.(*basicEPSSignal); ok {
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

func (e *BasicEPS) getValue(snap *model.StockDailySnapshot) float64 {
	if snap != nil && snap.BasicEPS != 0 {
		return snap.BasicEPS
	}
	return 0
}

type basicEPSSignal struct {
	indicator.BaseSignal
}

func (s *basicEPSSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "基本每股收益", "%.4f", "%.2f", sId, config)
}
