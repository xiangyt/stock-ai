package financial

import (
	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  PETTM — 市盈率(TTM) (数值型)
//  ID: 04001 = CatCodeFinancial("04") + IndPETTMSeq("001")
//  数据源: StockDailySnapshot.PETTM (float64, 单位: 倍)
// ============================================================================

type PETTM struct {
	indicator.BaseIndicator
}

var peDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于0", Alias: "亏损", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 0},
	{Seq: "02", Desc: "0~25", Alias: "低估", Operator: indicator.OpBetween, MinThreshold: 0, MaxThreshold: 25},
	{Seq: "03", Desc: "25~50", Alias: "合理", Operator: indicator.OpBetween, MinThreshold: 25, MaxThreshold: 50},
	{Seq: "04", Desc: "大于50", Alias: "高估", Operator: indicator.OpGT, MinThreshold: 50, MaxThreshold: 0},
}

func NewPETTM() *PETTM {
	p := &PETTM{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndPETTMSeq,
			NameStr:     "市盈率",
			CategoryVal: indicator.CatFinancial,
			Desc:        "默认使用市盈率(TTM)",
			UnitStr:     "倍",
		},
	}

	numberOps := signalutil.NumberOpsByUnit(p.UnitStr)

	indicatorName := "市盈率(TTM)"
	builtInSigs := signalutil.BuildRangeSignals(peDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &peTTMSignal{BaseSignal: bs}
	})
	p.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", "市盈率(TTM)", "自定义市盈率(TTM)筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpBetween, Params: map[string]any{indicator.ParamKeyMin: 0, indicator.ParamKeyMax: 30}})
	cs2 := indicator.NewBaseSignal("02", "市盈率(动)", "自定义市盈率(动)筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpBetween, Params: map[string]any{indicator.ParamKeyMin: 0, indicator.ParamKeyMax: 30}})
	cs3 := indicator.NewBaseSignal("03", "市盈率(静)", "自定义市盈率(静)筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpBetween, Params: map[string]any{indicator.ParamKeyMin: 0, indicator.ParamKeyMax: 30}})

	p.SetCustomSignals([]indicator.Signal{
		&peTTMSignal{BaseSignal: cs1},
		&peDynamicSignal{BaseSignal: cs2},
		&peStaticSignal{BaseSignal: cs3},
	})
	return p
}

func (p *PETTM) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("DailySnapshot").Error()}
	}

	for _, v := range configs {
		if s, ok := p.Signal[v.SignalID]; ok {
			var res *indicator.EvaluatedStock
			switch ss := s.(type) {
			case *peTTMSignal:
				res = ss.Evaluate(snap, v)
			case *peDynamicSignal:
				res = ss.Evaluate(snap, v)
			case *peStaticSignal:
				res = ss.Evaluate(snap, v)
			default:
				return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: v.SignalID, Message: indicator.ErrUnsupportedSignal.Error()}
			}
			if res.Result == indicator.ResultPassed {
				continue
			} else {
				return res
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: v.SignalID, Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

type peTTMSignal struct {
	indicator.BaseSignal
}

func (s *peTTMSignal) Evaluate(snap *model.StockDailySnapshot, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	value := s.getValue(snap)
	return signalutil.EvalNumberOp(value, s.NameStr, "%.2f", "%.0f", sId, config)
}

func (s *peTTMSignal) getValue(snap *model.StockDailySnapshot) float64 {
	if snap != nil && snap.PETTM != 0 {
		return snap.PETTM
	}
	if snap != nil && snap.PEStatic != 0 {
		return snap.PEStatic
	}
	return 0
}

type peDynamicSignal struct {
	indicator.BaseSignal
}

func (s *peDynamicSignal) Evaluate(snap *model.StockDailySnapshot, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	value := s.getValue(snap)
	return signalutil.EvalNumberOp(value, s.NameStr, "%.2f", "%.0f", sId, config)
}

func (s *peDynamicSignal) getValue(snap *model.StockDailySnapshot) float64 {
	if snap != nil && snap.PEDynamic != 0 {
		return snap.PEDynamic
	}
	return 0
}

type peStaticSignal struct {
	indicator.BaseSignal
}

func (s *peStaticSignal) Evaluate(snap *model.StockDailySnapshot, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	value := s.getValue(snap)
	return signalutil.EvalNumberOp(value, s.NameStr, "%.2f", "%.0f", sId, config)
}

func (s *peStaticSignal) getValue(snap *model.StockDailySnapshot) float64 {
	if snap != nil && snap.PEStatic != 0 {
		return snap.PEStatic
	}
	return 0
}
