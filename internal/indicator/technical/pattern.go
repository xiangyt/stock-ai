package technical

import (
	"stock-ai/internal/indicator"
	"stock-ai/internal/model"
)

// ============================================================================
//  Pattern — 形态 (序列型)
//  ID: 01006 = CatCodeMarket("01") + IndVolumeSeq("006")
//  数据源: GetDailyKline()
// ============================================================================

type Pattern struct {
	indicator.BaseIndicator
}

func NewPattern() *Pattern {
	i := &Pattern{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndPatternSeq,
			NameStr:     "形态",
			CategoryVal: indicator.CatTechnical,
			Desc:        "各种自定义技术指标",
			UnitStr:     "",
		},
	}

	i.SetBuiltInSignals([]indicator.Signal{
		NewSignal513(),
	})

	i.SetCustomSignals(nil)
	return i
}

func (i *Pattern) Evaluate(stock indicator.StockSource, config []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(config) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config[0].SignalID,
			Message: err.Error()}
	}

	for _, v := range config {
		if s, ok := i.Signal[v.SignalID]; ok {
			switch vv := s.(type) {
			case *Signal513:
				if res := vv.Evaluate(klines, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: v.SignalID,
			Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

// 513战法
type Signal513 struct {
	indicator.BaseSignal
}

func NewSignal513() *Signal513 {
	return &Signal513{
		BaseSignal: indicator.NewBaseSignal(
			"01",
			"513战法",
			"513战法",
			indicator.ValNumber,
			[]indicator.OperatorOption{},
			nil,
		),
	}
}

// Evaluate ...
func (s *Signal513) Evaluate(klines []*model.DailyKline, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	var key1, key2 = 5, 2
	if len(klines) < (key1 + key2 + 1) {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
			Message: indicator.DataEmptyError("513 kline").Error()}
	}

	if klines[key2].Volume > klines[key2+1].Volume*2 { // key2天前那天成交量大于2倍4天前成交量
		maxRate := float64((klines[key2].High - klines[key2+1].Close)) / float64(klines[key2+1].Close) * 100
		rate1 := float64((klines[key2].High - klines[key2].Close)) / float64(klines[key2+1].Close) * 100
		rate2 := float64((klines[key2].Close - klines[key2].Open)) / float64(klines[key2+1].Close) * 100
		if !(maxRate > 5 && rate1 > 1 && rate2 > 3) { //异动当天最高至少涨8%，回落超过1%，阳线高度至少3%
			return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
				Message: "未出现异动阳线"}
		}
	} else {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
			Message: "未出现异动阳线"}
	}
	for i := key2; i < key2+key1; i++ {
		if !(klines[i].Close >= klines[i].Open && klines[i].Close >= klines[i+1].Close) {
			return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: "至少5连阳"}
		}
	}
	for i := 0; i < key2; i++ {
		if klines[i].Open < klines[key2].Open {
			return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
				Message: "股价（收盘价）不能跌破那根异动阳线的开盘价"}
		} else if klines[i].Volume > klines[key2].Volume {
			return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
				Message: "成交量必须明显萎缩，不能超过异动阳线当天的量"}
		} else {
			rate := float64((klines[i].High - klines[i].Low)) / float64(klines[i+1].Close) * 100
			if rate >= 5 {
				return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
					Message: "盘面不能剧烈波动"}
			}
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}
