package main

import (
	"fmt"
	"strings"

	"stock-ai/internal/db"
	"stock-ai/internal/model"
)

// compareKlines 对比东财API与DB的日K线数据差异
func compareKlines(code string, date int) {
	if emAdapter == nil {
		return
	}

	// 1. 从东财API获取
	emKlines, emStatus := loadDailyKlinesFromEastMoney(emAdapter, code)
	if emStatus != loadOK {
		fmt.Println("[比对] 东财日K加载失败，跳过比对")
		return
	}

	// 2. 从DB获取
	dbKlines, err := db.FindDailyKlines(code, date, klineLoadLimit)
	if err != nil || len(dbKlines) == 0 {
		fmt.Println("[比对] DB日K加载失败或为空，跳过比对")
		return
	}

	// 3. 构建 date → kline 映射
	emByDate := make(map[int]*model.DailyKline, len(emKlines))
	for _, k := range emKlines {
		emByDate[k.TradeDate] = k
	}
	dbByDate := make(map[int]*model.DailyKline, len(dbKlines))
	for _, k := range dbKlines {
		dbByDate[k.TradeDate] = k
	}

	// 4. 逐日比对
	var diffs []string
	sameCount, diffCount, onlyEM, onlyDB := 0, 0, 0, 0

	// 取所有日期并集
	allDates := make(map[int]bool)
	for d := range emByDate {
		allDates[d] = true
	}
	for d := range dbByDate {
		allDates[d] = true
	}

	// 排序日期（从旧到新，方便查看）
	dates := make([]int, 0, len(allDates))
	for d := range allDates {
		dates = append(dates, d)
	}
	sortDates(dates)

	for _, d := range dates {
		em, emOK := emByDate[d]
		db, dbOK := dbByDate[d]

		switch {
		case emOK && !dbOK:
			onlyEM++
			if onlyEM <= 5 {
				diffs = append(diffs, fmt.Sprintf("  %d  仅东财有  Close=%d", d, em.Close))
			}
		case !emOK && dbOK:
			onlyDB++
			if onlyDB <= 5 {
				diffs = append(diffs, fmt.Sprintf("  %d  仅DB有    Close=%d", d, db.Close))
			}
		case emOK && dbOK:
			// 比较 OHLCV
			var issues []string
			if em.Open != db.Open {
				issues = append(issues, fmt.Sprintf("Open(%d≠%d)", db.Open, em.Open))
			}
			if em.High != db.High {
				issues = append(issues, fmt.Sprintf("High(%d≠%d)", db.High, em.High))
			}
			if em.Low != db.Low {
				issues = append(issues, fmt.Sprintf("Low(%d≠%d)", db.Low, em.Low))
			}
			if em.Close != db.Close {
				issues = append(issues, fmt.Sprintf("Close(%d≠%d)", db.Close, em.Close))
			}
			if em.Volume != db.Volume {
				issues = append(issues, fmt.Sprintf("Vol(%d≠%d)", db.Volume, em.Volume))
			}
			if len(issues) > 0 {
				diffCount++
				if diffCount <= 10 {
					diffs = append(diffs, fmt.Sprintf("  %d  %s", d, strings.Join(issues, ", ")))
				}
			} else {
				sameCount++
			}
		}
	}

	// 5. 输出汇总
	fmt.Println("┌──────────── 东财 vs DB 日K比对 ────────────┐")
	fmt.Printf("│  一致: %-6d  差异: %-6d                    │\n", sameCount, diffCount)
	fmt.Printf("│  仅东财: %-4d  仅DB: %-4d                    │\n", onlyEM, onlyDB)
	if len(diffs) > 0 {
		fmt.Println("├──────────────────────────────────────────────┤")
		for _, line := range diffs {
			fmt.Printf("│%s│\n", line)
		}
		if diffCount > 10 {
			fmt.Printf("│  ... 还有 %d 条差异未列出                      │\n", diffCount-10)
		}
	}
	fmt.Println("└──────────────────────────────────────────────┘")
	fmt.Println()
}

func sortDates(dates []int) {
	for i := 0; i < len(dates)-1; i++ {
		for j := i + 1; j < len(dates); j++ {
			if dates[i] > dates[j] {
				dates[i], dates[j] = dates[j], dates[i]
			}
		}
	}
}
