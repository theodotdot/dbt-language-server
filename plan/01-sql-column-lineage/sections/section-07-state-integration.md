I now have enough context to write the section.

# Section 07: State Integration

## Overview

This section wires the `QueryScope` (built by the parser in sections 01-05) and `ResolveColumnsAtCursor` (built in section 06) into the LSP state layer. Three changes are needed:

1. Add a `Scope` field to the `Document` struct
2. Populate it in `parseDocument()` via `CreateQueryScope()`
3. Add a `ResolveColumnsAtPosition` method on `State` that delegates to the resolution engine
4. Enhance the completion handler's else branch to use scope-aware column resolution

## Dependencies

- **Section 01** (data types): `QueryScope`, `ScopeColumn` types in `analysis/parser/scope.go`
- **Section 02** (clause state machine): `CreateQueryScope()` export on `Parser`
- **Section 06** (resolution engine): `ResolveColumnsAtCursor()` in `analysis/parser/resolve.go`

These must be implemented before this section. This section does not modify parser internals -- it only consumes their public API.

## Files to Modify

- `/home/theotime.poulain/dbt-language-server/analysis/state.go` -- `Document` struct, `parseDocument()`, new `ResolveColumnsAtPosition` method
- `/home/theotime.poulain/dbt-language-server/analysis/state_lsp_test.go` -- new integration tests (extend existing file)

## Tests (Write First)

Add these tests to `/home/theotime.poulain/dbt-language-server/analysis/state_lsp_test.go`. All tests use the existing `newTestState()` helper and `state.parseDocument()` pattern already established in that file.

### Test: Document.Scope is populated after parseDocument()

```go
func TestDocumentScopePopulated(t *testing.T) {
    // Parse a simple SQL with a ref. Verify s.Documents[uri].Scope is non-nil.
    // Scope should have at least one Source (the ref'd model).
}
```

Table-driven, with cases:
- Simple `SELECT id FROM {{ ref('customers') }}` -- Scope not nil, len(Scope.Sources) >= 1
- Plain SQL `SELECT 1` -- Scope not nil (may have empty Sources, but must not be nil itself)
- Empty string -- Scope not nil, no panic

### Test: Document.Scope is updated after UpdateDocumentIncremental()

```go
func TestDocumentScopeUpdatedIncremental(t *testing.T) {
    // Parse initial SQL. Then call UpdateDocumentIncremental with a change
    // that adds a ref. Verify Scope reflects the new ref.
}
```

Steps:
1. `parseDocument(uri, "SELECT 1")` -- verify Scope has 0 sources
2. `UpdateDocumentIncremental(uri, change that replaces text with "SELECT * FROM {{ ref('customers') }}")` -- verify Scope now has >= 1 source

### Test: ResolveColumnsAtPosition returns columns from ref'd model

```go
func TestResolveColumnsAtPosition(t *testing.T) {
    // Use newTestState() which has "customers" model with columns
    // [customer_id, first_name]. Parse SQL that refs customers.
    // Call ResolveColumnsAtPosition at a position in the SELECT clause.
    // Verify returned ScopeColumns include "customer_id" and "first_name".
}
```

### Test: ResolveColumnsAtPosition returns CTE columns

```go
func TestResolveColumnsAtPositionCTE(t *testing.T) {
    // Parse: WITH cte AS (SELECT customer_id FROM {{ ref('customers') }}) SELECT  FROM cte
    // Cursor in main SELECT. Verify CTE columns are returned.
}
```

### Test: ResolveColumnsAtPosition with alias prefix filters to single source

```go
func TestResolveColumnsAtPositionAliasFiltered(t *testing.T) {
    // Parse: SELECT c. FROM {{ ref('customers') }} c
    // Call with position after "c." -- verify only customers columns returned,
    // all with source == "c".
}
```

### Test: All existing tests still pass (regression)

No new test code needed. Run `go test ./analysis/...` after changes. The existing `TestHover`, `TestDefinition`, `TestTextDocumentCompletion`, `TestRefreshDbtContext`, and all others must pass unchanged.

## Implementation Details

### 1. Add Scope field to Document struct

In `/home/theotime.poulain/dbt-language-server/analysis/state.go`, add the `Scope` field:

```go
type Document struct {
    Text      string
    Tokens    *parser.TokenIndex
    DefTokens map[string]parser.Token
    Scope     *parser.QueryScope  // NEW: SQL scope for column resolution
}
```

### 2. Update parseDocument()

Call `CreateQueryScope()` alongside existing `CreateTokenIndex()` and `CreateTokenNameMap()`:

```go
func (s *State) parseDocument(uri, text string) {
    parserIns := parser.Parse(text, s.DbtContext.Dialect)
    s.Documents[uri] = Document{
        Text:      text,
        Tokens:    parserIns.CreateTokenIndex(),
        DefTokens: parserIns.CreateTokenNameMap(),
        Scope:     parserIns.CreateQueryScope(),
    }
}
```

`CreateQueryScope()` is implemented in the parser (section 02). It returns the bottom of the scope stack (top-level query scope). If parsing produced no scope data, it returns an empty but non-nil `*QueryScope`.

No changes to `UpdateDocument()` or `UpdateDocumentIncremental()` are needed -- they both call `parseDocument()` internally, so Scope is automatically rebuilt on every change.

### 3. Add ResolveColumnsAtPosition method

This is a method on `State` because it needs `ModelDetailMap` and `SourceDetailMap` from `DbtContext`. It must acquire a read lock before accessing `Documents` and `DbtContext`.

