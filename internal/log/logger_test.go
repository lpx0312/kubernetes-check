package log

import (
	"context"
	"testing"
)

func TestNew(t *testing.T) {
	logger := New("info", false)
	if logger == nil {
		t.Fatal("New() returned nil")
	}
	if logger.Logger == nil {
		t.Error("Logger field is nil")
	}
}

func TestStdout(t *testing.T) {
	if Stdout == nil {
		t.Error("Stdout is nil")
	}
	if Stdout.Logger == nil {
		t.Error("Stdout.Logger is nil")
	}
}

func TestLogger_ContextMethods(t *testing.T) {
	logger := New("debug", false)
	ctx := context.Background()

	// These methods should not panic
	logger.DebugContext(ctx, "test debug", "key", "value")
	logger.InfoContext(ctx, "test info", "key", "value")
	logger.WarnContext(ctx, "test warn", "key", "value")
	logger.ErrorContext(ctx, "test error", "key", "value")
}
