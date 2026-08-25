# tracers

`tracers` extracts trace information from a `context.Context` and normalizes
it to the W3C Trace Context format. It supports OpenTelemetry and Datadog APM
v2 so libraries can pass trace context across service and vendor boundaries
without depending on a single tracing implementation.

## Features

- Reads an OpenTelemetry `SpanContext` first.
- Falls back to a Datadog APM v2 span when no valid OpenTelemetry context is
  present.
- Preserves the W3C `traceparent` and `tracestate` values.
- Retains the complete 128-bit lowercase hexadecimal trace ID internally.
- Returns trace IDs as either 128-bit hexadecimal text or the decimal lower
  64-bit value.
- Validates incoming `traceparent` values using OpenTelemetry's propagation
  implementation.

## Requirements

- Go 1.25 or later

## Installation

```shell
go get github.com/88labs/go-utils/tracers
```

## Extract trace context

`ExtractTraceContext` checks the OpenTelemetry context first and uses Datadog
as a fallback. The application's OpenTelemetry `TracerProvider` should be
configured before starting spans.

```go
import (
	"context"

	"github.com/88labs/go-utils/tracers"
	"go.opentelemetry.io/otel"
)

tracer := otel.Tracer("example")
ctx, span := tracer.Start(context.Background(), "operation")
defer span.End()

info, ok := tracers.ExtractTraceContext(ctx)
if !ok {
	// No valid OpenTelemetry or Datadog trace context was found.
	return
}

traceParent := info.GetTraceParent()
traceState := info.GetTraceState()
traceID := info.GetTraceID(tracers.FormatString)
traceIDInt64 := info.GetTraceID(tracers.FormatInt64)
spanID := info.GetSpanID()
spanIDUInt64 := info.GetSpanIDUInt64() // Datadog log correlation
```

When both a valid OpenTelemetry `SpanContext` and a Datadog span are present,
the OpenTelemetry context takes precedence.

## Datadog APM v2

Datadog APM integration requires
[`dd-trace-go/v2`](https://pkg.go.dev/github.com/DataDog/dd-trace-go/v2).
`dd-trace-go` v1 is not supported.

Datadog spans can be extracted from the same context. The returned values are
normalized to W3C `traceparent` and `tracestate` fields.

```go
import (
	"context"

	ddtracer "github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/88labs/go-utils/tracers"
)

span, ctx := ddtracer.StartSpanFromContext(context.Background(), "operation")
defer span.Finish()

info, ok := tracers.ExtractTraceContext(ctx)
if !ok {
	return
}
// info.GetTraceParent() and info.GetTraceState() are ready for W3C propagation.
```

The Datadog tracer must be started and configured by the application. This
package only reads the span from the context and does not start or finish
tracing spans.

## Parse propagated values

Use `NewTraceInfoFromTraceParent` at an inbound propagation boundary.
Invalid `traceparent` values return the zero value. Invalid `tracestate`
values are ignored according to the W3C Trace Context processing model.

```go
import (
	"net/http"

	"github.com/88labs/go-utils/tracers"
)

func handle(r *http.Request) {
	info := tracers.NewTraceInfoFromTraceParent(
		r.Header.Get("traceparent"),
		r.Header.Get("tracestate"),
	)
	if !info.IsValid() {
		return
	}

	traceID := info.GetTraceID(tracers.FormatString)
	spanID := info.GetSpanID()
}
```

`IsValid` validates the current traceparent value as a W3C span context. Use
`GetTraceParent` and `GetTraceState` to retrieve the propagation values.
`GetSpanIDUInt64` returns the span ID as a `uint64` for Datadog log fields and
returns zero when the trace context is invalid.

## Trace ID formats

`TraceInfo` always retains the complete 128-bit trace ID as a lowercase
32-character hexadecimal string. Select the output representation at the
getter:

```go
traceIDString := info.GetTraceID(tracers.FormatString)
traceIDInt64 := info.GetTraceID(tracers.FormatInt64)
```

| Format | `GetTraceID` result |
|---|---|
| `FormatString` | 128-bit lowercase hexadecimal string (default) |
| `FormatInt64` | Decimal lower 64-bit value |

## Testing

Run the module tests from the `tracers` directory:

```shell
go test ./...
```

## References

- [W3C Trace Context](https://www.w3.org/TR/trace-context/)
- [OpenTelemetry Tracing API](https://opentelemetry.io/docs/specs/otel/trace/api/)
- [Datadog Go tracer v2](https://pkg.go.dev/github.com/DataDog/dd-trace-go/v2/ddtrace/tracer)
