package monitor

import (
	"stock-ai/internal/model"
)

// ============================================================================
//  测试辅助函数
// ============================================================================

func makeQuoteData(code string, changePct float64, volume int64) *model.QuoteData {
	return &model.QuoteData{
		Code:      code,
		Name:      "测试股",
		Price:     10.50,
		ChangePct: changePct,
		Volume:    volume,
		Turnover:  2.5,
	}
}

func makeRule(ruleType model.RuleType, paramsJSON string) model.MonitorRule {
	return model.MonitorRule{
		Type:   ruleType,
		Params: []byte(paramsJSON),
	}
}
