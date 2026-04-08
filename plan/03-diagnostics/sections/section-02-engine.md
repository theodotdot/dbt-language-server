Now I have all the context needed. Let me produce the section content.

# Section 02: Core Diagnostic Engine

## Overview

This section creates the core diagnostic engine in `analysis/diagnostics/diagnostics.go`. The engine manages checker registration, per-URI debouncing (150ms timers), snapshotting document/context under RLock, running registered checkers against snapshots, and publishing results via the shared utility from section-01.

**Depends on**: section-01-shared-publish (shared `publishDiagnostics` utility must exist)

**Blocks**: section-03-refs, section-04-jinja, section-05-lifecycle, section-06-columns

## Key Concepts

### Checker Type

A `Checker` is a function that receives immutable snapshots of a document and dbt context, returning zero or more LSP diagnostics:

```go
type Checker func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic
```

Checkers never touch live `*State`. They operate on copies taken under lock.

### Engine Struct

```go
type Engine struct {
    writer   io.Writer
    state    *analysis.State
    mu       sync.Mutex         // protects timers map
    timers   map[string]*time.Timer  // URI -> pending debounce timer
    checkers []Checker
}
```

The engine holds a reference to `*analysis.State` to acquire `RLock` for snapshotting. The `mu` mutex protects the `timers` map from concurrent access (main goroutine calls `RunDebounced`, timer goroutines fire callbacks).

### Concurrency Model

The existing message loop is single-threaded. Debounced diagnostics fire from goroutines after 150ms, creating concurrent reads against main-loop writes. The engine's `runCheckers` acquires `state.mu.RLock()`, copies `Document` and `DbtContext`, releases the lock, then runs checkers against the copies. This pairs with the WLock additions to state lifecycle methods (done in section-05).

## File to Create

**`/home/theotime.poulain/dbt-language-server/analysis/diagnostics/diagnostics.go`**

Package: `diagnostics`

### Constructor

`NewEngine(writer io.Writer, state *analysis.State, checkers ...Checker) *Engine`

Creates the engine, initializes the `timers` map, stores checkers.

### RunDebounced(uri string)

1. Acquire `e.mu.Lock()`
2. If timer exists for URI, call `timer.Stop()`
3. Create new `time.AfterFunc(150*time.Millisecond, func() { e.runCheckers(uri) })`
4. Store in `e.timers[uri]`
5. Release `e.mu.Unlock()`

### RunImmediate(uri string)

1. Acquire `e.mu.Lock()`
2. If timer exists for URI, call `timer.Stop()`, delete from map
3. Release `e.mu.Unlock()`
4. Call `e.runCheckers(uri)` synchronously

### Clear(uri string)

1. Acquire `e.mu.Lock()`
2. If timer exists for URI, call `timer.Stop()`, delete from map
3. Release `e.mu.Unlock()`
4. Publish empty diagnostics array for URI using the shared publish utility

### runCheckers(uri string)

1. Check URI ends with `.sql` -- if not, return immediately (no publish for non-SQL files)
2. Acquire `e.state.mu.RLock()`  (note: `State.mu` is `sync.RWMutex` and already exists, but is currently unexported lowercase `mu` -- access pattern TBD, may need a method like `state.GetDocumentSnapshot(uri)` or the field needs to be exported)
3. Copy `Document` for the URI from `state.Documents[uri]`
4. Copy `DbtContext` from `state.DbtContext`
5. Release RLock
6. If document not found, publish empty array and return
7. Iterate all checkers, calling each with the document and context copies
8. Collect all returned diagnostics into a single slice
9. Publish via shared utility with source `"dbt-ls"`

### State Access Pattern

`State.mu` is unexported. Rather than exporting it, add a snapshot method to State:

```go
func (s *State) Snapshot(uri string) (Document, DbtContext, bool)
```

This method acquires RLock internally, copies the document and context, and returns them. The `bool` indicates whether the document exists. This keeps locking encapsulated in the State type.

## File Extension Filter

Only `.sql` URIs are processed. Check with `strings.HasSuffix(uri, ".sql")` in `runCheckers` before doing any work. The parser produces garbage tokens for YAML, so diagnosing non-SQL files would produce false positives.

