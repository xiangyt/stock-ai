package financial

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  DeductNetProfit — 扣非净利润(亿元) (数值型)
//  ID: 04012 = CatCodeFinancial("04") + IndDeductNetProfitSeq("012")
//  数据源: StockDailySnapshot.DeductNetProfit (float64, 单位: 元)
//  显示单位: 亿元
//  公式: 净利润 - 非经常性损益（最新一期财报）
// ============================================================================

type DeductNetProfit struct {
	indicator.BaseIndicator
}

var deductNetProfitDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于0", Alias: "亏损", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 0},
	{Seq: "02", Desc: "0~10亿", Alias: "薄利", Operator: indicator.OpBetween, MinThreshold: 0, MaxThreshold: 10},
	{Seq: "03", Desc: "10~100亿", Alias: "中利", Operator: indicator.OpBetween, MinThreshold: 10, MaxThreshold: 100},
	{Seq: "04", Desc: "大于100亿", Alias: "厚利", Operator: indicator.OpGT, MinThreshold: 100, MaxThreshold: 0},
}

func NewDeductNetProfit() *DeductNetProfit {
	d := &DeductNetProfit{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndDeductNetProfitSeq,
			NameStr:     "扣非净利润",
			CategoryVal: indicator.CatFinancial,
			Desc:        "扣除非经常性损益后的净利润（最新一期财报）",
			UnitStr:     "亿元",
		},
	}

	numberOps := signalutil.NumberOpsByUnitMin(d.UnitStr, 0)

	indicatorName := d.NameStr
	builtInSigs := signalutil.BuildRangeSignals(deductNetProfitDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &deductNetProfitSignal{BaseSignal: bs}
	})
	d.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", indicatorName, "自定义扣非净利润筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 5.0}})
	d.SetCustomSignals([]indicator.Signal{&deductNetProfitSignal{BaseSignal: cs1}})
	return d
}

func (d *DeductNetProfit) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("deduct_net_profit").Error()}
	}
	value := d.getValue(snap)

	for _, v := range configs {
		if s, ok := d.Signal[v.SignalID]; ok {
			if ss, ok := s.(*deductNetProfitSignal); ok {
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

func (d *DeductNetProfit) getValue(snap *model.StockDailySnapshot) float64 {
	if snap == nil || snap.DeductNetProfit == 0 {
		return 0
	}
	return snap.DeductNetProfit / 1e8 // 元 → 亿元
}

type deductNetProfitSignal struct {
	indicator.BaseSignal
}

func (s *deductNetProfitSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "扣非净利润", "%.2f亿", "%.0f亿", sId, config)
}
