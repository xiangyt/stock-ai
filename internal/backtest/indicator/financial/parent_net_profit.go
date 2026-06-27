package financial

import (
	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  ParentNetProfit — 归母净利润(亿元) (数值型)
//  ID: 04011 = CatCodeFinancial("04") + IndParentNetProfitSeq("011")
//  数据源: StockDailySnapshot.ParentNetProfit (float64, 单位: 元)
//  显示单位: 亿元
//  公式: 归属母公司所有者的净利润（最新一期财报）
// ============================================================================

type ParentNetProfit struct {
	indicator.BaseIndicator
}

var parentNetProfitDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于0", Alias: "亏损", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 0},
	{Seq: "02", Desc: "0~10亿", Alias: "薄利", Operator: indicator.OpBetween, MinThreshold: 0, MaxThreshold: 10},
	{Seq: "03", Desc: "10~100亿", Alias: "中利", Operator: indicator.OpBetween, MinThreshold: 10, MaxThreshold: 100},
	{Seq: "04", Desc: "大于100亿", Alias: "厚利", Operator: indicator.OpGT, MinThreshold: 100, MaxThreshold: 0},
}

func NewParentNetProfit() *ParentNetProfit {
	p := &ParentNetProfit{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndParentNetProfitSeq,
			NameStr:     "归母净利润",
			CategoryVal: indicator.CatFinancial,
			Desc:        "归属母公司所有者的净利润（最新一期财报）",
			UnitStr:     "亿元",
		},
	}

	numberOps := signalutil.NumberOpsByUnitMin(p.UnitStr, 0)

	indicatorName := p.NameStr
	builtInSigs := signalutil.BuildRangeSignals(parentNetProfitDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &parentNetProfitSignal{BaseSignal: bs}
	})
	p.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", indicatorName, "自定义归母净利润筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 5.0}})
	p.SetCustomSignals([]indicator.Signal{&parentNetProfitSignal{BaseSignal: cs1}})
	return p
}

func (p *ParentNetProfit) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("parent_net_profit").Error()}
	}
	value := p.getValue(snap)

	for _, v := range configs {
		if s, ok := p.Signal[v.SignalID]; ok {
			if ss, ok := s.(*parentNetProfitSignal); ok {
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

func (p *ParentNetProfit) getValue(snap *model.StockDailySnapshot) float64 {
	if snap == nil || snap.ParentNetProfit == 0 {
		return 0
	}
	return snap.ParentNetProfit / 1e8 // 元 → 亿元
}

type parentNetProfitSignal struct {
	indicator.BaseSignal
}

func (s *parentNetProfitSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "归母净利润", "%.2f亿", "%.0f亿", sId, config)
}
