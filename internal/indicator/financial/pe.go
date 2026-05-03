package financial

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
)

// ============================================================================
//  PETTM — 市盈率(TTM) (数值型)
//  ID: 040001 = CatCodeFinancial("04") + IndPETTMSeq("001")
//  数据源: StockDailySnapshot.PETTM (float64, 单位: 倍)
// ============================================================================

type PETTM struct {
	indicator.BaseIndicator
}

var peDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于0", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 0},
	{Seq: "02", Desc: "0~25", Operator: indicator.OpBetween, MinThreshold: 0, MaxThreshold: 25},
	{Seq: "03", Desc: "25~50", Operator: indicator.OpBetween, MinThreshold: 25, MaxThreshold: 50},
	{Seq: "04", Desc: "大于50", Operator: indicator.OpGT, MinThreshold: 50, MaxThreshold: 0},
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

	builtInSigs := signalutil.BuildRangeSignals(peDefs, "市盈率(TTM)", numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &peTTMSignal{bs}
	})
	p.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", "市盈率(TTM)", "自定义市盈率(TTM)筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpBetween, Params: map[string]any{indicator.ParamKeyMin: 0, indicator.ParamKeyMax: 30}})
	cs2 := indicator.NewBaseSignal("02", "市盈率(动)", "自定义市盈率(动)筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpBetween, Params: map[string]any{indicator.ParamKeyMin: 0, indicator.ParamKeyMax: 30}})
	cs3 := indicator.NewBaseSignal("03", "市盈率(静)", "自定义市盈率(静)筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpBetween, Params: map[string]any{indicator.ParamKeyMin: 0, indicator.ParamKeyMax: 30}})

	p.SetCustomSignals([]indicator.Signal{
		&peTTMSignal{cs1}, &peDynamicSignal{cs2}, &peStaticSignal{cs3},
	})
	return p
}

func (p *PETTM) Evaluate(stock *indicator.StockData, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: "未配置信号"}
	}

	if stock.DailySnapshot != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: "数据缺失: PE_TTM"}
	}

	for _, v := range configs {
		if s, ok := p.Signal[v.SignalID]; ok {
			var res *indicator.EvaluatedStock
			switch ss := s.(type) {
			case *peTTMSignal:
				res = ss.Evaluate(stock, v)
			case *peDynamicSignal:
				res = ss.Evaluate(stock, v)
			case *peStaticSignal:
				res = ss.Evaluate(stock, v)
			default:
				return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: v.SignalID, Message: "不支持的信号"}
			}
			if res.Result == indicator.ResultPassed {
				continue
			} else {
				return res
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: v.SignalID, Message: "不支持的信号"}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

type peTTMSignal struct {
	indicator.BaseSignal
}

func (s *peTTMSignal) Evaluate(stock *indicator.StockData, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	value := s.getValue(stock)
	return signalutil.EvalNumberOp(value, s.NameStr, "%.2f", "%.0f", sId, config)
}

func (s *peTTMSignal) getValue(stock *indicator.StockData) float64 {
	if stock.DailySnapshot != nil && stock.DailySnapshot.PETTM != 0 {
		return stock.DailySnapshot.PETTM
	}
	if stock.DailySnapshot != nil && stock.DailySnapshot.PEStatic != 0 {
		return stock.DailySnapshot.PEStatic
	}
	return 0
}

type peDynamicSignal struct {
	indicator.BaseSignal
}

func (s *peDynamicSignal) Evaluate(stock *indicator.StockData, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	value := s.getValue(stock)
	return signalutil.EvalNumberOp(value, s.NameStr, "%.2f", "%.0f", sId, config)
}

func (s *peDynamicSignal) getValue(stock *indicator.StockData) float64 {
	if stock.DailySnapshot != nil && stock.DailySnapshot.PEDynamic != 0 {
		return stock.DailySnapshot.PEDynamic
	}
	return 0
}

type peStaticSignal struct {
	indicator.BaseSignal
}

func (s *peStaticSignal) Evaluate(stock *indicator.StockData, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	value := s.getValue(stock)
	return signalutil.EvalNumberOp(value, s.NameStr, "%.2f", "%.0f", sId, config)
}

func (s *peStaticSignal) getValue(stock *indicator.StockData) float64 {
	if stock.DailySnapshot != nil && stock.DailySnapshot.PEStatic != 0 {
		return stock.DailySnapshot.PEStatic
	}
	return 0
}
