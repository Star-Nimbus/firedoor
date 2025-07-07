package telemetry

import (
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// ConfigureZapLogger returns zap.Options configured for the given log level string.
func ConfigureZapLogger(logLevel string) zap.Options {
	var opts zap.Options
	switch logLevel {
	case "debug":
		opts.Level = zapcore.DebugLevel
	case "info":
		opts.Level = zapcore.InfoLevel
	case "warn":
		opts.Level = zapcore.WarnLevel
	case "error":
		opts.Level = zapcore.ErrorLevel
	default:
		opts.Level = zapcore.InfoLevel
	}
	opts.Development = true // preserve existing behavior
	return opts
}
