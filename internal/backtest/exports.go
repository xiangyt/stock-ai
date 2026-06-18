package backtest

// ============================================================================
//  类型重导出 — 将 types 包的接口和结构体平铺到本包，保持现有代码兼容
// ============================================================================

import "stock-ai/internal/backtest/types"

// 卖出规则相关
type (
	DayBar          = types.DayBar
	ExitChecker     = types.ExitChecker
	PriorityChecker = types.PriorityChecker
	EntryHook       = types.EntryHook
	DailyUpdateHook = types.DailyUpdateHook
	ExitDecision    = types.ExitDecision
	HoldingPosition = types.HoldingPosition

	ExitCheckerFactory = types.ExitCheckerFactory
	SlippageInjector   = types.SlippageInjector
	Prioritizer        = types.Prioritizer
)

// 仓位管理相关
type (
	Allocator      = types.Allocator
	AllocRequest   = types.AllocRequest
	AllocResult    = types.AllocResult
	AllocOrder     = types.AllocOrder
	CandidateInfo  = types.CandidateInfo
	AllocatorFactory = types.AllocatorFactory
)

// 函数重导出
var (
	RegisterExitChecker = types.RegisterExitChecker
	RegisterAllocator   = types.RegisterAllocator
	BuildExitChain      = types.BuildExitChain
	GetSortedChain      = types.GetSortedChain
	GetPriority         = types.GetPriority
	CreateAllocator     = types.CreateAllocator
	CalcBuyAmount       = types.CalcBuyAmount
)
