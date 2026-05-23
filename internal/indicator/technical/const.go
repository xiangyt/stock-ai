package technical

// ============================================================================
//  基本面指标序号 (分类码 01 由 BaseIndicator.ID() 动态拼接)
//
//  完整6位ID = "01" + Seq, 如 IndPatternSeq="006" → "01006"
// ============================================================================

const (
	IndMaSeq      = "001" // 均线
	IndMoneySeq   = "002" // 资金流入
	IndMacdSeq    = "003" // MACD
	IndKdjSeq     = "004" // KDJ
	IndRsiSeq     = "005" // RSI
	IndPatternSeq    = "006" // 形态
	IndTopBottomSeq  = "100" // 顶底信号
	IndRedThreeSeq   = "101" // 红三角
	IndLongShortSeq  = "102" // 缠论多空
	IndRedTopSeq     = "103" // 红顶底
)
