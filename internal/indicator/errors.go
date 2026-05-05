package indicator

import "errors"

// ============================================================================
//  常见错误定义 — 指标评估过程中的统一错误门面
//
//  使用 errors.Is / errors.As 进行错误匹配，指标 Evaluate 返回的
//  EvaluatedStock.Message 中可按需包含这些错误信息。
// ============================================================================

var (
	// ErrDataEmpty 数据为空：请求的数据类型在 DB 中不存在（如新股无财报、停牌无K线等）
	ErrDataEmpty = errors.New("indicator: data is empty")

	// ErrDatabase 数据库错误：查询/连接失败
	ErrDatabase = errors.New("indicator: database error")

	// ErrNetwork 网络错误：外部数据源不可达
	ErrNetwork = errors.New("indicator: network error")

	// ErrUnsupportedSignal 不支持的信号：SignalID 无法匹配到任何已注册信号
	ErrUnsupportedSignal = errors.New("indicator: unsupported signal")

	// ErrNoConfig 未配置信号：传入的 configs 为空
	ErrNoConfig = errors.New("indicator: no signal config")

	// ErrIndicatorNotFound 指标不存在：SignalID 对应的指标未注册
	ErrIndicatorNotFound = errors.New("indicator: indicator not found")

	// ErrDataNotReady 数据未就绪：懒加载尚未完成（通常不应出现）
	ErrDataNotReady = errors.New("indicator: data not ready")
)

// DataEmptyError 创建带上下文的数据为空错误，field 为缺失的数据字段名。
// 例: DataEmptyError("DailySnapshot") → "indicator: data is empty: DailySnapshot"
func DataEmptyError(field string) error {
	return errors.Join(ErrDataEmpty, errors.New(field))
}

// DatabaseError 创建带上下文的数据库错误。
func DatabaseError(err error) error {
	return errors.Join(ErrDatabase, err)
}

// NetworkError 创建带上下文的网络错误。
func NetworkError(err error) error {
	return errors.Join(ErrNetwork, err)
}
