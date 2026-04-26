package screener_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"stock-ai/internal/model"
	"stock-ai/internal/screener"
	"stock-ai/internal/screener/indicators"
)

func TestOpRegistry(t *testing.T) {
	fmt.Println("===== 1. 已注册操作符的指标 =====")
	ids := indicators.RegisteredIndicatorIDs()
	fmt.Printf("  已注册 %d 个指标:\n", len(ids))
	for _, id := range ids {
		if ops, ok := indicators.GetRegisteredOperators(id); ok {
			opSyms := ""
			for _, o := range ops {
				opSyms += o.Symbol + " "
			}
			fmt.Printf("    %-18s → [%s] (%d ops)\n", id, opSyms, len(ops))
		}
	}

	fmt.Println("\n===== 2. GetIndicatorOperators (显式注册) =====")
	macdOps, ok := screener.GetIndicatorOperators("macd_cross")
	if !ok { t.Fatal("macd_cross not found") }
	fmt.Printf("  指标: %s, 操作符数: %d\n", macdOps.Name, len(macdOps.Operators))
	for _, op := range macdOps.Operators {
		fmt.Printf("    [%s] %s — %s\n", op.Symbol, op.Label, op.Example)
	}

	fmt.Println("\n===== 3. GetIndicatorOperators (回退到默认) =====")
	peOps, ok := screener.GetIndicatorOperators("pe_ttm")
	if !ok { t.Fatal("pe_ttm not found") }
	fmt.Printf("  指标: %s, 操作符数: %d (回退)\n", peOps.Name, len(peOps.Operators))

	fmt.Println("\n===== 4. BuildSignal: PE 区间 + MACD 金叉 =====")
	sig1, err := screener.BuildSignal("pe_ttm", model.OpBetween, map[string]interface{}{
		"min_value": 20.0,
		"max_value": 50.0,
	})
	if err != nil { t.Fatal(err) }

	sig2, err := screener.BuildSignal("macd_cross", model.OpCrossUp, map[string]interface{}{
		"within_days": 3,
	})
	if err != nil { t.Fatal(err) }

	b1, _ := json.MarshalIndent(sig1, "", "  ")
	b2, _ := json.MarshalIndent(sig2, "", "  ")
	fmt.Printf("PE Signal: %s\n%s\n\n", sig1.HumanReadableDescription(), string(b1))
	fmt.Printf("MACD Signal: %s\n%s\n\n", sig2.HumanReadableDescription(), string(b2))

	fmt.Println("\n===== 5. 全量指标树统计 =====")
	all := screener.GetAllIndicatorsWithOperators()
	totalInds, totalOps, registeredCount := 0, 0, 0
	for cat, inds := range all {
		fmt.Printf("  【%s】%d 个指标\n", cat.DisplayName(), len(inds))
		totalInds += len(inds)
		for _, ind := range inds {
			totalOps += len(ind.Operators)
			if _, reg := indicators.GetRegisteredOperators(ind.ID); reg {
				registeredCount++
			}
		}
	}
	fmt.Printf("\n总计: %d 个指标, %d 个操作符选项 (其中 %d 个显式注册, %d 个回退默认) ✅\n",
		totalInds, totalOps, registeredCount, totalInds-registeredCount)
}
