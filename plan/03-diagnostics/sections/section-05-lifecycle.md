Now I have all the context needed. Let me produce the section content.

# Section 5: Diagnostic Lifecycle Integration

## Overview

Wire the diagnostic engine into `main.go`: initialize the engine when fusion is disabled, hook `didOpen`/`didChange`/`didSave`/`didClose` handlers to trigger diagnostics, filter by `.sql` URI extension. Also add write locking to `State` document lifecycle methods to prevent data races with debounce goroutines.

## Dependencies

- **section-01-shared-publish**: shared `publishDiagnostics` utility must exist
- **section-02-engine**: `diagnostics.Engine` with `NewEngine`, `RunImmediate`, `RunDebounced`, `Clear` must exist
- **section-03-refs**: ref/source/var/macro checkers must be registered
- **section-04-jinja**: jinja syntax checkers must be registered

## Tests

File: `analysis/diagnostics/integration_test.go`

These tests validate the lifecycle wiring. The engine itself is tested in section-02; these tests confirm the integration points work correctly.

```go
// Test: didOpen triggers immediate diagnostics for .sql file
// Test: didOpen does NOT trigger diagnostics for .yml file
// Test: didChange triggers debounced diagnostics
// Test: didSave triggers immediate diagnostics
// Test: didClose publishes empty diagnostics array
// Test: engine is nil when FusionEnabled is true (no built-in diagnostics)
// Test: engine is created when FusionEnabled is false
```

File: `analysis/state_test.go` (or `analysis/diagnostics/integration_test.go`)

```go
// Test: concurrent RLock from runCheckers doesn't deadlock with WLock from OpenDocument
// Test: document update during debounce period — checker sees latest state (re-reads under lock)
```

### Test approach

For lifecycle tests, create a `bytes.Buffer` as the writer, set up a `State` with a known document and `DbtContext`, instantiate the engine, then call the handler logic and assert on what gets written to the buffer. Use short debounce timeouts or `RunImmediate` for deterministic assertions.

For locking tests, use goroutines that concurrently call `OpenDocument` (write path) and `engine.RunImmediate` (read path) in a loop. The test passes if it completes without deadlock or race detector complaints (`go test -race`).

## Implementation

### 1. Add `didClose` LSP notification type

File: `/home/theotime.poulain/dbt-language-server/lsp/textdocument_didclose.go` (create)

Define the `DidCloseTextDocumentNotification` type. This type does not exist in the codebase yet. It mirrors the structure of `DidSaveTextDocumentNotification`:

```go
package lsp

type DidCloseTextDocumentNotification struct {
	Notification
	Params DidCloseTextDocumentParams `json:"params"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}
```

### 2. Add write locking to State document lifecycle methods

File: `/home/theotime.poulain/dbt-language-server/analysis/state.go` (modify)

The `State` struct already has a `sync.RWMutex` field (`mu`). Currently, `SetFusionEnabled` and `IsFusionEnabled` use it, but none of the document lifecycle methods acquire the lock.

Add `mu.Lock()` / `defer mu.Unlock()` to these methods:

- `OpenDocument` — wraps the entire method body (including `refreshDbtContext` and `parseDocument`)
- `UpdateDocument` — wraps `parseDocument` call
- `UpdateDocumentIncremental` — wraps the entire method body (reads `s.Documents[uri]`, applies changes, calls `parseDocument`)
- `SaveDocument` — wraps `refreshDbtContext` call

This pairs with the `RLock` the diagnostic engine acquires in `runCheckers` (section-02). The main message loop writes through these methods; debounce goroutines read via `RLock`.

**Important**: `refreshDbtContext` does file I/O (reads `dbt_project.yml`, scans model/macro dirs). Holding the write lock during this is acceptable because the debounce goroutines only need the lock briefly to snapshot, and the main loop is single-threaded (no self-deadlock risk). However, if latency becomes a concern, a future optimization could narrow the lock scope.

### 3. Initialize diagnostic engine in main.go

File: `/home/theotime.poulain/dbt-language-server/main.go` (modify)

Add import for `"github.com/j-clemons/dbt-language-server/analysis/diagnostics"`.

After the `state` initialization and before the scanner loop, conditionally create the engine:

```go
var diagEngine *diagnostics.Engine
if *fusion == "" {
    diagEngine = diagnostics.NewEngine(writer, &state)
}
```

The engine is only created when no `--fusion` flag is provided. When `--fusion` is set, `diagEngine` remains `nil` and all diagnostic calls are no-ops (guarded by nil checks).

Note: checking `*fusion == ""` rather than `state.FusionEnabled` because fusion validation runs asynchronously in a goroutine and `FusionEnabled` may not be set yet at this point. The `--fusion` flag string is the authoritative signal.

### 4. Wire handlers in handleMessage

File: `/home/theotime.poulain/dbt-language-server/main.go` (modify)

The `handleMessage` function needs to accept the engine as a parameter. Update its signature:

```go
func handleMessage(logger *log.Logger, writer io.Writer, state *analysis.State, diagEngine *diagnostics.Engine, method string, contents []byte)
```

Update the call site in the `for scanner.Scan()` loop accordingly.

Then add diagnostic calls to the existing cases and a new `didClose` case:

**`textDocument/didOpen`** — after `state.OpenDocument(...)` and `fusion.FusionCompile(...)`, add:

```go
if diagEngine != nil && strings.HasSuffix(request.Params.TextDocument.URI, ".sql") {
    diagEngine.RunImmediate(request.Params.TextDocument.URI)
}
```

**`textDocument/didChange`** — after `state.UpdateDocumentIncremental(...)`, add:

```go
if diagEngine != nil && strings.HasSuffix(request.Params.TextDocument.URI, ".sql") {
    diagEngine.RunDebounced(request.Params.TextDocument.URI)
}
```

**`textDocument/didSave`** — after `state.SaveDocument(...)` and `fusion.FusionCompile(...)`, add:

```go
if diagEngine != nil && strings.HasSuffix(request.Params.TextDocument.URI, ".sql") {
    diagEngine.RunImmediate(request.Params.TextDocument.URI)
}
```

**`textDocument/didClose`** (new case) — add between existing cases:

```go
case "textDocument/didClose":
    var request lsp.DidCloseTextDocumentNotification
    if err := json.Unmarshal(contents, &request); err != nil {
        logger.Printf("textDocument/didClose: %s", err)
        return
    }
    logger.Printf("Closed: %s", request.Params.TextDocument.URI)
    if diagEngine != nil {
        diagEngine.Clear(request.Params.TextDocument.URI)
    }
