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
)
