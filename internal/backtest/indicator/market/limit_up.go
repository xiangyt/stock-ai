package market

import (
	"fmt"
	"math"

	"stock-ai/internal/backtest/indicator"
	signalutil "stock-ai/internal/backtest/indicator/signalutil"
	"stock-ai/internal/model"
)

const (
	paramMinCount = "min_count" // 最低涨停次数，默认 2

	maxLimitDays = 60 // calcLimitUpInfo 最大计算天数
)

// ============================================================================
//  公共函数：calcLimitUpInfo — 计算近 60 个交易日每日涨停信息
//
//  klines[0] 为最新交易日，Close 单位为分。
//  返回切片长度 = min(60, len(klines)-1)，[0] 对应最新一日。
//
//  涨停价 = floor(前收盘价 × (1 + 涨跌幅限制), 分) 向下取整为分。
//  涨跌幅: 主板 10%, 科创/创业板 20%, 北交所 30%
//
//  判断依据: 当日收盘价 >= 涨停价 即为涨停。
// ============================================================================

// LimitUpInfo 涨停判定结果
type LimitUpInfo struct {
	IsLimitUp  bool // 当日涨停（Close >= 涨停价）
	LimitPrice int  // 涨停价（分），向下取整
}

func calcLimitUpInfo(klines []*model.DailyKline, board string) []LimitUpInfo {
	// 至少需要 2 根 K 线（当日 + 前一日收盘价）
	if len(klines) < 2 {
		return nil
	}

	n := len(klines) - 1 // 最多可用天数（最后一日缺少前收）
	if n > maxLimitDays {
		n = maxLimitDays
	}

	ratio := getLimitRatio(board)
	result := make([]LimitUpInfo, n)

	for i := 0; i < n; i++ {
		prevClose := klines[i+1].Close // 前收盘价（分）
		// 涨停价 = prevClose × (1 + ratio)，向下取整到分（避免四舍五入引入的误差）
		limitPrice := int(math.Floor(float64(prevClose) * (1 + ratio)))

		result[i] = LimitUpInfo{
			IsLimitUp:  klines[i].Close >= limitPrice,
			LimitPrice: limitPrice,
		}
	}
	return result
}

// getLimitRatio 根据上市板块判断涨跌幅限制比例。
//
// 板块与涨跌幅对应关系:
//
//	主板/中小板: 10%
//	科创板/创业板: 20%
//	北交所: 30%
//	默认（未知板块）: 10%
func getLimitRatio(board string) float64 {
	switch board {
	case model.BoardMain, model.BoardSME:
		return 0.10
	case model.BoardChiNext, model.BoardStar:
		return 0.20
	case model.BoardBSE:
		return 0.30
	default:
		return 0.10
	}
}

// ============================================================================
//  LimitUp — 涨停股 (序列型)
//  ID: 02005 = CatCodeMarket("02") + IndLimitUpSeq("005")
//  数据源: GetDailyKline() + GetDetail()
// ============================================================================

type LimitUp struct {
	indicator.BaseIndicator
}

func NewLimitUp() *LimitUp {
	i := &LimitUp{
		BaseIndicator: indicator.BaseIndicator{
			Seq:         IndLimitUpSeq,
			NameStr:     "涨停",
			CategoryVal: indicator.CatMarket,
			Desc:        "涨停股指标，判断近N个交易日涨停次数>=X",
			UnitStr:     "",
		},
	}

	i.SetBuiltInSignals([]indicator.Signal{
		NewBuiltInLimitUpCount(),
	})

	i.SetCustomSignals([]indicator.Signal{
		NewCustomLimitUpCount(),
	})
	return i
}

// Evaluate 涨停指标评估入口。
//
//  1. 获取 K 线数据 + 股票详情（板块）
//  2. 统一计算 calcLimitUpInfo（所有信号共享）
//  3. 分发给具体信号
func (i *LimitUp) Evaluate(stock indicator.StockSource, config []*indicator.SignalConfig) *indicator.EvaluatedStock {
	if len(config) == 0 {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, Message: indicator.ErrNoConfig.Error()}
	}

	klines, err := stock.GetDailyKline()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config[0].SignalID,
			Message: err.Error()}
	}
	if len(klines) < 2 {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config[0].SignalID,
			Message:  "K线数据不足，需要至少 2 根日K线",
		}
	}

	detail, err := stock.GetDetail()
	if err != nil {
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: config[0].SignalID,
			Message: err.Error()}
	}

	infos := calcLimitUpInfo(klines, detail.ListingBoard)

	for _, v := range config {
		if s, ok := i.Signal[v.SignalID]; ok {
			switch vv := s.(type) {
			case *SignalLimitUpCount:
				if res := vv.Evaluate(infos, v); res.Result == indicator.ResultPassed {
					continue
				} else {
					return res
				}
			}
		}
		return &indicator.EvaluatedStock{Result: indicator.ResultRejected, SignalID: v.SignalID,
			Message: indicator.ErrUnsupportedSignal.Error()}
	}
	return &indicator.EvaluatedStock{Result: indicator.ResultPassed}
}