```

The `diagEngine.Clear()` call publishes an empty diagnostics array for the URI, which clears any displayed diagnostics in the editor. It also cancels any pending debounce timer for that URI.

Optionally, also remove the document from `state.Documents` to free memory. This is not strictly required since opening the same file again will overwrite the entry, but it prevents stale data accumulation:

```go
delete(state.Documents, request.Params.TextDocument.URI)
```

If adding document deletion, it should be done under write lock. Add a `CloseDocument` method to `State` that acquires `mu.Lock()`, deletes the entry, and unlocks.

### 5. URI extension filter

The `.sql` extension check uses `strings.HasSuffix(uri, ".sql")`. This is applied at the call site in `handleMessage` rather than inside the engine, keeping the engine generic. Non-SQL files (`.yml`, `.md`, `.py`) are never sent to the engine.

The `strings` package is already imported in `main.go` (not currently, but add it). Alternatively, the engine's `RunImmediate`/`RunDebounced` methods could perform this check internally — either approach works. The plan specifies the filter in `main.go` for visibility.

## File Summary

| File | Action |
|------|--------|
| `/home/theotime.poulain/dbt-language-server/lsp/textdocument_didclose.go` | Create — `DidCloseTextDocumentNotification` type |
| `/home/theotime.poulain/dbt-language-server/analysis/state.go` | Modify — add `mu.Lock()`/`mu.Unlock()` to `OpenDocument`, `UpdateDocument`, `UpdateDocumentIncremental`, `SaveDocument`; optionally add `CloseDocument` method |
| `/home/theotime.poulain/dbt-language-server/main.go` | Modify — import diagnostics package, create engine, update `handleMessage` signature, add diagnostic calls to didOpen/didChange/didSave, add didClose case |
| `/home/theotime.poulain/dbt-language-server/analysis/diagnostics/integration_test.go` | Create — lifecycle and locking integration tests |

## Edge Cases

- **Engine is nil when fusion enabled**: all `diagEngine` calls are guarded by `diagEngine != nil`. No panics.
- **Non-.sql URIs**: filtered out by `strings.HasSuffix` before engine calls. YAML files (which produce garbage tokens) never trigger diagnostics.
- **didClose for file not in Documents map**: `Clear` just publishes empty array; no error if URI was never opened.
- **Rapid open/close cycle**: if didClose arrives while a debounce timer is pending, `Clear` cancels the timer before publishing empty diagnostics.
- **Fusion validation race**: engine creation uses `*fusion == ""` (the CLI flag string), not `state.FusionEnabled` (which is set asynchronously). This avoids a race condition where the engine might or might not be created depending on goroutine scheduling.

## Implementation Notes (Post-Implementation)

### Deviations from plan
- Added `safeBuffer` wrapper in integration tests to avoid race on `bytes.Buffer` (race detector caught concurrent writes from debounce goroutine and test assertion).
- `didClose` doesn't filter by `.sql` — intentional per plan, Clear on non-.sql is harmless.
- Engine already filters `.sql` internally in `runCheckers()` — double filter with call-site check provides defense-in-depth.
- `writer := os.Stdout` moved before engine creation to avoid use-before-declare.

### Files created
- `lsp/textdocument_didclose.go` — DidCloseTextDocumentNotification type
- `analysis/diagnostics/integration_test.go` — 8 tests (lifecycle + concurrent locking)

### Files modified
- `analysis/state.go` — mu.Lock in OpenDocument/UpdateDocument/UpdateDocumentIncremental/SaveDocument + CloseDocument method
- `main.go` — engine init, handleMessage signature, didOpen/didChange/didSave hooks, didClose handler

### Test count: 8 (didOpen, nonSqlSkipped x2, didChange debounced, didClose, engineNil, concurrentLocking, allCheckers, publishFormat)