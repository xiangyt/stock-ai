package fundamental

// ============================================================================
//  基本面指标序号 (分类码 03 由 BaseIndicator.ID() 动态拼接)
//
//  完整6位ID = "03" + Seq, 如 IndListingBoardSeq="001" → "03001"
// ============================================================================

const (
	IndListingBoardSeq  = "001" // 上市板块
	IndTotalSharesSeq   = "002" // 总股本
	IndFloatSharesSeq   = "003" // 流通股本
	IndTotalMarketCapSeq    = "004" // 总市值
	IndCirculateMarketCapSeq = "005" // 流通市值
	IndFloatRatioSeq        = "006" // 流通比例
	IndFreeHoldRatioSeq     = "007" // 十大流通股东持股比例
	IndHoldRatioSeq         = "008" // 十大股东持股比例
	IndHolderNumSeq         = "009" // 股东户数
	IndAvgHoldSharesSeq     = "010" // 户均持股数
	IndStSeq                = "011" // ST股
)
