package financial

import (
	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

// ============================================================================
//  DebtRatio — 资产负债率(%) (数值型)
//  ID: 04010 = CatCodeFinancial("04") + IndDebtRatioSeq("010")
//  数据源: StockDailySnapshot.DebtRatio (float64, 单位: %)
//  公式: 总负债 / 总资产 × 100%
//  注: 金融行业（银行/保险/券商）负债率天然偏高，使用时需注意行业差异
// ============================================================================

type DebtRatio struct {
	indicator.BaseIndicator
}

var debtRatioDefs = []signalutil.RangeDef{
	{Seq: "01", Desc: "小于20", Alias: "低负债", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 20},
	{Seq: "02", Desc: "20~40", Alias: "适中", Operator: indicator.OpBetween, MinThreshold: 20, MaxThreshold: 40},
	{Seq: "03", Desc: "大于40", Alias: "高负债", Operator: indicator.OpBetween, MinThreshold: 40, MaxThreshold: 0},
}

func NewDebtRatio() *DebtRatio {
	dr := &DebtRatio{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndDebtRatioSeq,
			NameStr:     "资产负债率",
			CategoryVal: indicator.CatFinancial,
			Desc:        "资产负债率 = 总负债 / 总资产 × 100%",
			UnitStr:     "%",
		},
	}

	numberOps := signalutil.NumberOpsByUnit(dr.UnitStr)

	indicatorName := "资产负债率"
	builtInSigs := signalutil.BuildRangeSignals(debtRatioDefs, indicatorName, numberOps, func(bs indicator.BaseSignal) indicator.Signal {
		return &debtRatioSignal{BaseSignal: bs}
	})
	dr.SetBuiltInSignals(builtInSigs)

	cs1 := indicator.NewBaseSignal("01", indicatorName, "自定义资产负债率筛选条件", indicator.ValNumber, numberOps,
		&indicator.SignalConfig{Operator: indicator.OpLT, Params: map[string]any{indicator.ParamKeyThreshold: 60.0}})
	dr.SetCustomSignals([]indicator.Signal{&debtRatioSignal{BaseSignal: cs1}})
	return dr
}

func (d *DebtRatio) Evaluate(stock indicator.StockSource, configs []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(configs) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	snap, err := stock.GetDailySnapshot()
	if err != nil || snap == nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: configs[0].SignalID, Message: indicator.DataEmptyError("DebtRatio").Error()}
	}
	value := d.getValue(snap)
	for _, v := range configs {
		if s, ok := d.Signal[v.SignalID]; ok {
			if ss, ok := s.(*debtRatioSignal); ok {
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

func (d *DebtRatio) getValue(snap *model.StockDailySnapshot) float64 {
	if snap != nil && snap.DebtRatio != 0 {
		return snap.DebtRatio
	}
	return 0
}

type debtRatioSignal struct {
	indicator.BaseSignal
}

func (s *debtRatioSignal) Evaluate(value float64, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	return signalutil.EvalNumberOp(value, "资产负债率", "%.2f%%", "%.0f", sId, config)
}
