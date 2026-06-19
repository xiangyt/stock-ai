package monitor

// ============================================================================
//  ChangeType — 监控配置变更类型
//
//  与 scheduler.ChangeType 对应，用于 MonitorConfigService → Monitor 的配置变更通知。
// ============================================================================

// ChangeType 配置变更类型
type ChangeType string

const (
	ChangeCreated         ChangeType = "created"
	ChangeUpdated         ChangeType = "updated"
	ChangeDeleted         ChangeType = "deleted"
	ChangeEnabled         ChangeType = "enabled"
	ChangeDisabled        ChangeType = "disabled"
	ChangeHoldingChanged  ChangeType = "holding_changed" // 持仓股变动，需重算 ScopeHeld 的 code 集合
)

// ConfigChange 配置变更事件
type ConfigChange struct {
	Type     ChangeType
	ConfigID uint
}

// ============================================================================
//  QuoteEventBus — 从 quotecache 导入，monitor 直接使用 quotecache.QuoteEventBus
//
//  Monitor 通过 quotecache.QuoteCache 的 QuoteEventBus 部分接收行情事件。
//  类型别名方便 monitor 包内引用，不改语义。
// ============================================================================
