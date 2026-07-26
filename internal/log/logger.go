package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"

	"stock-ai/internal/config"

	"github.com/google/uuid"
)

// traceIDKey 是 context 中 trace_id 的键类型，避免与其他包冲突
type traceIDKey struct{}

// NewTraceID 生成一个新的 trace_id（UUID v4）。
func NewTraceID() string {
	return uuid.NewString()
}

// levelColors 各日志级别对应的 ANSI 颜色（整行染色）
var levelColors = map[slog.Level]string{
	slog.LevelDebug: "\033[94m", // 亮蓝色（bright blue）
	slog.LevelInfo:  "\033[32m", // 绿色
	slog.LevelWarn:  "\033[33m", // 黄色
	slog.LevelError: "\033[31m", // 红色
}

const colorReset = "\033[0m"

// Init 根据配置初始化全局 slog Logger。
//
// 开发环境（log.format=text）使用自定义 consoleHandler，整行按级别染色；
// 生产环境（log.format=json）输出结构化 JSON 便于日志采集系统解析。
func Init() {
	cfg := config.Get().Log
	level := parseLevel(cfg.Level)

	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "text" {
		handler = newConsoleHandler(os.Stderr, level)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(handler))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// consoleHandler 开发环境控制台 handler，整行按日志级别染色。
type consoleHandler struct {
	w     io.Writer
	mu    *sync.Mutex
	level slog.Level
	attrs []slog.Attr
}

// newConsoleHandler 创建控制台 handler。
func newConsoleHandler(w io.Writer, level slog.Level) *consoleHandler {
	return &consoleHandler{w: w, level: level, mu: &sync.Mutex{}}
}

// Enabled 判断是否输出该级别日志。
func (h *consoleHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

// WithAttrs 派生带预设属性的 handler（共享同一把锁）。
func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nh := *h
	nh.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &nh
}

// WithGroup 暂不支持分组（开发控制台日志无需分组）。
func (h *consoleHandler) WithGroup(string) slog.Handler { return h }

// Handle 格式化并输出一条日志，整行用级别对应的颜色包裹。
func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Time.Format("2006-01-02 15:04:05"))
	b.WriteByte(' ')
	b.WriteString(levelLabel(r.Level))
	if r.Message != "" {
		b.WriteByte(' ')
		b.WriteString(r.Message)
	}
	for _, a := range h.attrs {
		writeAttr(&b, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&b, a)
		return true
	})
	b.WriteByte('\n')

	out := levelColors[r.Level] + b.String() + colorReset
	h.mu.Lock()
	_, err := io.WriteString(h.w, out)
	h.mu.Unlock()
	return err
}

// writeAttr 将单个属性写入 builder，格式为 key=value。
func writeAttr(b *strings.Builder, a slog.Attr) {
	b.WriteByte(' ')
	b.WriteString(a.Key)
	b.WriteByte('=')
	b.WriteString(formatVal(a.Value))
}

// formatVal 格式化属性值，含空格等特殊字符时加引号。
func formatVal(v slog.Value) string {
	s := v.String()
	if needsQuote(s) {
		return strconv.Quote(s)
	}
	return s
}

// needsQuote 判断字符串是否需要加引号。
func needsQuote(s string) bool {
	if s == "" {
		return true
	}
	for _, c := range s {
		if c == ' ' || c == '"' || c == '=' || c < 0x20 {
			return true
		}
	}
	return false
}

// levelLabel 返回 3 字母的级别缩写，便于对齐。
func levelLabel(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERR"
	case l >= slog.LevelWarn:
		return "WRN"
	case l >= slog.LevelInfo:
		return "INF"
	default:
		return "DBG"
	}
}

// WithTraceID 将 trace_id 注入 context。
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

// TraceIDFromContext 从 context 中提取 trace_id，未携带时返回空字符串。
func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey{}).(string); ok {
		return v
	}
	return ""
}
