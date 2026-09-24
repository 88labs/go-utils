---
name: code-review
description: Review go-utils diffs and PRs for downstream compatibility, standards compliance, dependency risk, public-repository safety, and meaningful Go test coverage.
---

# Code review for go-utils

Review the requested diff against its surrounding module and downstream use. Prioritize observable bugs and release risk over style preferences. This repository contains independently versioned public Go modules. Read each changed module's `go.mod`, README, tests, and applicable repository instructions. Do not assume there is a root Go module.

## Workflow

1. Establish the scope and intended behavior from the request, diff, public API, module metadata, and authoritative specifications. Do not treat the current implementation alone as the specification.
2. Check the contracts below. For each suspected issue, identify a concrete input, caller, or release scenario, and verify it against code, tests, and an official source where relevant. Do not repeat a finding at multiple locations.
3. Run focused checks for changed modules when useful. State what ran and what could not be verified. A passing test suite or coverage percentage does not prove completeness.
4. Report actionable findings first, ordered by severity. Give the exact file and line, failure scenario, consumer impact, and the smallest reasonable correction or missing test. Separate confirmed findings from open questions. If none are found, state that and mention material verification limits.

## Review contracts

### Public API and releases

- Treat exported identifiers, signatures, behavior, errors, zero values, wire formats, configuration defaults, and supported Go versions as downstream contracts. Check source compatibility and observable behavior, including callers using `errors.Is` and `errors.As`.
- Check each changed module independently: module path, `go.mod`/`go.sum`, intermodule imports, and release or tag implications against the [Go Modules Reference](https://go.dev/ref/mod). Flag potential breaking changes and assess compatible extensions or migration paths. A major version bump does not excuse accidental breakage.
- Review alternate and failure paths: nil and zero values, limits, malformed or duplicate inputs, cancellation, cleanup, ordering, and concurrent use when applicable.

### Specifications and implementation

- For protocol and library interfaces, consult the applicable primary source before accepting behavior: IETF RFCs, [Go documentation](https://go.dev/doc/), or official [Google](https://github.com/google), [gRPC](https://github.com/grpc), [Protocol Buffers](https://github.com/protocolbuffers), and [ConnectRPC](https://github.com/connectrpc) repositories and documentation. Check the version in use and all required cases, not only examples or the common path. Cite the precise source in a finding.
- For AWS-facing code, require AWS SDK for Go v2 and flag v1 imports or dependencies in changed integrations: [AWS ended v1 support](https://aws.amazon.com/blogs/developer/announcing-end-of-support-for-aws-sdk-for-go-v1-on-july-31-2025/). Consult the relevant [AWS service documentation](https://docs.aws.amazon.com/), the [AWS SDK for Go v2 documentation](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/), the official [aws-sdk-go-v2 repository](https://github.com/aws/aws-sdk-go-v2), and [smithy-go](https://github.com/aws/smithy-go) when its runtime is involved. Verify behavior against the service and SDK versions used by the changed module. For S3 `Content-Disposition`, also check [RFC 6266](https://www.rfc-editor.org/info/rfc6266) and [RFC 8187](https://www.rfc-editor.org/info/rfc8187).
- For `tracers` and trace propagation in `aws`, use [W3C Trace Context](https://www.w3.org/TR/trace-context/), the [OpenTelemetry specification](https://opentelemetry.io/docs/specs/otel/), and the official [Datadog Go tracer](https://github.com/DataDog/dd-trace-go) where its bridge is involved.
- For format conversions and validation, use [RFC 5322](https://www.rfc-editor.org/info/rfc5322) for `emailvalidator` within its documented subset, the [Unicode Standard's BOM definition](https://www.unicode.org/versions/latest/core-spec/chapter-23/) for `utf8bom`, and the [Protocol Buffers Timestamp definition](https://protobuf.dev/reference/protobuf/google.protobuf/) for `tspb_cast`.
- For upstream SDK wrappers, inspect the official [Sentry Go SDK](https://github.com/getsentry/sentry-go) for `sentryhelper` and the versioned [oklog/ulid implementation](https://github.com/oklog/ulid) for `ulid`. For `cerrors/http`, check [HTTP Semantics](https://www.rfc-editor.org/rfc/rfc9110.html); for `sql-escape`, check the target database vendor's official `LIKE` and `ESCAPE` documentation rather than assuming one SQL dialect.
- For `errgroup`, compare its contract with the versioned upstream [golang.org/x/sync/errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) documentation and source. Verify that differences in panic propagation, cancellation, and concurrency limits are intentional and tested.
- For `backoff` and `jitter`, check the documented policy against [AWS's Exponential Backoff and Jitter guidance](https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/). For `hashutil`, use Go's [crypto/sha256 documentation](https://pkg.go.dev/crypto/sha256) and the [NIST Secure Hash Standard](https://csrc.nist.gov/pubs/fips/180-4/upd1/final) when algorithm details matter. For other standard-library wrappers, consult the relevant [Go package documentation](https://pkg.go.dev/std).
- Review for clarity, simplicity, concision, and maintainability. Apply [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) and the [Google Go style guide](https://google.github.io/styleguide/go/) with context: names and comments, error flow, pointer/value semantics, context propagation, and bounded goroutine lifetimes. Raise style only when it affects understanding or maintenance.
- Prefer the standard library where it meets the contract. For a new external dependency, require a concrete capability that existing code or the standard library cannot reasonably provide. Inspect transitive dependencies, version support, maintenance, and public API exposure. Flag avoidable dependencies and supply chain risk.

### Tests and public data

- Check that tests define new behavior where practical and that every new or changed library behavior has comprehensive executable scenarios. Expect normal and alternate outcomes, boundaries, invalid input, error identity and wrapping, concurrency and cancellation, cleanup, and external I/O seams when applicable. Preserve regression cases for existing public behavior.
- Check each test's preconditions, action, and observable result. Assert full relevant outputs and side effects; omitted expected fields must not silently pass. Prefer focused table-driven cases when they clarify differences, useful failure messages, and `t.Helper()` for helpers. Do not demand a particular test shape without a coverage benefit.
- This is a public repository. Flag credentials, internal implementation details, nonpublic infrastructure information, and real customer names or data in source, tests, fixtures, comments, docs, and review text. Use synthetic data. Public ANDPAD information and clearly fictional examples such as `ANDPAD inc` or `安藤太郎` are acceptable when they reveal no customer or private fact.

## Output

For each finding, give a severity (`P0`–`P3`), exact location, reproducible scenario, consequence for consumers, and supporting evidence. End with checks run and unresolved risks. Do not publish private context or quote internal documents in a public review.
