package adapter

import "errors"

// ErrNotImplemented 表示该方法尚未实现
var ErrNotImplemented = errors.New("not implemented")

// ErrUnsupported 表示该数据源不支持此功能
var ErrUnsupported = errors.New("feature unsupported by this data source")
