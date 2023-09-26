package ctxawslocal

import "context"

type ctxKeyLocal struct{}

func WithContext(ctx context.Context, opts ...OptionMock) context.Context {
	return context.WithValue(ctx, ctxKeyLocal{}, getConf(opts...))
}

func IsLocal(ctx context.Context) bool {
	if _, ok := ctx.Value(ctxKeyLocal{}).(ConfMock); ok {
		return true
	}
	return false
}

func GetConf(ctx context.Context) (*ConfMock, bool) {
	if conf, ok := ctx.Value(ctxKeyLocal{}).(ConfMock); ok {
		return &conf, true
	}
	return nil, false
}
