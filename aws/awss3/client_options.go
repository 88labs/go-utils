package awss3

import (
	"io"
	"log/slog"

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

var noopLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// ClientOption configures a Client created with NewClient.
type ClientOption interface {
	apply(*clientConfig)
}

type clientConfig struct {
	logger *slog.Logger
}

type clientOptionFunc func(*clientConfig)

func (f clientOptionFunc) apply(cfg *clientConfig) {
	f(cfg)
}

func defaultClientConfig() clientConfig {
	return clientConfig{
		logger: GlobalLogger,
	}
}

// WithLogger configures a Client to emit structured logs via slog.
// When logger is nil, a no-op logger is used.
func WithLogger(logger *slog.Logger) ClientOption {
	return clientOptionFunc(func(cfg *clientConfig) {
		cfg.logger = normalizeLogger(logger)
	})
}

// WithZapLogger configures a Client to emit structured logs via a zap logger.
// When logger is nil, a no-op logger is used.
func WithZapLogger(logger *zap.Logger) ClientOption {
	return WithLogger(NewLoggerFromZap(logger))
}

// NewLoggerFromZap bridges a zap logger into slog so it can be used with awss3.
// When logger is nil, a no-op logger is returned.
func NewLoggerFromZap(logger *zap.Logger) *slog.Logger {
	if logger == nil {
		return noopLogger
	}
	return slog.New(zapslog.NewHandler(logger.Core()))
}

func normalizeLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return noopLogger
	}
	return logger
}
