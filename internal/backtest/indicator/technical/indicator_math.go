package technical

import "math"

// ============================================================================
//  公共数学辅助函数 — 供 technical 包下各指标复用
//
//  所有函数假定 data[0] = 最旧, data[n-1] = 最新 (通达信公式规范)
// ============================================================================

// ref 引用前N周期的数据。REF(data, 1) = 前一天的值。
func ref(data []float64, n int) []float64 {
	result := make([]float64, len(data))
	for i := range data {
		if i < n {
			result[i] = 0
		} else {
			result[i] = data[i-n]
		}
	}
	return result
}

// sma 平滑移动平均。公式: (M * x[i] + (N-M) * prevSMA) / N
func sma(x []float64, n, m int) []float64 {
	result := make([]float64, len(x))
	if len(x) == 0 {
		return result
	}
	result[0] = x[0]
	for i := 1; i < len(x); i++ {
		if i < n {
			sum := 0.0
			for j := 0; j <= i; j++ {
				sum += x[j]
			}
			result[i] = sum / float64(i+1)
		} else {
			result[i] = (float64(m)*x[i] + float64(n-m)*result[i-1]) / float64(n)
		}
	}
	return result
}

// ema 指数移动平均。alpha = 2/(n+1)
func ema(data []float64, n int) []float64 {
	result := make([]float64, len(data))
	if len(data) == 0 {
		return result
	}
	alpha := 2.0 / (float64(n) + 1.0)
	result[0] = data[0]
	for i := 1; i < len(data); i++ {
		result[i] = alpha*data[i] + (1-alpha)*result[i-1]
	}
	return result
}

// ma 简单移动平均
func ma(data []float64, n int) []float64 {
	result := make([]float64, len(data))
	for i := range data {
		if i < n-1 {
			sum := 0.0
			for j := 0; j <= i; j++ {
				sum += data[j]
			}
			result[i] = sum / float64(i+1)
		} else {
			sum := 0.0
			for j := i - n + 1; j <= i; j++ {
				sum += data[j]
			}
			result[i] = sum / float64(n)
		}
	}
	return result
}

// llv N周期内的最小值
func llv(data []float64, n int) []float64 {
	result := make([]float64, len(data))
	for i := range data {
		if i < n-1 {
			minVal := data[0]
			for j := 0; j <= i; j++ {
				if data[j] < minVal {
					minVal = data[j]
				}
			}
			result[i] = minVal
		} else {
			minVal := data[i-n+1]
			for j := i - n + 2; j <= i; j++ {
				if data[j] < minVal {
					minVal = data[j]
				}
			}
			result[i] = minVal
		}
	}
	return result
}

// hhv N周期内的最大值
func hhv(data []float64, n int) []float64 {
	result := make([]float64, len(data))
	for i := range data {
		if i < n-1 {
			maxVal := data[0]
			for j := 0; j <= i; j++ {
				if data[j] > maxVal {
					maxVal = data[j]
				}
			}
			result[i] = maxVal
		} else {
			maxVal := data[i-n+1]
			for j := i - n + 2; j <= i; j++ {
				if data[j] > maxVal {
					maxVal = data[j]
				}
			}
			result[i] = maxVal
		}
	}
	return result
}

// cross 判断A是否上穿B。前一天 A<=B 且 今天 A>B。
func cross(A, B []float64) []bool {
	result := make([]bool, len(A))
	for i := 1; i < len(A); i++ {
		result[i] = (A[i-1] <= B[i-1]) && (A[i] > B[i])
	}
	return result
}

// vecAbs 向量逐元素取绝对值
func vecAbs(data []float64) []float64 {
	result := make([]float64, len(data))
	for i, val := range data {
		result[i] = math.Abs(val)
	}
	return result
}

// refBool 引用前N周期的布尔值。REF(data, 1) = 前一天的值。
func refBool(data []bool, n int) []bool {
	result := make([]bool, len(data))
	for i := range data {
		if i < n {
			result[i] = false
		} else {
			result[i] = data[i-n]
		}
	}
	return result
}
