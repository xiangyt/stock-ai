package backtest

import "errors"

var (
	ErrSignalNotFound = errors.New("signal not found")
	ErrDataNotFound   = errors.New("stock data not found")
	ErrNoConfig       = errors.New("no signal config provided")
	ErrInvalidParams  = errors.New("invalid signal params")
)