// ============================================================================
//  SignalLimitUpCount — M-N天有 X 次及以上涨停
//
//  信号ID: 02005001
//
//  参数:
//    lookback_start (默认 4): 窗口起点（N天前，默认4 = 近5天）
//    lookback_end   (默认 0): 窗口终点（N天前，默认0 = 今天）
//    min_count      (默认 2): 最低涨停次数
//
//  判定规则: 窗口内每日收盘价 >= 涨停价则计数，满足 count >= minCount 即通过。
// ============================================================================

type SignalLimitUpCount struct {
	indicator.BaseSignal
}

// newLimitUpCountSignal 涨停次数信号私有工厂。
// signalType 为空时使用 name 作为 SignalType。
func newLimitUpCountSignal(name, desc, signalType string) *SignalLimitUpCount {
	base := indicator.NewBaseSignal(
		"01",
		name,
		desc,
		indicator.ValSeries,
		[]indicator.OperatorOption{
			{
				Operator: indicator.OpCustom,
				Label:    "参数设置",
				Params: []indicator.ParamDef{
					signalutil.ParamLookbackStart(4, "天前"),
					signalutil.ParamLookbackEnd(0, "天前"),
					signalutil.ParamNumber(paramMinCount, "涨停次数>=", 2, "次"),
				},
			},
		},
		&indicator.SignalConfig{
			Operator: indicator.OpCustom,
			Params: map[string]any{
				indicator.ParamKeyLookbackStart: float64(4),
				indicator.ParamKeyLookbackEnd:   float64(0),
				paramMinCount:                   float64(2),
			},
		},
	)
	if signalType != "" {
		base.SetSignalType(signalType)
	}
	return &SignalLimitUpCount{BaseSignal: base}
}

// NewBuiltInLimitUpCount 内置涨停信号：近5日 ≥2 次涨停。
func NewBuiltInLimitUpCount() *SignalLimitUpCount {
	return newLimitUpCountSignal("近一周有2次及以上涨停", "近5个交易日有2次以上涨停", "")
}

// NewCustomLimitUpCount 自定义涨停信号：时间窗口和涨停次数由用户自由配置。
func NewCustomLimitUpCount() *SignalLimitUpCount {
	return newLimitUpCountSignal("涨停次数", "自定义时间窗口内涨停次数", "自定义涨停")
}

// Evaluate 评估近 N 日涨停次数是否满足阈值。
//
// infos[0] 为最新一日，与 klines 方向一致。
func (s *SignalLimitUpCount) Evaluate(infos []LimitUpInfo, config *indicator.SignalConfig) *indicator.EvaluatedStock {
	if !config.IsCustom() {
		config = s.DefaultConfig()
	}

	start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 4))
	end := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
	minCount := int(config.GetFloat64(paramMinCount, 2))

	// 容错：确保 start >= end（start 更早，end 更近）
	if start < end {
		start, end = end, start
	}

	// 参数越界：lookback_start 不能超出涨停信息最大计算范围
	if start >= maxLimitDays {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("参数越界：回看起点 %d 天前超出最大范围 %d 天前", start, maxLimitDays-1),
		}
	}

	// 确保窗口起点在数据范围内
	if start >= len(infos) {
		return &indicator.EvaluatedStock{
			Result:   indicator.ResultRejected,
			SignalID: config.SignalID,
			Message:  fmt.Sprintf("K线数据不足：需要 %d 天前数据，实际仅 %d 天", start, len(infos)-1),
		}
	}

	// 统计窗口内涨停的交易日数量
	count := 0
	for i := end; i <= start; i++ {
		if infos[i].IsLimitUp {
			count++
		}
	}

	if count >= minCount {
		return &indicator.EvaluatedStock{Result: indicator.ResultPassed, SignalID: config.SignalID}
	}

	return &indicator.EvaluatedStock{
		Result:   indicator.ResultRejected,
		SignalID: config.SignalID,
		Message:  fmt.Sprintf("近 %d 日涨停次数=%d，不满足 >= %d", start-end+1, count, minCount),
	}
}
