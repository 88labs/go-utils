package awss3

import (
	"context"
	"log/slog"
	"time"
)

func (c *Client) logOperation(ctx context.Context, operation string, attrs ...slog.Attr) func(err error, extra ...slog.Attr) {
	if c == nil || c.logger == nil {
		return func(error, ...slog.Attr) {}
	}

	logger := c.logger.With(slog.String("component", "awss3"))
	baseAttrs := make([]slog.Attr, 0, len(attrs)+1)
	baseAttrs = append(baseAttrs, slog.String("operation", operation))
	baseAttrs = append(baseAttrs, attrs...)
	startedAt := time.Now()

	return func(err error, extra ...slog.Attr) {
		logAttrs := make([]slog.Attr, 0, len(baseAttrs)+len(extra)+2)
		logAttrs = append(logAttrs, baseAttrs...)
		logAttrs = append(logAttrs, extra...)
		logAttrs = append(logAttrs, slog.Duration("duration", time.Since(startedAt)))

		if err != nil {
			logAttrs = append(logAttrs, slog.Any("error", err))
			logger.LogAttrs(ctx, slog.LevelError, "awss3 operation failed", logAttrs...)
			return
		}

		logger.LogAttrs(ctx, slog.LevelInfo, "awss3 operation completed", logAttrs...)
	}
}
