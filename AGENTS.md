# go-utils Agent Instructions

Keep the existing `.github/copilot-instructions.md` authoritative for Go style, module layout,
error handling, concurrency, and testing. Load it together with the nearest package README and
the relevant module's `go.mod`; this repository is a collection of independently versioned public
modules rather than one root Go module.

## Test-matrix gate

Repository file edits must use the approved file-edit tools; shell redirection, `tee`, `sed -i`, `cp`, `mv`, `rm`, and similar shell writes are prohibited. Shell commands remain available for reads and tests.

The required matrices must cover public API and downstream compatibility, semantic versioning and
module boundaries, normal and alternate outcomes, zero/one/max and negative/zero inputs where
applicable, malformed or duplicate values, error identity/wrapping, concurrency/cancellation,
resource cleanup, and external I/O seams. Every scenario needs preconditions, action/input,
observable result or error oracle, technique, level, and executable evidence. Statement coverage,
pairwise combinations, and test counts are supporting evidence only.

Before a feature or refactor production edit, establish a focused current-behavior regression
baseline; then use the smallest failing test. Coverage-only work must not change behavior. If a
boundary, failure outcome, compatibility rule, or expected oracle is ambiguous, stop and ask the
user. Do not treat the current implementation as the specification.

## Test harness

Use the existing per-module Taskfile tasks (`task -p test`) and package-local test conventions.
Keep tests beside the module they exercise, minimize mocks to external I/O, and assert concrete
results, stable error behavior, side effects, ordering, and cleanup. Do not claim semantic
completeness from aggregate counts or a single package's coverage.

## Session handoff

Before stopping, provide a Markdown table with work item, completed work, checks/results, and
remaining blockers.

The cross-client hooks also observe shell/Bash tool events and deny common direct write commands.
This parsing is intentionally conservative and cannot prove that a complex shell pipeline is
read-only; hooks are a workflow guardrail, not a security boundary, so review and CI remain
authoritative.
