package fundamental

import (
	"stock-ai/internal/indicator"
	signalutil "stock-ai/internal/indicator/signalutil"
)

// ============================================================================
//  枚举选项定义
// ============================================================================

var ListingBoardOptions = []indicator.EnumOption{
	{Value: "main", Label: "主板"},
	{Value: "chinext", Label: "创业板"},
	{Value: "star", Label: "科创板"},
	{Value: "bse", Label: "北交所"},
}

// ============================================================================
//  ListingBoard — 上市板块 (枚举型)
//  ID: 03001 = CatCodeFundamental("03") + IndListingBoardSeq("001")
// ============================================================================

type ListingBoard struct {
	indicator.BaseIndicator
}

// boardDefs 内置上市板块信号定义表
// 新增板块只需在此追加一行，无需修改其他逻辑
var boardDefs = []signalutil.EnumDef{
	{Seq: "01", Desc: "主板", Operator: indicator.OpEQ, Value: "main"},
	{Seq: "02", Desc: "创业板", Operator: indicator.OpEQ, Value: "chinext"},
	{Seq: "03", Desc: "科创板", Operator: indicator.OpEQ, Value: "star"},
	{Seq: "04", Desc: "北交所", Operator: indicator.OpEQ, Value: "bse"},
}

func NewListingBoard() *ListingBoard {
	l := &ListingBoard{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndListingBoardSeq,
			NameStr:     "上市板块",
			CategoryVal: indicator.CatFundamental,
			Desc:        "股票所属上市交易所板块",
			UnitStr:     "",
		},
	}

	ops := signalutil.EnumOps(ListingBoardOptions)

	// 数据驱动生成内置板块信号
	builtInSigs := signalutil.BuildEnumSignals(boardDefs, "上市板块", ops, func(bs indicator.BaseSignal) indicator.Signal {
		return &ListingBoardSignal{bs}
	})
	l.SetBuiltInSignals(builtInSigs)

	// 自定义上市板块分类信号
	industryDefs := []signalutil.EnumDef{
		{Seq: "01", Desc: "自选上市板块", Operator: indicator.OpIn, Values: []string{}},
	}
	customSigs := signalutil.BuildEnumSignals(industryDefs, "上市板块", ops, func(bs indicator.BaseSignal) indicator.Signal {
		return &ListingBoardSignal{bs}
	})
	l.SetCustomSignals(customSigs)

	return l
}

func (l *ListingBoard) Evaluate(stock indicator.StockSource, config []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(config) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	detail, err := stock.GetDetail()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config[0].SignalID, Message: err.Error()}
	}
	var board = detail.ListingBoard
	if board == "" {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config[0].SignalID, Message: indicator.DataEmptyError("listing_board").Error()}
	}

	stockName := stock.GetName()
	for _, v := range config {
		if s, ok := l.Signal[v.SignalID]; ok {
			switch vv := s.(type) {
			case *ListingBoardSignal:
				if res := vv.Evaluate(stockName, board, v); res.Result == indicator.ResultPassed {
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

type ListingBoardSignal struct {
	indicator.BaseSignal
}

// Evaluate 委托给 signalutil.EvalEnumOp 统一评估
func (s *ListingBoardSignal) Evaluate(name, board string, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	signalID := config.SignalID
	if !config.IsCustom() {
		// 内置信号：取默认配置的 SignalID
		signalID = s.DefaultConfig().SignalID
	}
	return signalutil.EvalEnumOp(board, "板块", signalID, config)
}
