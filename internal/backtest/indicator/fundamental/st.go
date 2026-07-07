package fundamental

import (
	"strings"

	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
)

// ============================================================================
//  IsSt — 是否为ST股 (枚举型)
//  ID: 03011 = CatCodeFundamental("03") + IndStSeq("011")
//  数据源: GetDetail().Name（股票简称），后期通过快照数据判断
//
//  判断逻辑: 股票简称包含"ST"即视为ST股（含*ST、ST等前缀变体）
// ============================================================================

// StOptions ST状态枚举选项
var StOptions = []indicator.EnumOption{
	{Value: "st", Label: "是ST股"},
	{Value: "normal", Label: "非ST股"},
}

// stDefs 内置ST股信号定义表
var stDefs = []signalutil.EnumDef{
	{Seq: "01", Desc: "是ST股", Alias: "ST", Operator: indicator.OpEQ, Value: "st"},
	{Seq: "02", Desc: "非ST股", Alias: "非ST", Operator: indicator.OpEQ, Value: "normal"},
}

// IsSt ST股判断指标
type IsSt struct {
	indicator.BaseIndicator
}

// NewIsSt 创建ST股指标实例
func NewIsSt() *IsSt {
	s := &IsSt{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndStSeq,
			NameStr:     "ST股",
			CategoryVal: indicator.CatFundamental,
			Desc:        "判断股票是否为ST股（暂时通过简称判断，后期通过快照数据）",
			UnitStr:     "",
		},
	}

	ops := signalutil.EnumOps(StOptions)
	indicatorName := s.NameStr

	// 内置信号：是ST股 / 非ST股
	builtInSigs := signalutil.BuildEnumSignals(stDefs, indicatorName, ops, func(bs indicator.BaseSignal) indicator.Signal {
		return &stSignal{BaseSignal: bs}
	})
	s.SetBuiltInSignals(builtInSigs)

	return s
}

// Evaluate 评估股票是否为ST股
func (s *IsSt) Evaluate(stock indicator.StockSource, config []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(config) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	// 取得股票详情，通过简称判断ST（后期通过快照数据判断）
	detail, err := stock.GetDetail()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config[0].SignalID, Message: err.Error()}
	}

	stVal := "normal"
	if isStStock(detail.Name) {
		stVal = "st"
	}

	for _, v := range config {
		if sig, ok := s.Signal[v.SignalID]; ok {
			switch vv := sig.(type) {
			case *stSignal:
				if res := vv.Evaluate(stVal, v); res.Result == indicator.ResultPassed {
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

// stSignal ST股信号
type stSignal struct {
	indicator.BaseSignal
}

// Evaluate 委托给 signalutil.EvalEnumOp 统一评估
func (s *stSignal) Evaluate(stVal string, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	signalID := config.SignalID
	if !config.IsCustom() {
		signalID = s.DefaultConfig().SignalID
	}
	return signalutil.EvalEnumOp(stVal, "ST状态", signalID, config)
}

// isStStock 通过股票简称判断是否为ST股。
//
// 包含"ST"子串即视为ST股，匹配 *ST、ST、SST 等变体。
// 后期改为通过快照数据的 is_st 字段判断。
func isStStock(name string) bool {
	return strings.Contains(name, "ST")
}
