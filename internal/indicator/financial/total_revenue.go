package financial

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  TotalRevenue — 营业总收入(亿元) (数值型)
//  ID: 04013 = CatCodeFinancial("04") + IndTotalRevenueSeq("013")
//  数据源: StockDailySnapshot.TotalRevenue (float64, 单位: 元)
//  显示单位: 亿元
//  公式: 营业收入 + 利息收入等（最新一期财报）
// ============================================================================

type TotalRevenue struct {
	indicator.BaseIndicator
}

var totalRevenueDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于10亿", Operator: indicator.OpLT, MinThreshold: 10, MaxThreshold: 0},
	{Seq: "02", Desc: "10亿~100亿", Operator: indicator.OpBetween, MinThreshold: 10, MaxThreshold: 100},
	{Seq: "03", Desc: "100亿~1000亿", Operator: indicator.OpBetween, MinThreshold: 100, MaxThreshold: 1000},
	{Seq: "04", Desc: "大于1000亿", Operator: indicator.OpGT, MinThreshold: 1000, MaxThreshold: 0},
}

func NewTotalRevenue() *TotalRevenue {
	t := &TotalRevenue{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndTotalRevenueSeq,
			NameStr:     "营业总收入",
			CategoryVal: indicator.CatFinancial,
			Desc:        "营业总收入（最新一期财报）",
			UnitStr:     "亿元",
		},
	}

	numberOps := signalutil.NumberOpsByUnitMin(t.UnitStr, 0)

	builtInSigs := signalutil.BuildRangeSignals(totalRevenueDefs, t.NameStr, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &totalRevenueSignal{bs}
	})
	t.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", t.NameStr, "自定义营业总收入筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpGT, Params: map[string]any{indicator.ParamKeyThreshold: 50.0}})
	t.SetCustomSignals([]indicator.Signal{&totalRevenueSignal{cs1}})
	return t
}

func (t *TotalRevenue) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("total_revenue").Error()}
	}
	value := t.getValue(snap)

	for _, v := range configs {
		if s, ok := t.Signal[v.SignalID]; ok {
			if ss, ok := s.(*totalRevenueSignal); ok {
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

func (t *TotalRevenue) getValue(snap *model.StockDailySnapshot) float64 {
	if snap == nil || snap.TotalRevenue == 0 {
		return 0
	}
	return snap.TotalRevenue / 1e8 // 元 → 亿元
}

type totalRevenueSignal struct {
	indicator.BaseSignal
}

func (s *totalRevenueSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "营业总收入", "%.2f亿", "%.0f亿", sId, config)
}
