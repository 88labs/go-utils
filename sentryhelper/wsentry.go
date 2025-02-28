package sentryhelper

import (
	"context"
	"errors"
	"reflect"
	"slices"

	"github.com/getsentry/sentry-go"
)

func CaptureException(ctx context.Context, exception error, opts ...Option) {
	conf := config{
		Level: sentry.LevelError,
	}
	for _, opt := range opts {
		opt.Apply(&conf)
	}
	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		hub.WithScope(func(scope *sentry.Scope) {
			event := EventFromException(hub, exception, conf.Level)
			scope.SetLevel(conf.Level)
			for _, breadcrumb := range conf.Breadcrumbs {
				scope.AddBreadcrumb(breadcrumb, len(conf.Breadcrumbs))
			}
			for _, attachment := range conf.Attachments {
				scope.AddAttachment(attachment)
			}
			if conf.Tag != nil {
				scope.SetTags(conf.Tag)
			}
			if conf.Extra != nil {
				scope.SetExtras(conf.Extra)
			}
			if conf.Contexts != nil {
				scope.SetContexts(conf.Contexts)
			}
			if conf.User != nil {
				scope.SetUser(*conf.User)
			}
			if len(conf.Fingerprint) > 0 {
				scope.SetFingerprint(conf.Fingerprint)
			}
			for _, processor := range conf.EventProcessors {
				scope.AddEventProcessor(processor)
			}
			hub.CaptureEvent(event)
		})
	}
}

func CaptureMessage(ctx context.Context, message string, opts ...Option) {
	conf := config{
		Level: sentry.LevelInfo,
	}
	for _, opt := range opts {
		opt.Apply(&conf)
	}
	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		hub.WithScope(func(scope *sentry.Scope) {
			scope.SetLevel(conf.Level)
			for _, breadcrumb := range conf.Breadcrumbs {
				scope.AddBreadcrumb(breadcrumb, len(conf.Breadcrumbs))
			}
			for _, attachment := range conf.Attachments {
				scope.AddAttachment(attachment)
			}
			if conf.Tag != nil {
				scope.SetTags(conf.Tag)
			}
			if conf.Extra != nil {
				scope.SetExtras(conf.Extra)
			}
			if conf.Contexts != nil {
				scope.SetContexts(conf.Contexts)
			}
			if conf.User != nil {
				scope.SetUser(*conf.User)
			}
			if len(conf.Fingerprint) > 0 {
				scope.SetFingerprint(conf.Fingerprint)
			}
			for _, processor := range conf.EventProcessors {
				scope.AddEventProcessor(processor)
			}
			hub.CaptureMessage(message)
		})
	}
}

// EventFromException creates a new Sentry event from the given `error` instance.
func EventFromException(hub *sentry.Hub, exception error, level sentry.Level) *sentry.Event {
	event := sentry.NewEvent()
	event.Level = level
	event.Exception = SetException(exception, hub.Client().Options().MaxErrorDepth)

	return event
}

// SetException appends the unwrapped errors to the event's exception list.
//
// maxErrorDepth is the maximum depth of the error chain we will look
// into while unwrapping the errors. If maxErrorDepth is -1, we will
// unwrap all errors in the chain.
func SetException(exception error, maxErrorDepth int) []sentry.Exception {
	if exception == nil {
		return nil
	}

	err := exception

	setException := make([]sentry.Exception, 0, maxErrorDepth)

	for i := 0; err != nil && (i < maxErrorDepth || maxErrorDepth == -1); i++ {
		// Add the current error to the exception slice with its details
		setException = append(setException, sentry.Exception{
			Value:      err.Error(),
			Type:       errorType(err),
			Stacktrace: sentry.ExtractStacktrace(err),
		})

		// Attempt to unwrap the error using the standard library's Unwrap method.
		// If errors.Unwrap returns nil, it means either there is no error to unwrap,
		// or the error does not implement the Unwrap method.
		unwrappedErr := errors.Unwrap(err)

		if unwrappedErr != nil {
			// The error was successfully unwrapped using the standard library's Unwrap method.
			err = unwrappedErr
			continue
		}

		cause, ok := err.(interface{ Cause() error })
		if !ok {
			// We cannot unwrap the error further.
			break
		}

		// The error implements the Cause method, indicating it may have been wrapped
		// using the github.com/pkg/errors package.
		err = cause.Cause()
	}

	// Add a trace of the current stack to the most recent error in a chain if
	// it doesn't have a stack trace yet.
	// We only add to the most recent error to avoid duplication and because the
	// current stack is most likely unrelated to errors deeper in the chain.
	if setException[0].Stacktrace == nil {
		setException[0].Stacktrace = sentry.NewStacktrace()
	}
	// replace the leading error to notify
	setException[0].Type = errorType(originError(exception))

	if len(setException) <= 1 {
		return nil
	}

	// event.Exception should be sorted such that the most recent error is last.
	slices.Reverse(setException)

	for i := range setException {
		setException[i].Mechanism = &sentry.Mechanism{
			IsExceptionGroup: true,
			ExceptionID:      i,
			Type:             "generic",
		}
		if i == 0 {
			continue
		}
		setException[i].Mechanism.ParentID = sentry.Pointer(i - 1)
	}
	return setException
}

func errorType(err error) string {
	return reflect.Indirect(reflect.ValueOf(err)).Type().String()
}

func originError(err error) error {
	unwrapErr, v := err, err
	for v != nil {
		switch previous := v.(type) {
		case interface{ Unwrap() error }:
			v = previous.Unwrap()
		case interface{ Cause() error }:
			v = previous.Cause()
		default:
			return unwrapErr
		}
		if v != nil {
			unwrapErr = v
		}
	}
	return unwrapErr
}
