package screener_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"stock-ai/internal/model"
	"stock-ai/internal/screener"
)

func TestFreeCombo(t *testing.T) {
	fmt.Println("===== 1. 数值型指标: PE(TTM) 的操作符 =====")
	peOps, ok := screener.GetIndicatorOperators("pe_ttm")
	if !ok { t.Fatal("pe_ttm not found") }
	fmt.Printf("  指标: %s (类型=%s, 单位=%s)\n", peOps.Name, peOps.ValueType, peOps.Unit)
	for _, op := range peOps.Operators {
		fmt.Printf("  [%s] %s — 例: %s (参数数: %d)\n", op.Symbol, op.Label, op.Example, len(op.Params))
	}

	fmt.Println("\n===== 2. 序列型指标: MACD交叉 =====")
	macdOps, _ := screener.GetIndicatorOperators("macd_cross")
	fmt.Printf("  指标: %s (类型=%s) 操作符数=%d\n", macdOps.Name, macdOps.ValueType, len(macdOps.Operators))

	fmt.Println("\n===== 3. BuildSignal: PE 在 20~50 之间 =====")
	signal, err := screener.BuildSignal("pe_ttm", model.OpBetween, map[string]interface{}{"min_value": 20.0, "max_value": 50.0})
	if err != nil { t.Fatal(err) }
	b, _ := json.MarshalIndent(signal, "", "  ")
	fmt.Printf("%s\n描述: %s\n", string(b), signal.HumanReadableDescription())

	fmt.Println("\n===== 4. BuildSignal: MACD 近3天金叉 =====")
	sig2, err := screener.BuildSignal("macd_cross", model.OpCrossUp, map[string]interface{}{"within_days": 3})
	if err != nil { t.Fatal(err) }
	b2, _ := json.MarshalIndent(sig2, "", "  ")
	fmt.Printf("%s\n描述: %s\n", string(b2), sig2.HumanReadableDescription())

	fmt.Println("\n===== 5. 全量指标树 =====")
	all := screener.GetAllIndicatorsWithOperators()
	totalInds, totalOps := 0, 0
	for cat, inds := range all {
		fmt.Printf("  【%s】%d 个指标\n", cat.DisplayName(), len(inds))
		totalInds += len(inds)
		for _, ind := range inds {
			totalOps += len(ind.Operators)
			fmt.Printf("    %-12s %-8s → %d ops\n", ind.ID, ind.ValueType, len(ind.Operators))
		}
	}
	fmt.Printf("\n总计: %d 个指标, %d 个操作符选项 ✅\n", totalInds, totalOps)
}
