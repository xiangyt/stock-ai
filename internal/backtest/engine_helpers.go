package backtest

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	"stock-ai/internal/indicator"
	"stock-ai/internal/model"
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
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, err
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, err
	}

	holidays := getHolidaySet()
	var days []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		// 排除周末
		wd := d.Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			continue
		}
		dateStr := d.Format("2006-01-02")
		// 排除法定节假日
		if holidays[dateStr] {
			continue
		}
		days = append(days, dateStr)
	}
	return days, nil
}

// addTradingDays 从给定日期往后加 N 个交易日，返回新日期字符串
func addTradingDays(dateStr string, n int) (string, error) {
	holidays := getHolidaySet()
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return "", err
	}
	added := 0
	for added < n {
		t = t.AddDate(0, 0, 1)
		wd := t.Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			continue
		}
		if holidays[t.Format("2006-01-02")] {
			continue
		}
		added++
	}
	return t.Format("2006-01-02"), nil
}

// getHolidaySet 从 utils 节假日列表构建集合（日期格式统一为 2006-01-02）
func getHolidaySet() map[string]bool {
	holidays := getHolidays()
	set := make(map[string]bool, len(holidays))
	for _, h := range holidays {
		set[h] = true
	}
	return set
}

// getHolidays 返回 A 股法定节假日（与 utils 保持一致）
func getHolidays() []string {
	return []string{
		// 2025
		"2025-01-01",
		"2025-01-28", "2025-01-29", "2025-01-30", "2025-01-31",
		"2025-02-03", "2025-02-04",
		"2025-04-04",
		"2025-05-01", "2025-05-02", "2025-05-05",
		"2025-05-31", "2025-06-02",
		"2025-10-01", "2025-10-02", "2025-10-03", "2025-10-06",
		"2025-10-07", "2025-10-08",
		// 2026
		"2026-01-01", "2026-01-02",
		"2026-02-16", "2026-02-17", "2026-02-18",
		"2026-04-05",
		"2026-05-01", "2026-05-02", "2026-05-03", "2026-05-04", "2026-05-05",
		"2026-06-19",
		"2026-09-25",
		"2026-10-01", "2026-10-02", "2026-10-05", "2026-10-06", "2026-10-07",
	}
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

// calcStampTax 计算 A 股卖出印花税（成交金额 × 0.1%）
func calcStampTax(amount float64) float64 {
	tax := amount * 0.001
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
