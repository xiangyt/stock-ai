package main

import (
	"fmt"
	"math"
	"os"

	"stock-ai/internal/backtest/indicator/technical"
	"stock-ai/internal/config"
	"stock-ai/internal/db"
	"stock-ai/utils"
)

func main() {
	cfg, _ := config.Load("config.yaml")
	db.Init(&cfg.Database)
	defer db.Close()

	code := "300037"
	if len(os.Args) > 1 {
		code = os.Args[1]
	}
	klineCount := 100000
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &klineCount)
	}
	klines, _ := db.FindDailyKlines(code, utils.TodayTradeDate(), klineCount)
	fmt.Printf("%s: %d K线\n", code, len(klines))

	r := technical.BuildCYQ(klines, technical.CyqAccuracyFactor)
	last := len(r.ProfitRatio) - 1
	fmt.Printf("Cost90: %.2f ~ %.2f (宽%.2f)\n",
		r.Cost90[last][0], r.Cost90[last][1], r.Cost90[last][1]-r.Cost90[last][0])

	data := r.XData
	n := len(data)

	// 全局最大值
	maxVal := 0.0
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}
	sigRatio := 0.15
	sigThreshold := maxVal * sigRatio
	halfHeight := 0.5 // cyqPeakWidthRatio
	minPeakWidth := 3
	valleyRatio := 0.55 // cyqValleyDepthRatio
	minDist := 6         // cyqMinPeakDistance

	fmt.Printf("\n全局最高=%.4f, 显著性阈值(%.0f%%)=%.4f\n", maxVal, sigRatio*100, sigThreshold)

	// ====== 阶段1：显著性 ======
	type candidate struct{ Idx int; Height float64 }
	var cands []candidate
	for i := 1; i < n-1; i++ {
		if data[i] > data[i-1] && data[i] > data[i+1] && data[i] >= sigThreshold {
			cands = append(cands, candidate{i, data[i]})
		}
	}
	if n >= 2 {
		if data[0] > data[1] && data[0] >= sigThreshold { cands = append(cands, candidate{0, data[0]}) }
		ln := n - 1
		if data[ln] > data[ln-1] && data[ln] >= sigThreshold { cands = append(cands, candidate{ln, data[ln]}) }
	}
	fmt.Printf("\n===== 阶段1(显著性>= %.0f%%): %d 个候选峰 =====\n", sigRatio*100, len(cands))
	for _, c := range cands {
		pct := c.Height / maxVal * 100
		fmt.Printf("  [%3d] 价格=%.2f 高度=%.4f (%5.1f%%)\n", c.Idx, r.YData[c.Idx], c.Height, pct)
	}

	// ====== 阶段2：峰宽过滤 ======
	type qualifiedPeak struct {
		Idx       int
		Height    float64
		LeftEdge  int
		RightEdge int
		Width     int
	}
	var peaks []qualifiedPeak
	for _, c := range cands {
		left, right := c.Idx, c.Idx
		for left > 0 && data[left-1] >= c.Height*halfHeight {
			left--
		}
		for right < n-1 && data[right+1] >= c.Height*halfHeight {
			right++
		}
		width := right - left + 1
		mark := " ✅"
		if width < minPeakWidth { mark = " ❌ 过窄" } else { peaks = append(peaks, qualifiedPeak{c.Idx, c.Height, left, right, width}) }
		fmt.Printf("  [%3d] 峰宽: Left=%3d Right=%3d Width=%d%s\n", c.Idx, left, right, width, mark)
	}
	fmt.Printf("\n===== 阶段2(峰宽>=%d): %d 峰通过 =====\n", minPeakWidth, len(peaks))

	if len(peaks) <= 1 {
		fmt.Println("<=1 峰，跳过谷深过滤")
		return
	}

	// ====== 阶段2.5：重叠区按谷底切分 ======
	// 按位置排序
	for i := 0; i < len(peaks); i++ {
		for j := i + 1; j < len(peaks); j++ {
			if peaks[j].Idx < peaks[i].Idx { peaks[i], peaks[j] = peaks[j], peaks[i] }
		}
	}
	fmt.Printf("\n===== 阶段2.5(重叠区按谷底切分) =====\n")
	for i := 1; i < len(peaks); i++ {
		if peaks[i-1].RightEdge >= peaks[i].LeftEdge {
			valleyIdx := peaks[i].LeftEdge
			minVal := data[valleyIdx]
			for j := peaks[i-1].RightEdge + 1; j < peaks[i].LeftEdge; j++ {
				if data[j] < minVal { minVal = data[j]; valleyIdx = j }
			}
			oldPrevR, oldCurrL := peaks[i-1].RightEdge, peaks[i].LeftEdge
			peaks[i-1].RightEdge = valleyIdx - 1
			peaks[i].LeftEdge = valleyIdx + 1
			fmt.Printf("  [%3d] RightEdge %d→%d | [%3d] LeftEdge %d→%d | 谷底[%d]=%.4f (%.1f%%)\n",
				peaks[i-1].Idx, oldPrevR, peaks[i-1].RightEdge,
				peaks[i].Idx, oldCurrL, peaks[i].LeftEdge,
				valleyIdx, minVal, minVal/maxVal*100)
		} else {
			fmt.Printf("  [%3d] vs [%3d]: 无重叠 (%d < %d)\n",
				peaks[i-1].Idx, peaks[i].Idx, peaks[i-1].RightEdge, peaks[i].LeftEdge)
		}
	}
	// 重算宽度
	for i := range peaks { peaks[i].Width = peaks[i].RightEdge - peaks[i].LeftEdge + 1 }

	// ====== 阶段3：谷深过滤 + 最小峰距合并 ======
	// 按位置排序
	for i := 0; i < len(peaks); i++ {
		for j := i + 1; j < len(peaks); j++ {
			if peaks[j].Idx < peaks[i].Idx { peaks[i], peaks[j] = peaks[j], peaks[i] }
		}
	}

	filtered := make([]qualifiedPeak, 0, len(peaks))
	filtered = append(filtered, peaks[0])
	fmt.Printf("\n===== 阶段3(谷深< %.0f%% + 峰距<%d 合并) =====\n", valleyRatio*100, minDist)
	fmt.Printf("  初始保留: [%d] 价格=%.2f\n", peaks[0].Idx, r.YData[peaks[0].Idx])

	for i := 1; i < len(peaks); i++ {
		prev := filtered[len(filtered)-1]
		curr := peaks[i]

		dist := curr.Idx - prev.Idx

		// 峰距检查
		if dist < minDist {
			action := "丢弃(低)"
			if curr.Height > prev.Height {
				filtered[len(filtered)-1] = curr
				action = "替换(更高)"
			}
			fmt.Printf("  [%3d] vs [%3d]: 距离=%d < %d → %s (合并)\n",
				curr.Idx, prev.Idx, dist, minDist, action)
			continue
		}

		// 找谷底
		valleyMin := data[curr.LeftEdge]
		for j := prev.RightEdge + 1; j < curr.LeftEdge; j++ {
			if data[j] < valleyMin { valleyMin = data[j] }
		}
		lowerH := prev.Height
		if curr.Height < lowerH { lowerH = curr.Height }
		valleyThreshold := lowerH * valleyRatio

		passed := valleyMin <= valleyThreshold
		status := "✅ 独立"
		action := "保留"
		if !passed {
			status = "❌ 谷太浅"
			if curr.Height > filtered[len(filtered)-1].Height {
				filtered[len(filtered)-1] = curr
				action = "替换(更高)"
			} else {
				action = "丢弃"
			}
		} else {
			filtered = append(filtered, curr)
		}

		fmt.Printf("  [%3d] vs [%3d]: 距离=%d, 谷底=%.4f(阈值%.4f), 较低峰高=%.4f → %s %s\n",
			curr.Idx, prev.Idx, dist, valleyMin, valleyThreshold, lowerH, status, action)
	}

	fmt.Printf("\n===== 最终结果: %d 峰 =====\n", len(filtered))
	for _, p := range filtered {
		fmt.Printf("  [%3d] 价格=%.2f 高度=%.4f (%5.1f%%)\n", p.Idx, r.YData[p.Idx], p.Height, p.Height/maxVal*100)
	}

	// 双峰判定
	if len(filtered) == 2 {
		h0, h1 := filtered[0].Height, filtered[1].Height
		maxH, minH := h0, h1
		if h1 > h0 { maxH, minH = h1, h0 } else if h1 < h0 { minH = h1 }
		diffRatio := 0.0
		if maxH > 0 { diffRatio = (maxH - minH) / maxH }

		p0, p1 := r.YData[filtered[0].Idx], r.YData[filtered[1].Idx]
		lowP, highP := p0, p1
		if p1 < p0 { lowP, highP = p1, p0 }
		costRange := r.Cost90[last][1] - r.Cost90[last][0]
		d := math.Abs(highP - lowP)

		fmt.Printf("\n=== 双峰密集判定 ===\n")
		fmt.Printf("  峰距: %.2f 元, 阈值(宽×50%%): %.2f → %v\n", d, costRange*0.5, d > costRange*0.5)
		fmt.Printf("  峰高差比: (%.4f-%.4f)/%.4f = %.4f (%.1f%%), 阈值<20%% → %v\n",
			maxH, minH, maxH, diffRatio, diffRatio*100, diffRatio < 0.20)
	}
}
