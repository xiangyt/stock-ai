package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm/logger"

	applog "stock-ai/internal/log"
)

// SlowSQLThreshold 慢查询判定阈值
const SlowSQLThreshold = 500 * time.Millisecond

// GormLogger 适配 GORM 的 logger.Interface，将 SQL 日志统一交给 slog 输出，
// 并从 context 中提取 trace_id 实现链路追踪。
type GormLogger struct {
	level logger.LogLevel
}

// NewGormLogger 创建自定义 GORM Logger。
//
// level 控制输出粒度：logger.Info 记录全部 SQL，logger.Warn 仅慢查询，logger.Error 仅错误。
func NewGormLogger(level logger.LogLevel) *GormLogger {
	return &GormLogger{level: level}
}

// LogMode 返回切换日志级别后的新实例（不修改原实例）。
func (l *GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	nl := *l
	nl.level = level
	return &nl
}

// Info 记录 Info 级别日志。
func (l *GormLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= logger.Info {
		slog.InfoContext(ctx, fmt.Sprintf(msg, args...), "trace_id", applog.TraceIDFromContext(ctx))
	}
}

// Warn 记录 Warn 级别日志。
func (l *GormLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= logger.Warn {
		slog.WarnContext(ctx, fmt.Sprintf(msg, args...), "trace_id", applog.TraceIDFromContext(ctx))
	}
}

// Error 记录 Error 级别日志。
func (l *GormLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= logger.Error {
		slog.ErrorContext(ctx, fmt.Sprintf(msg, args...), "trace_id", applog.TraceIDFromContext(ctx))
	}
}

// Trace 记录每条 SQL 的执行情况：错误 SQL 记 Error，慢查询记 Warn，普通 SQL 记 Debug。
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.level <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	traceID := applog.TraceIDFromContext(ctx)

	if err != nil {
		sql, rows := fc()
		slog.ErrorContext(ctx, "SQL ERROR",
			"trace_id", traceID,
			"sql", sql,
			"rows", rows,
			"error", err.Error(),
			"elapsed_ms", elapsed.Milliseconds(),
		)
		return
	}

	if elapsed > SlowSQLThreshold && l.level >= logger.Warn {
		sql, rows := fc()
		slog.WarnContext(ctx, "SQL SLOW",
			"trace_id", traceID,
			"sql", sql,
			"rows", rows,
			"elapsed_ms", elapsed.Milliseconds(),
		)
		return
	}

	if l.level >= logger.Info {
		sql, rows := fc()
		slog.DebugContext(ctx, "SQL",
			"trace_id", traceID,
			"sql", sql,
			"rows", rows,
			"elapsed_ms", elapsed.Milliseconds(),
		)
	}
}