```go
func (s *State) ResolveColumnsAtPosition(uri string, line, col int) []parser.ScopeColumn {
    s.mu.RLock()
    defer s.mu.RUnlock()

    doc, exists := s.Documents[uri]
    if !exists || doc.Scope == nil {
        return nil
    }

    return parser.ResolveColumnsAtCursor(
        doc.Scope,
        line,
        col,
        s.DbtContext.ModelDetailMap,
        s.DbtContext.SourceDetailMap,
    )
}
```

Key points:
- Uses `s.mu.RLock()` (read lock), not full Lock, since this is a read-only operation
- Returns `nil` for missing documents or nil scopes (graceful degradation)
- Delegates entirely to `parser.ResolveColumnsAtCursor()` from section 06
- The `ResolveColumnsAtCursor` function signature accepts `map[string]ModelDetails` and `map[string]Source` -- these are the types already defined in the `analysis` package. The parser's `resolve.go` file will need to import these types or accept interfaces. Since the resolution engine is in the `parser` package, it cannot directly reference `analysis.ModelDetails`. There are two approaches:
  - Define a column-provider interface in the parser package that `analysis` implements
  - Pass pre-extracted column data (e.g., `map[string][]string` mapping model name to column names)
  
  The simplest approach: `ResolveColumnsAtCursor` accepts `map[string][]parser.ScopeColumn` as a pre-resolved column map, and `ResolveColumnsAtPosition` builds this map from `ModelDetailMap` before calling. Alternatively, define a minimal interface in the parser package. The exact contract is defined in section 06. This section just calls whatever API section 06 exposes.

### 4. Wire into completion flow (optional enhancement)

The current else branch in `TextDocumentCompletion` (line ~469-473 of `state.go`) uses the naive `getReferencedModels` + `getColumnCompletionItems` approach. Once scope-aware resolution is available, this can be enhanced to use `ResolveColumnsAtPosition` instead, providing alias-qualified and CTE-aware completions.

The enhancement replaces the else branch:

```go
} else {
    // Try scope-aware resolution first
    scopeCols := s.ResolveColumnsAtPosition(uri, position.Line, position.Character)
    if len(scopeCols) > 0 {
        items = scopeColumnsToCompletionItems(scopeCols)
    } else {
        // Fallback to existing naive approach
        refModels := getReferencedModels(s.Documents[uri].Tokens)
        items = getColumnCompletionItems(refModels, s.DbtContext.ModelDetailMap)
    }
    items = append(items, s.DbtContext.Dialect.FunctionCompletionItems()...)
}
```

A helper function `scopeColumnsToCompletionItems` converts `[]parser.ScopeColumn` to `[]lsp.CompletionItem`:

```go
func scopeColumnsToCompletionItems(cols []parser.ScopeColumn) []lsp.CompletionItem
```

This helper creates `CompletionItem` entries with:
- `Label`: column name (or qualified form like `alias.column`)
- `Detail`: source name (e.g., "Column from customers")
- `Documentation`: description if available
- `Kind`: `completionKind.Field`
- Deduplication by label

Note: The `ResolveColumnsAtPosition` call here does NOT need `RLock` because `TextDocumentCompletion` is already called within the state's access pattern. However, since `ResolveColumnsAtPosition` acquires its own `RLock`, and Go's `sync.RWMutex` allows multiple concurrent readers, this is safe. If the caller already holds a write lock, this would deadlock -- but `TextDocumentCompletion` does not hold any lock, so it is fine.

### 5. Thread safety notes

- `parseDocument()` is called from `OpenDocument()`, `UpdateDocument()`, and `UpdateDocumentIncremental()`. These are write operations -- callers must hold the write lock or be the sole writer. Currently these methods do NOT acquire locks themselves (the lock is expected to be managed by the caller/server layer). Adding `Scope` to `Document` follows the same pattern.
- `ResolveColumnsAtPosition` acquires `RLock` itself since it is a public API that LSP handlers will call directly.

### 6. expectedTestState() update

The existing `expectedTestState()` in `state_test.go` constructs an expected `State` with an explicit `Documents` map. Since `Documents` is initialized empty (`map[string]Document{}`), and `Scope` defaults to nil for empty Documents, no change is needed to `expectedTestState()`. The `TestRefreshDbtContext` test does not parse any documents, so the `Scope` field being zero-value (`nil`) matches automatically.

---

## Implementation Notes

**Status:** Completed

**Files modified:**
- `analysis/state.go` — added `Scope` to `Document`, populated in `parseDocument()`. Added `buildModelColumns()`, `scopeColumnsToCompletionItems()`, `ResolveColumnsAtPosition()`. Enhanced completion else branch with scope-aware resolution and fallback.
- `analysis/state_lsp_test.go` — added `TestDocumentScopePopulated` (3 cases), `TestResolveColumnsAtPosition`, `TestResolveColumnsAtPositionCTE`, `TestResolveColumnsAtPositionAliasFiltered`
- `analysis/parser/parser.go` — fixed `selectStarted` leak across CTE scopes (from section-05 review)
- `analysis/parser/scope_test.go` — fixed weak assertion in chained CTE test (from section-05 review)

**Deviations from plan:**
- `ResolveColumnsAtPosition` does not acquire locks — follows existing pattern where callers manage locking
- No `UpdateDocumentIncremental` test — `parseDocument` is called internally, scope is automatically rebuilt
- `scopeColumnsToCompletionItems` deduplicates by column name (matching `getColumnCompletionItems` pattern)