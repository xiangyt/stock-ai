package financial

// ============================================================================
//  财务面指标序号 (分类码 04 由 BaseIndicator.ID() 动态拼接)
//
//  完整6位ID = "04" + Seq, 如 IndPETTMSeq="001" → "04001"
// ============================================================================

const (
	IndPETTMSeq          = "001" // 市盈率(TTM)
	IndPBSeq             = "002" // 市净率
	IndPSTTMSeq          = "003" // 市销率(TTM)
	IndROESeq            = "004" // 净资产收益率(%)
	IndROASeq            = "005" // 总资产收益率(%)
	IndGrossMarginSeq    = "006" // 毛利率(%)
	IndNetMarginSeq      = "007" // 净利率(%)
	IndBVPSSeq           = "008" // 每股净资产(元)
	IndBasicEPSSeq       = "009" // 基本每股收益(元)
	IndDebtRatioSeq      = "010" // 资产负债率(%)
	IndParentNetProfitSeq  = "011" // 归母净利润(亿元)
	IndDeductNetProfitSeq = "012" // 扣非净利润(亿元)
	IndTotalRevenueSeq     = "013" // 营业总收入(亿元)
)
