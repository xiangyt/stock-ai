package backtest

// ============================================================================
//  包初始化 — 通过空白导入激活子包的 init() 注册
//  子包 implements 通过 init() 调用 RegisterExitChecker / RegisterAllocator
// ============================================================================

import (
	_ "stock-ai/internal/backtest/alloc" // 注册 5 个 Allocator
	_ "stock-ai/internal/backtest/exit"  // 注册 5 个 ExitChecker
)
