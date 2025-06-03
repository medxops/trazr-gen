// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"io"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapOutputWriter returns the correct io.Writer for zap logs based on CLI mode.
func ZapOutputWriter(terminal bool) io.Writer {
	if terminal {
		return io.Discard
	}
	return os.Stdout
}

// CreateLogger creates a logger for use by trazr-gen
// Uses ZapOutputWriter() for output destination.
func CreateLogger(level string, terminal bool) (*zap.Logger, error) {
	zapLevel := zapcore.InfoLevel
	switch strings.ToLower(level) {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "warn", "warning":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	case "info", "information":
		zapLevel = zapcore.InfoLevel
	}
	cfg := zap.NewProductionConfig()
	enc := zapcore.NewJSONEncoder(cfg.EncoderConfig)
	core := zapcore.NewCore(enc, zapcore.AddSync(ZapOutputWriter(terminal)), zapLevel)
	logger := zap.New(core)

	return logger, nil
}
