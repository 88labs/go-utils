# go-utils Agent Instructions

Keep the existing `.github/copilot-instructions.md` authoritative for Go style, module layout,
error handling, concurrency, and testing. Load it together with the nearest package README and
the relevant module's `go.mod`; this repository is a collection of independently versioned public
modules rather than one root Go module.

## Test workflow

Repository file edits must use the approved file-edit tools; shell redirection, `tee`, `sed -i`, `cp`, `mv`, `rm`, and similar shell writes are prohibited. Shell commands remain available for reads and tests.

Write tests before new production code. For existing-logic changes, run the relevant tests after
the implementation batch. CI, infrastructure, documentation, and other non-production-code work
does not require application tests. Use the existing per-module Taskfile tasks (`task -p test`) and
package-local test conventions.
Keep tests beside the module they exercise, minimize mocks to external I/O, and assert concrete
results, stable error behavior, side effects, ordering, and cleanup. Do not claim semantic
completeness from aggregate counts or a single package's coverage.

## Session handoff

Before stopping, provide a Markdown table with work item, completed work, checks/results, and
remaining blockers.

Review and CI remain authoritative.