## Publishing

Use the shared `publishDiagnostics` utility extracted in section-01. All diagnostics emitted by the built-in engine use `Source: "dbt-ls"`. The source field is set by each checker (or by the engine after collecting results -- the engine should overwrite/set the Source field on each diagnostic before publishing to ensure consistency).

## Tests

**File: `/home/theotime.poulain/dbt-language-server/analysis/diagnostics/diagnostics_test.go`**

Package: `diagnostics`

Test strategy: create a `*State` with a known document, construct an `Engine` with mock checkers, and verify behavior by inspecting what gets written to an `io.Writer` (use `bytes.Buffer`).

### Test List

```go
// Test: NewEngine creates engine with checkers registered
// - Pass 2 checkers to NewEngine, verify engine.checkers has length 2

// Test: RunImmediate runs all checkers and publishes diagnostics for URI
// - Register a checker that returns 1 diagnostic
// - Call RunImmediate with a .sql URI
// - Parse the JSON-RPC message from the buffer
// - Verify the diagnostic appears in the published notification

// Test: RunImmediate with no diagnostics publishes empty array
// - Register a checker returning nil
// - Verify published notification has empty Diagnostics array

// Test: RunDebounced fires after 150ms delay
// - Call RunDebounced, verify no output immediately
// - Sleep ~200ms, verify output appeared

// Test: RunDebounced resets timer on subsequent calls (only last fires)
// - Call RunDebounced, wait 50ms, call RunDebounced again
// - Wait 200ms, verify only one publish occurred (not two)

// Test: RunImmediate cancels pending debounced timer
// - Call RunDebounced, immediately call RunImmediate
// - Wait 200ms, verify only one publish occurred (the immediate one)

// Test: Clear publishes empty diagnostic array for URI
// - Verify Clear writes a notification with empty Diagnostics

// Test: Clear cancels pending debounced timer for URI
// - Call RunDebounced, then Clear before timer fires
// - Wait 200ms, verify only the Clear publish occurred (empty array)

// Test: Multiple checkers -- diagnostics from all checkers are combined into single publish
// - Register 2 checkers each returning 1 diagnostic
// - Call RunImmediate, verify published notification has 2 diagnostics

// Test: Non-.sql URI is skipped (no publish)
// - Call RunImmediate with a .yml URI
// - Verify nothing written to buffer
```

### Test Helpers

For parsing output from the buffer, strip the `Content-Length: N\r\n\r\n` header prefix and JSON-unmarshal the `lsp.DiagnosticsNotification`. A helper like `parseNotification(t *testing.T, buf *bytes.Buffer) lsp.DiagnosticsNotification` simplifies repeated assertions.

For creating test state: populate `State.Documents` with a minimal document (can have nil Tokens and empty DefTokens for engine-level tests since the checkers are mocked). The `DbtContext` can use empty maps.

### Publishing Tests

These verify the shared publish utility (from section-01) but are listed here for completeness:

```go
// Test: publish writes valid LSP DiagnosticsNotification to writer
// Test: publish sets Source field to "dbt-ls" on all diagnostics
// Test: published notification has correct JSON-RPC method "textDocument/publishDiagnostics"
```

If the shared utility is already tested in section-01, these can be skipped here. The engine tests above implicitly validate publishing through the buffer assertions.

## Implementation Notes (Post-Implementation)

All files matched plan. Deviations:
- Added `State.Snapshot(uri)` method as planned, using `RLock`
- Timer callback now cleans up `e.timers[uri]` after firing (code review fix — prevents unbounded map growth)
- Publishing tests skipped here — covered in section-01

### Test count: 10 tests in 1 new test file
- `TestNewEngineRegistersCheckers`
- `TestRunImmediatePublishesDiagnostics`
- `TestRunImmediateNoDiagnosticsPublishesEmptyArray`
- `TestRunDebouncedFiresAfterDelay`
- `TestRunDebouncedResetsTimer`
- `TestRunImmediateCancelsPendingDebounce`
- `TestClearPublishesEmptyDiagnostics`
- `TestClearCancelsPendingDebounce`
- `TestMultipleCheckersCombineDiagnostics`
- `TestNonSqlURISkipped`