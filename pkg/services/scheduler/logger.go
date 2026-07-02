package scheduler

import (
	"fmt"
	"log"
)

// Logger 是调度器内部使用的最小化日志接口。
// 兼容 stdlib log、slog、zap、logrus 等任何实现。
type Logger interface {
	Debug(msg string, keysAndValues ...any)
	Info(msg string, keysAndValues ...any)
	Warn(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
}

// noopLogger 是默认的静默实现。
type noopLogger struct{}

func (n noopLogger) Debug(string, ...any) {}
func (n noopLogger) Info(string, ...any)  {}
func (n noopLogger) Warn(string, ...any)  {}
func (n noopLogger) Error(string, ...any) {}

// stdLogger 把标准库 log.Logger 包装成 Logger。
type stdLogger struct {
	inner *log.Logger
}

func (s stdLogger) Debug(msg string, kvs ...any) { s.inner.Print("[DEBUG] " + msg + formatKVs(kvs)) }
func (s stdLogger) Info(msg string, kvs ...any)  { s.inner.Print("[INFO] " + msg + formatKVs(kvs)) }
func (s stdLogger) Warn(msg string, kvs ...any)  { s.inner.Print("[WARN] " + msg + formatKVs(kvs)) }
func (s stdLogger) Error(msg string, kvs ...any) { s.inner.Print("[ERROR] " + msg + formatKVs(kvs)) }

func formatKVs(kvs []any) string {
	if len(kvs) == 0 {
		return ""
	}
	var out string
	for i := 0; i+1 < len(kvs); i += 2 {
		out += fmt.Sprintf(" %v=%v", kvs[i], kvs[i+1])
	}
	return out
}

// NewStdLogger 从标准库 log.Logger 创建 Logger。
func NewStdLogger(l *log.Logger) Logger { return stdLogger{inner: l} }
