package fundamental

import (
	"fmt"

	"stock-ai/internal/screener/indicator"
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

var IndustryOptions = []indicator.EnumOption{
	{Value: "银行", Label: "银行"}, {Value: "非银金融", Label: "非银金融"},
	{Value: "医药生物", Label: "医药生物"}, {Value: "食品饮料", Label: "食品饮料"},
	{Value: "家用电器", Label: "家用电器"}, {Value: "电子", Label: "电子"},
	{Value: "计算机", Label: "计算机"}, {Value: "通信", Label: "通信"},
	{Value: "电力设备", Label: "电力设备"}, {Value: "汽车", Label: "汽车"},
	{Value: "机械设备", Label: "机械设备"}, {Value: "国防军工", Label: "国防军工"},
	{Value: "有色金属", Label: "有色金属"}, {Value: "基础化工", Label: "基础化工"},
	{Value: "石油石化", Label: "石油石化"}, {Value: "煤炭", Label: "煤炭"},
	{Value: "钢铁", Label: "钢铁"}, {Value: "建筑材料", Label: "建筑材料"},
	{Value: "建筑装饰", Label: "建筑装饰"}, {Value: "房地产", Label: "房地产"},
	{Value: "轻工制造", Label: "轻工制造"}, {Value: "商贸零售", Label: "商贸零售"},
	{Value: "纺织服饰", Label: "纺织服饰"}, {Value: "社会服务", Label: "社会服务"},
	{Value: "交通运输", Label: "交通运输"}, {Value: "公用事业", Label: "公用事业"},
	{Value: "传媒", Label: "传媒"}, {Value: "农林牧渔", Label: "农林牧渔"},
	{Value: "综合", Label: "综合"},
}

// ============================================================================
//  ListingBoard — 上市板块 (枚举型)
//  ID: 030001 = CatCodeFundamental("03") + IndListingBoardSeq("001")
// ============================================================================

type ListingBoard struct {
	indicator.BaseIndicator
}

// boardDef 定义单个上市板块信号的元数据
type boardDef struct {
	seq   string // 2位信号序号
	name  string // 显示名称
	value string // 枚举值 (对应 model.Stock.ListingBoard)
}

// boardDefs 内置上市板块信号定义表
// 新增板块只需在此追加一行，无需修改其他逻辑
var boardDefs = []boardDef{
	{"01", "主板", "main"},
	{"02", "创业板", "chinext"},
	{"03", "科创板", "star"},
	{"04", "北交所", "bse"},
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
	enumParam := indicator.ParamDef{
		Key: "values", Label: "选择板块", Type: "select_multi",
		Required: true, Options: ListingBoardOptions,
	}
	oo := []indicator.OperatorOption{
		{Operator: indicator.OpIn, Label: "属于", Params: []indicator.ParamDef{enumParam}},
	}

	// 数据驱动生成内置板块信号
	builtInSigs := make([]indicator.Signal, 0, len(boardDefs))
	for _, def := range boardDefs {
		bs := indicator.NewBaseSignal(
			def.seq, def.name,
			fmt.Sprintf("股票属于指定%s", def.name),
			indicator.ValEnum, oo,
			&indicator.SignalConfig{
				Operator: indicator.OpIn,
				Params:   map[string]any{"values": []string{def.value}},
			},
		)
		builtInSigs = append(builtInSigs, &ListingBoardSignal{bs})
	}
	l.SetBuiltInSignals(builtInSigs)

	// 自定义行业分类信号
	cs1 := indicator.NewBaseSignal("01", "行业", "股票属于指定行业", indicator.ValEnum, []indicator.OperatorOption{
		{Operator: indicator.OpIn, Label: "属于", Params: []indicator.ParamDef{enumParam}},
		{Operator: indicator.OpNotIn, Label: "不属于", Params: []indicator.ParamDef{enumParam}},
	}, &indicator.SignalConfig{Operator: indicator.OpIn, Params: map[string]any{"values": []string{}}})
	l.SetCustomSignals([]indicator.Signal{
		&ListingBoardSignal{cs1},
	})
	return l
}

func (l *ListingBoard) Evaluate(stock *indicator.StockData, config []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(config) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: "未配置信号"}
	}
	var (
		board string
	)
	if stock.Detail != nil {
		board = stock.Detail.ListingBoard
	}
	if board == "" {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config[0].SignalID, Message: "数据缺失: listing_board"}
	}

	for _, v := range config {
		if s, ok := l.Signal[v.SignalID]; ok {
			switch vv := s.(type) {
			case *ListingBoardSignal:
				if res := vv.Evaluate(stock.Detail.Name, board, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}

			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: v.SignalID, Message: "不支持的信号"}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

type ListingBoardSignal struct {
	indicator.BaseSignal
}

func (s *ListingBoardSignal) Evaluate(name, board string, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	sId := config.SignalID
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}
	values := indicator.ToStringSlice(config.GetParam("values", []string{}))
	switch config.Operator {
	case indicator.OpIn:
		if indicator.ContainsString(values, board) {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("%s 板块=%s ✓", name, board)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("%s 板块=%s 不在 %v 中", name, board, values)}
	case indicator.OpNotIn:
		if !indicator.ContainsString(values, board) {
			return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: sId, Message: fmt.Sprintf("%s 板块=%s (排除%v) ✓", name, board, values)}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: fmt.Sprintf("%s 板块=%s 在排除列表中", name, board)}
	default:
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: sId, Message: "不支持的操作符"}
	}
}
