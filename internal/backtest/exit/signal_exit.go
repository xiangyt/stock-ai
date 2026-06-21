package exit

import (
	"encoding/json"

	"stock-ai/internal/backtest/types"
)

// ============================================================================
//  SignalExitChecker 信号退出检查器
//  实现: ExitChecker + PriorityChecker + DailyUpdateHook
//  逻辑: 评估指定信号是否触发，触发则卖出
//  信号评估由引擎通过 SetEvaluator 注入回调实现
// ============================================================================

type SignalExitChecker struct {
	signalID  string
	operator  string
	params    map[string]any
	priority  int
	evalFn    SignalEvalFunc
}

// SignalEvalFunc 信号评估回调（由引擎注入）
// code: 股票代码, dateInt: 交易日期整数(YYYYMMDD)
// 返回 true 表示信号触发
type SignalEvalFunc func(signalID, operator string, params map[string]any, code string, dateInt int) bool

type signalExitParams struct {
	SignalID string         `json:"signal_id"`
	Operator string         `json:"operator"`
	Params   map[string]any `json:"params,omitempty"`
}

func NewSignalExitChecker(params json.RawMessage) (*SignalExitChecker, error) {
	var p signalExitParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return &SignalExitChecker{
		signalID: p.SignalID,
		operator: p.Operator,
		params:   p.Params,
		priority: 3, // 信号退出，优先级高于 time_exit
	}, nil
}

func (s *SignalExitChecker) Name() string      { return "signal_exit" }
func (s *SignalExitChecker) Priority() int       { return s.priority }
func (s *SignalExitChecker) SetPriority(p int)    { s.priority = p }

// SetEvaluator 由引擎注入信号评估回调（不依赖 indicator 包）
func (s *SignalExitChecker) SetEvaluator(fn SignalEvalFunc) { s.evalFn = fn }

func (s *SignalExitChecker) Check(pos *types.HoldingPosition, bar types.DayBar) *types.ExitDecision {
	if s.evalFn == nil {
		return nil // 未注入评估器，跳过
	}
	dateInt := parseDateYYYYMMDD(bar.Date)
	if s.evalFn(s.signalID, s.operator, s.params, pos.StockCode, dateInt) {
		return &types.ExitDecision{Reason: "signal_exit", Price: bar.Close}
	}
	return nil
}

// parseDateYYYYMMDD 将 "YYYY-MM-DD" 转为 YYYYMMDD 整数
func parseDateYYYYMMDD(date string) int {
	if len(date) != 10 {
		return 0
	}
	y := int(date[0]-'0')*1000 + int(date[1]-'0')*100 + int(date[2]-'0')*10 + int(date[3]-'0')
	m := int(date[5]-'0')*10 + int(date[6]-'0')
	d := int(date[8]-'0')*10 + int(date[9]-'0')
	return y*10000 + m*100 + d
}

func init() {
	types.RegisterExitChecker("signal_exit", func(params json.RawMessage) (types.ExitChecker, error) {
		return NewSignalExitChecker(params)
	})
}
