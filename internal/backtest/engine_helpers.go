package backtest

import (
	"encoding/json"
	"math"
	"sort"
	"strings"

	"stock-ai/internal/indicator"
	"stock-ai/internal/model"
	"stock-ai/utils"
)

// ============================================================================
//  回测引擎辅助 — 交易日计算 / 序列化 / 费用 / 工厂函数
// ============================================================================

// newExitCheckerChain 创建默认卖出规则优先级链
func newExitCheckerChain() exitCheckerChain {
	return exitCheckerChain{&StopLossChecker{}, &TakeProfitChecker{}, &TimeExitChecker{}}
}

// =========================== 交易日 ===========================

// getTradingDays 返回 [startDate, endDate] 区间内所有 A 股交易日（日期字符串）
func getTradingDays(startDate, endDate string) ([]string, error) {
	return utils.GetTradingDays(startDate, endDate)
}

// addTradingDays 从给定日期往后加 N 个交易日，返回新日期字符串
func addTradingDays(dateStr string, n int) (string, error) {
	return utils.AddTradingDays(dateStr, n)
}

// =========================== 序列化 ===========================

func mustMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// =========================== 费用计算 ===========================

// calcCommission 计算买入/卖出佣金（单边）
// commissionRate: 万分之x（如 2.5 = 万分之2.5）
// minCommission: true = 不免五（最低5元）
func calcCommission(amount, commissionRate float64, minCommission bool) float64 {
	fee := amount * commissionRate / 10000.0
	fee = math.Round(fee*10000) / 10000
	if minCommission && fee < 5.0 {
		fee = 5.0
	}
	return fee
}

// calcStampTax 计算 A 股卖出印花税（成交金额 × 0.05%，仅卖出时收取）
func calcStampTax(amount float64) float64 {
	tax := amount * 0.0005
	return math.Round(tax*10000) / 10000
}

// =========================== 工具函数 ===========================

// parseSignals 解析策略的 Conditions JSON 为 []indicator.SignalConfig
func parseSignals(conditionsJSON string) ([]*indicator.SignalConfig, error) {
	var signals []*indicator.SignalConfig
	if err := json.Unmarshal([]byte(conditionsJSON), &signals); err != nil {
		return nil, err
	}
	return signals, nil
}

// sortedStockCodes 按代码排序（用于稳定输出）
func sortedStockCodes(codes map[string]bool) []string {
	var list []string
	for c := range codes {
		list = append(list, c)
	}
	sort.Strings(list)
	return list
}

// =========================== 策略条件引用 ===========================

// strategyConditions 存储回测用策略条件
type strategyConditions struct {
	Conditions    []*indicator.SignalConfig
	LogicalOp     model.LogicalOp
	ExitRules     *model.ExitRules
	PositionRules *model.PositionRules
}

// parseStrategyRules 从 strategy 解析出回测所需条件
func parseStrategyRules(strategy *model.Strategy) (*strategyConditions, error) {
	sc := &strategyConditions{
		LogicalOp:     strategy.LogicalOp,
		ExitRules:     &model.ExitRules{},
		PositionRules: &model.PositionRules{},
	}

	// 解析买入条件
	if strings.TrimSpace(strategy.Conditions) != "" {
		var configs []*indicator.SignalConfig
		if err := json.Unmarshal([]byte(strategy.Conditions), &configs); err != nil {
			return nil, err
		}
		sc.Conditions = configs
	}

	// 解析卖出规则
	if strings.TrimSpace(strategy.ExitRules) != "" {
		if err := json.Unmarshal([]byte(strategy.ExitRules), sc.ExitRules); err != nil {
			return nil, err
		}
	}

	// 解析仓位规则
	if strings.TrimSpace(strategy.PositionRules) != "" {
		if err := json.Unmarshal([]byte(strategy.PositionRules), sc.PositionRules); err != nil {
			return nil, err
		}
	}

	return sc, nil
}
