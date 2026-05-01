package sentryhelper

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/getsentry/sentry-go"
)

func testHubContext(t *testing.T) (context.Context, *sentry.MockTransport) {
	t.Helper()
	tr := &sentry.MockTransport{}
	cl, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:       "https://public@example.com/1",
		Transport: tr,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	hub := sentry.NewHub(cl, sentry.NewScope())
	return sentry.SetHubOnContext(context.Background(), hub), tr
}

func TestSetException_nil(t *testing.T) {
	if got := SetException(nil, 10); got != nil {
		t.Fatalf("SetException(nil) = %#v, want nil", got)
	}
}

func TestSetException_singleError(t *testing.T) {
	err := errors.New("only")
	if got := SetException(err, 10); got != nil {
		t.Fatalf("single error: SetException = %#v, want nil (exception group needs 2+)", got)
	}
}

func TestSetException_wrappedChain(t *testing.T) {
	inner := errors.New("inner")
	outer := fmt.Errorf("outer: %w", inner)
	ex := SetException(outer, 10)
	if len(ex) != 2 {
		t.Fatalf("len(Exception) = %d, want 2", len(ex))
	}
	if ex[0].Value != inner.Error() {
		t.Errorf("ex[0].Value = %q, want inner message", ex[0].Value)
	}
	if ex[1].Value != outer.Error() {
		t.Errorf("ex[1].Value = %q, want outer message", ex[1].Value)
	}
	for i, e := range ex {
		if e.Mechanism == nil || !e.Mechanism.IsExceptionGroup {
			t.Errorf("ex[%d].Mechanism.IsExceptionGroup = false, want true", i)
		}
	}
	if ex[0].Mechanism.ExceptionID != 0 || ex[1].Mechanism.ExceptionID != 1 {
		t.Errorf("unexpected ExceptionID: %#v, %#v", ex[0].Mechanism, ex[1].Mechanism)
	}
	if ex[1].Mechanism.ParentID == nil || *ex[1].Mechanism.ParentID != 0 {
		t.Errorf("ex[1] parent id = %#v, want 0", ex[1].Mechanism.ParentID)
	}
}

func TestSetException_maxDepthStopsChain(t *testing.T) {
	e1 := errors.New("1")
	e2 := fmt.Errorf("2: %w", e1)
	e3 := fmt.Errorf("3: %w", e2)
	// max depth 1 → only one frame collected → len<=1 → nil
	if got := SetException(e3, 1); got != nil {
		t.Fatalf("maxDepth=1 on chain: got %#v, want nil", got)
	}
	if got := SetException(e3, 2); len(got) != 2 {
		t.Fatalf("maxDepth=2: len = %d, want 2", len(got))
	}
}

type errWithCause struct {
	cause error
}

func (e errWithCause) Error() string { return "withCause" }
func (e errWithCause) Cause() error   { return e.cause }

func TestSetException_causeUnwrap(t *testing.T) {
	root := errors.New("root")
	outer := errWithCause{cause: root}
	ex := SetException(outer, 10)
	if len(ex) != 2 {
		t.Fatalf("len = %d, want 2", len(ex))
	}
	if ex[0].Value != root.Error() {
		t.Errorf("ex[0].Value = %q", ex[0].Value)
	}
}

func TestEventFromException(t *testing.T) {
	ctx, _ := testHubContext(t)
	hub := sentry.GetHubFromContext(ctx)
	event := EventFromException(hub, errors.New("x"), sentry.LevelWarning)
	if event.Level != sentry.LevelWarning {
		t.Errorf("Level = %v, want warning", event.Level)
	}
	if event.Exception != nil {
		t.Errorf("single-error chain: Exception = %#v, want nil", event.Exception)
	}
}

func TestCaptureException_noHub(t *testing.T) {
	CaptureException(context.Background(), errors.New("noop"))
}

func TestCaptureException_sendsEvent(t *testing.T) {
	ctx, tr := testHubContext(t)
	inner := errors.New("inner")
	outer := fmt.Errorf("wrapped: %w", inner)
	CaptureException(ctx, outer)
	events := tr.Events()
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	ev := events[0]
	if ev.Level != sentry.LevelError {
		t.Errorf("Level = %v, want error", ev.Level)
	}
	if len(ev.Exception) != 2 {
		t.Fatalf("len(Exception) = %d, want 2", len(ev.Exception))
	}
}

func TestCaptureMessage_sendsEvent(t *testing.T) {
	ctx, tr := testHubContext(t)
	CaptureMessage(ctx, "hello")
	events := tr.Events()
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Level != sentry.LevelInfo {
		t.Errorf("Level = %v, want info", events[0].Level)
	}
	if events[0].Message != "hello" {
		t.Errorf("Message = %q, want hello", events[0].Message)
	}
}

func TestCaptureException_optionsScope(t *testing.T) {
	ctx, tr := testHubContext(t)
	inner := errors.New("inner")
	outer := fmt.Errorf("wrapped: %w", inner)
	CaptureException(ctx, outer,
		WithLevel(sentry.LevelFatal),
		WithTag(Tag{"k": "v"}),
		WithFingerprint("fp-a", "fp-b"),
		WithUser(sentry.User{ID: "user-1"}),
	)
	if len(tr.Events()) != 1 {
		t.Fatalf("len(events) = %d", len(tr.Events()))
	}
	ev := tr.Events()[0]
	if ev.Level != sentry.LevelFatal {
		t.Errorf("Level = %v, want fatal", ev.Level)
	}
	if ev.Tags["k"] != "v" {
		t.Errorf("Tags[k] = %q", ev.Tags["k"])
	}
	if len(ev.Fingerprint) != 2 || ev.Fingerprint[0] != "fp-a" {
		t.Errorf("Fingerprint = %#v", ev.Fingerprint)
	}
	if ev.User.ID != "user-1" {
		t.Errorf("User.ID = %q", ev.User.ID)
	}
}
