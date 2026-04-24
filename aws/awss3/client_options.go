package awss3

import (
	"log/slog"

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

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
// Panics if logger is nil.
func WithLogger(logger *slog.Logger) ClientOption {
	if logger == nil {
		panic("awss3: WithLogger: logger must not be nil")
	}
	return clientOptionFunc(func(cfg *clientConfig) {
		cfg.logger = logger
	})
}

// WithZapLogger configures a Client to emit structured logs via a zap logger.
// Panics if logger is nil.
func WithZapLogger(logger *zap.Logger) ClientOption {
	if logger == nil {
		panic("awss3: WithZapLogger: logger must not be nil")
	}
	return WithLogger(NewLoggerFromZap(logger))
}

// NewLoggerFromZap bridges a zap logger into slog so it can be used with awss3.
// Panics if logger is nil.
func NewLoggerFromZap(logger *zap.Logger) *slog.Logger {
	if logger == nil {
		panic("awss3: NewLoggerFromZap: logger must not be nil")
	}
	return slog.New(zapslog.NewHandler(logger.Core()))
}
