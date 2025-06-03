package common

import (
	"io"
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestZapOutputWriter(t *testing.T) {
	if w := ZapOutputWriter(true); w != io.Discard {
		t.Errorf("ZapOutputWriter(true) = %v, want io.Discard", w)
	}
	if w := ZapOutputWriter(false); w != os.Stdout {
		t.Errorf("ZapOutputWriter(false) = %v, want os.Stdout", w)
	}
}

func TestCreateLogger(t *testing.T) {
	levels := []string{"debug", "info", "warn", "warning", "error", "information", "unknown"}
	for _, lvl := range levels {
		logger, err := CreateLogger(lvl, false)
		if err != nil {
			t.Errorf("CreateLogger(%q, false) returned error: %v", lvl, err)
		}
		if logger == nil {
			t.Errorf("CreateLogger(%q, false) returned nil logger", lvl)
		}
		// Test logging does not panic
		logger.Info("test log", zap.String("level", lvl))
	}
}
