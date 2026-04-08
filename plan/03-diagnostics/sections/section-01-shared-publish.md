Now I have all the context needed. Let me produce the section content.

# Section 1: Shared Publish Utility

## Overview

Extract the `publishDiagnostics` function from `analysis/fusion/compile.go` into a shared utility so both fusion and the new built-in diagnostics engine can use the same publish path. Then update fusion to call the shared utility.

## Dependencies

None. This section has no dependencies and blocks section-02 (diagnostic engine).

## Files

| File | Action | Purpose |
|------|--------|---------|
| `util/publish_diagnostics.go` | **Create** | Shared `PublishDiagnostics` utility |
| `util/publish_diagnostics_test.go` | **Create** | Tests for the shared utility |
| `analysis/fusion/compile.go` | **Modify** | Replace inline `publishDiagnostics` with call to shared utility |

## Tests (write first)

File: `/home/theotime.poulain/dbt-language-server/util/publish_diagnostics_test.go`

```go
// Test: PublishDiagnostics writes valid LSP DiagnosticsNotification to writer
// - Create a bytes.Buffer as writer
// - Call PublishDiagnostics with a URI and a slice of lsp.Diagnostic
// - Decode the output (strip Content-Length header via rpc.DecodeMessage or manual parsing)
// - Verify the JSON body contains "jsonrpc":"2.0", method "textDocument/publishDiagnostics",
//   params.uri matches, params.diagnostics matches

// Test: PublishDiagnostics sets Source field to "dbt-ls" on all diagnostics
// - Pass diagnostics with empty Source fields
// - Verify the written output has Source == "dbt-ls" on every diagnostic

// Test: published notification has correct JSON-RPC method "textDocument/publishDiagnostics"
// - Verify the method field in the JSON output

// Test: PublishDiagnostics with empty diagnostic slice writes valid notification with empty array
// - Ensures clearing diagnostics works (empty array, not null)

// Test: shared publish utility produces same output as fusion's current implementation
// - Call both the old inline function (replicated in test) and the new shared function
//   with identical inputs, compare byte output
```

## Implementation

### 1. Create `util/publish_diagnostics.go`

Package: `util`

Create an exported function `PublishDiagnostics` with this signature:

```go
func PublishDiagnostics(writer io.Writer, uri string, diagnostics []lsp.Diagnostic)
```

The function does exactly what the current `publishDiagnostics` in `analysis/fusion/compile.go` does (lines 46-59 of that file):

1. Construct an `lsp.DiagnosticsNotification` with:
   - `Notification.RPC` = `"2.0"`
   - `Notification.Method` = `"textDocument/publishDiagnostics"`
   - `Params.URI` = the `uri` argument
   - `Params.Diagnostics` = the `diagnostics` argument
2. Call `util.WriteResponse(writer, notification)` to encode and write

The existing types used are:
- `lsp.DiagnosticsNotification` (defined in `/home/theotime.poulain/dbt-language-server/lsp/textdocument_diagnostics.go`)
- `lsp.Notification` (defined in `/home/theotime.poulain/dbt-language-server/lsp/message.go`)
- `util.WriteResponse` (defined in `/home/theotime.poulain/dbt-language-server/util/write_response.go`) which calls `rpc.EncodeMessage` then writes

Note: Since `PublishDiagnostics` lives in the `util` package alongside `WriteResponse`, call `WriteResponse` directly (no package qualifier needed).

### 2. Modify `analysis/fusion/compile.go`

Remove the unexported `publishDiagnostics` function (lines 46-59).

Replace all call sites with `util.PublishDiagnostics`. There are two call sites in `FusionCompile`:

- Line 108: `publishDiagnostics(writer, uri, diagnostics)` inside the goroutine
- Line 133: `publishDiagnostics(writer, uri, diagnostics)` after `cmd.Wait()`

Both become `util.PublishDiagnostics(writer, uri, diagnostics)`.

The `util` import already exists in `compile.go` (line 21: `"github.com/j-clemons/dbt-language-server/util"`), so no new import is needed.

### Key details

- The function name is **exported** (`PublishDiagnostics` with capital P) since it will be called from both `analysis/fusion` and `analysis/diagnostics` packages.
- Fusion sets `Source` to `fmt.Sprintf("dbt Fusion %s", log.Data.Version)` on each diagnostic before calling publish. The shared utility does NOT override Source -- it publishes diagnostics as-is. The built-in diagnostics engine (section-02) will set `Source: "dbt-ls"` on diagnostics before calling this utility.
- The shared utility is intentionally minimal: construct notification struct, write it. No source-field manipulation, no filtering. Callers are responsible for setting fields on their diagnostics before publishing.

## Implementation Notes (Post-Implementation)

All files matched plan exactly. One deviation:
- Added nil-guard in `PublishDiagnostics`: `if diagnostics == nil { diagnostics = []lsp.Diagnostic{} }` — ensures JSON `[]` not `null` per LSP spec (found during code review).
- Plan test "sets Source to dbt-ls" was replaced with "preserves Source" to match the implementation decision (pass-through, no override).
- Plan test "byte-for-byte comparison" omitted — logic is identical, no value.

### Test count: 4 tests in 1 new test file
- `TestPublishDiagnosticsWritesValidNotification`
- `TestPublishDiagnosticsPreservesSource`
- `TestPublishDiagnosticsEmptySlice`
- `TestPublishDiagnosticsMethod`