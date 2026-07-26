package log

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
)

// SlogLineWriter 是一个 io.Writer，将按行写入的文本转为 slog 日志。
// 用于把 gin.DefaultWriter 等第三方库的输出接入 slog，统一日志格式与染色。
type SlogLineWriter struct {
	level slog.Level
	buf   []byte
	mu    sync.Mutex
}

// NewSlogLineWriter 创建把整行文本按指定级别输出到 slog 的 writer。
func NewSlogLineWriter(level slog.Level) *SlogLineWriter {
	return &SlogLineWriter{level: level}
}

// Write 实现 io.Writer，按换行切分并逐行输出 slog。
func (w *SlogLineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimRight(string(w.buf[:idx]), "\r")
		w.buf = w.buf[idx+1:]
		if line != "" {
			slog.Log(context.Background(), w.level, line)
		}
	}
	return len(p), nil
}
