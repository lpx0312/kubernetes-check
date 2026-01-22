package log

import (
	"log/slog"
	"os"
	"strings"
)

// Logger 结构化日志封装
type Logger struct {
	*slog.Logger
}

// New 创建新的日志器
func New(level string, pretty bool) *Logger {
	opts := &slog.HandlerOptions{
		Level: parseLevel(level),
	}

	var handler slog.Handler
	if pretty {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return &Logger{slog.New(handler)}
}

// parseLevel 解析日志级别
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// 预定义日志器实例
var (
	// Stdout 标准输出日志器（info 级别，文本格式）
	Stdout = New("info", true)

	// Debug 调试日志器（debug 级别，文本格式）
	Debug = New("debug", true)
)
