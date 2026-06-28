package backtest

import (
	"context"
	"sync"
	"time"
)

// frontendActivityTimeout 前端断连检测超时。
// 若超过此时间内前端未拉取进度，视为断连并停止回测。
const frontendActivityTimeout = 20 * time.Second

// ============================================================================
//  RunManager — 运行中回测的全局管理
// ============================================================================

var (
	rmMu       sync.Mutex
	rmCancels  = make(map[uint64]context.CancelFunc)
	rmLastSeen = make(map[uint64]time.Time)
)

// RegisterRun 注册运行中的回测（由引擎在 runAsync 中调用）。
func RegisterRun(runID uint64, cancel context.CancelFunc) {
	rmMu.Lock()
	defer rmMu.Unlock()
	rmCancels[runID] = cancel
	rmLastSeen[runID] = time.Now()
}

// CancelRun 取消指定 runID 的回测。
func CancelRun(runID uint64) {
	rmMu.Lock()
	cancel, ok := rmCancels[runID]
	rmMu.Unlock()
	if ok {
		cancel()
	}
}

// CheckInRun 记录前端活跃时间（每次 GetRunStatus 时调用）。
func CheckInRun(runID uint64) {
	rmMu.Lock()
	defer rmMu.Unlock()
	rmLastSeen[runID] = time.Now()
}

// LastActivity 返回前端最近一次活跃时间。
func LastActivity(runID uint64) time.Time {
	rmMu.Lock()
	defer rmMu.Unlock()
	return rmLastSeen[runID]
}

// CleanupRun 清理已结束的回测记录。
func CleanupRun(runID uint64) {
	rmMu.Lock()
	defer rmMu.Unlock()
	delete(rmCancels, runID)
	delete(rmLastSeen, runID)
}
