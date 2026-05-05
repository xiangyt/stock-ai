package technical

import (
	"stock-ai/internal/indicator"
	"stock-ai/internal/model"
)

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
	if len(klines) < 5 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
			Message: indicator.DataEmptyError("513 kline").Error()}
	}

	var reds = make([]*model.DailyKline, 0, 7)
	var index1 int //1根异动倍量阳线
	for i, v := range klines {
		if v.Close > v.Open {
			reds = append(reds, v)
		} else {
			break
		}
		if len(reds) > 2 && reds[i-1].Volume > reds[i].Volume*2 { // 前一天成交量大于2倍当日成交量
			maxRate := float64((reds[i-1].High - reds[i].Close)) / float64(reds[i].Close) * 100
			rate1 := float64((reds[i-1].High - reds[i-1].Close)) / float64(reds[i].Close) * 100
			rate2 := float64((reds[i-1].Close - reds[i-1].Open)) / float64(reds[i].Close) * 100
			if maxRate > 8 && rate1 > 1 && rate2 > 3 { //异动当天最高至少涨8%，回落超过1%，阳线高度至少3%
				index1 = i - 1
			}
		}
	}
	if !(index1 == 3 || index1 == 4) {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
			Message: "在出现那根异动倍量阳线后，接下来的3个交易日是关键的验证期"}
	}
	switch {
	case len(reds) < 5:
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: "至少5连阳"}
	case index1 == 0:
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID, Message: "没有异动"}
	default:
		for i := 0; i < index1; i++ {
			if reds[i].Open < reds[index1].Open {
				return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
					Message: "股价（收盘价）不能跌破那根异动阳线的开盘价"}
			} else if reds[i].Volume > reds[index1].Volume {
				return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
					Message: "成交量必须明显萎缩，不能超过异动阳线当天的量"}
			} else {
				rate := float64((reds[i].High - reds[i].Low)) / float64(reds[i+1].Close) * 100
				if rate > 5 {
					return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config.SignalID,
						Message: "盘面不能剧烈波动"}
				}
			}
		}
	}

	return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
}
