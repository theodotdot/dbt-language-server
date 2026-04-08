I have all the context. Now I can write the section.

# Section 03: Scope-Based Column Completion

## Overview

This section replaces the current `getReferencedModels()` fallthrough path in `TextDocumentCompletion` with scope-based column resolution. It introduces a new `getScopeColumnCompletionItems()` function, adds source column resolution via post-processing of `SourceKindSource` entries, and maintains backward compatibility when `doc.Scope` is nil.

## Dependencies

- **section-01-source-columns**: Provides `Columns []Column` on `SourceTable` and `SourceTableProperties`. Without this, source column resolution returns nothing.
- **Split 01 (sql-column-lineage)**: Provides `parser.QueryScope`, `parser.ScopeColumn`, `parser.SourceRef`, `parser.SourceKindSource`, `Document.Scope`, and `State.ResolveColumnsAtPosition()`. The scope-based path requires these types/methods to exist.

## Files to Modify

- `/home/theotime.poulain/dbt-language-server/analysis/get_completion_items.go` -- add `getScopeColumnCompletionItems()` function
- `/home/theotime.poulain/dbt-language-server/analysis/state.go` -- update `TextDocumentCompletion` else branch
- `/home/theotime.poulain/dbt-language-server/analysis/get_completion_items_test.go` -- unit tests for new function
- `/home/theotime.poulain/dbt-language-server/analysis/state_lsp_test.go` -- integration tests for scope-based completion

---

## Tests First

### Unit tests: `getScopeColumnCompletionItems`

File: `/home/theotime.poulain/dbt-language-server/analysis/get_completion_items_test.go`

Add a `TestGetScopeColumnCompletionItems` function with table-driven subtests. Uses `parser.ScopeColumn` as input. Requires import of `github.com/j-clemons/dbt-language-server/analysis/parser`.

Test cases:

1. **Single source** -- Input: one `ScopeColumn{Name: "customer_id", Source: "customers"}`. Verify: returned item has `Label: "customer_id"`, `Kind: completionKind.Field`, `Detail` contains "customers", `SortText` starts with `"0"`.

2. **Multiple sources, no deduplication** -- Input: two ScopeColumns with the same `Name: "id"` but different `Source` values ("orders" and "customers"). Verify: two items returned (not one), each with different `Detail` showing the source name.

3. **Duplicate column names from different sources** -- Same as above but verify `Detail` fields are `"Column from orders"` and `"Column from customers"` respectively, allowing the user to distinguish.

4. **With alias** -- Input: `ScopeColumn{Name: "total", Source: "orders", Qualified: "o.total"}` where Source is "orders" but there's an alias context. The `Detail` should show the source name. The exact format: `"Column from orders"`.

5. **SortText has "0" prefix** -- Verify every returned item's `SortText` equals `"0" + columnName`.

6. **Empty input** -- Input: nil/empty slice. Returns empty (not nil) slice.

7. **Description carried through** -- Input: `ScopeColumn{Name: "amount", Source: "payments", Description: "Payment amount"}`. Verify `Documentation: "Payment amount"`.

### Integration tests: scope-based column path

File: `/home/theotime.poulain/dbt-language-server/analysis/state_lsp_test.go`

These tests verify the full dispatch through `TextDocumentCompletion` when `doc.Scope` is non-nil. They depend on split 01 being implemented (specifically `Document.Scope` and `ResolveColumnsAtPosition`).

Add tests as part of a `TestScopeColumnCompletion` function or extend the existing completion test structure.

Test cases:

1. **Source column completion** -- Set up a document with `SELECT \n FROM {{ source('my_source', 'my_table') }}`. The `newTestState()` source must have `Columns` populated on its `SourceTable` (depends on section-01-source-columns). Position cursor in SELECT clause. Verify completion items include source columns.

2. **Source with no columns** -- Same setup but source table has empty `Columns`. Verify no column items returned for that source (only dialect functions).

3. **Backward compatibility: nil Scope** -- Create a document without triggering scope population (or explicitly set `doc.Scope = nil`). Call `TextDocumentCompletion` at a position that would hit the else branch. Verify the old `getReferencedModels()` + `getColumnCompletionItems()` path executes (items should match model columns from `ModelDetailMap`).

4. **CTE column completion** -- Parse SQL with a CTE. Position cursor in the outer SELECT. Verify CTE output columns appear in completion. (Requires split 01 CTE parsing.)

5. **JOIN column merge** -- Parse SQL with two JOINed refs. Position cursor in SELECT. Verify columns from both sources appear, including duplicate column names (e.g., `id` from both tables) with different `Detail` values.

6. **JOIN with duplicate column names** -- Specifically verify that when `orders` and `customers` both have `customer_id`, both appear in the completion list with source attribution in `Detail`.

---

## Implementation Details

### `getScopeColumnCompletionItems` function

File: `/home/theotime.poulain/dbt-language-server/analysis/get_completion_items.go`

```go
func getScopeColumnCompletionItems(columns []parser.ScopeColumn) []lsp.CompletionItem {
    // Convert each ScopeColumn to a CompletionItem.
    // No deduplication -- all columns shown even if names collide.
    // Detail format: "Column from <Source>"
    // SortText: "0" + column name (higher priority than functions at "2")
    // Kind: completionKind.Field
    // Documentation: ScopeColumn.Description
}
```

Key behavior differences from existing `getColumnCompletionItems`:
- **No deduplication**: the old function uses a `seen` map to dedup by column name across models. The new function intentionally shows all columns, using `Detail` to distinguish sources. This is correct for JOIN scenarios where users need `o.id` vs `c.id`.
- **SortText prefix**: uses `"0"` prefix instead of bare column name. This gives columns higher priority over SQL functions (which will use `"2"` prefix after section-06-sort-text).
- Takes `[]parser.ScopeColumn` instead of model names + model map.

### Source column resolution via post-processing

The scope engine (split 01) returns empty columns for `SourceKindSource` entries because it has no access to `SourceDetailMap`. This section must bridge that gap.

In `TextDocumentCompletion`'s else branch (or in a helper called from there), after calling `ResolveColumnsAtPosition`, post-process the result:

1. Get the document's `Scope`
2. Iterate `Scope.Sources` looking for entries with `Kind == parser.SourceKindSource`
3. For each such source, look up `s.DbtContext.SourceDetailMap[sourceRef.SourceName].Tables[sourceRef.Name].Columns`
4. Convert those `Column` values to `parser.ScopeColumn` values (setting `Source` to the source table name, `Description` from the column)
5. Append to the columns list from `ResolveColumnsAtPosition`

This post-processing approach avoids modifying split 01's `ResolveColumnsAtPosition` API.

Pseudocode for the helper:

```go
func (s *State) resolveSourceColumns(scope *parser.QueryScope) []parser.ScopeColumn {
    // For each source in scope.Sources where Kind == SourceKindSource:
    //   look up s.DbtContext.SourceDetailMap[source.SourceName]
    //   for each table column, create ScopeColumn{Name, Source: source.Name, Description}
    // Return collected columns
}
```

### Updated dispatch in `TextDocumentCompletion`

File: `/home/theotime.poulain/dbt-language-server/analysis/state.go`

The `else` branch (currently lines 469-472) changes from:

```go
} else {
    refModels := getReferencedModels(s.Documents[uri].Tokens)
    items = getColumnCompletionItems(refModels, s.DbtContext.ModelDetailMap)
    items = append(items, s.DbtContext.Dialect.FunctionCompletionItems()...)
}
```

To:

```go
} else {
    doc := s.Documents[uri]
    if doc.Scope != nil {
        // Scope-based resolution (split 01 present)
        columns := s.ResolveColumnsAtPosition(uri, position)
        columns = append(columns, s.resolveSourceColumns(doc.Scope)...)
        items = getScopeColumnCompletionItems(columns)
    } else {
        // Fallback: old path (split 01 not deployed or parse failure)
        refModels := getReferencedModels(doc.Tokens)
        items = getColumnCompletionItems(refModels, s.DbtContext.ModelDetailMap)
    }
    items = append(items, s.DbtContext.Dialect.FunctionCompletionItems()...)
}
```

Key points:
- `doc.Scope != nil` check provides backward compatibility
- `FunctionCompletionItems()` is appended regardless of which path runs
- `resolveSourceColumns` is called separately and appended to scope results
- `ResolveColumnsAtPosition` is a method on `State` added by split 01's section-07

### Types from split 01 consumed here

These types must exist in `analysis/parser/scope.go` (created by split 01):

- `ScopeColumn` -- struct with fields: `Name string`, `Source string`, `Qualified string`, `Description string`
- `QueryScope` -- struct with `Sources []*SourceRef` field
- `SourceRef` -- struct with `Kind SourceKind`, `Name string`, `SourceName string`, `Alias string`
- `SourceKindSource` -- constant of type `SourceKind`

And this method must exist on `State` (added by split 01 section-07):

- `ResolveColumnsAtPosition(uri string, position lsp.Position) []parser.ScopeColumn`

And this field must exist on `Document` (added by split 01 section-07):

- `Scope *parser.QueryScope`

### Test state updates

The `newTestState()` in `state_lsp_test.go` must have `Columns` on its source table entries for source column completion tests to work. After section-01-source-columns adds the `Columns` field to `SourceTable`, update `newTestState()`:

```go
"my_table": {
    Name:        "my_table",
    Description: "Table description",
    Table:       "my_source",
    URI:         "/test/models/schema.yml",
    Range:       lsp.Range{...},
    Columns: []Column{
        {Name: "payment_id", Description: "Payment identifier"},
        {Name: "amount", Description: "Payment amount"},
    },
},
```

This ensures the source column post-processing path has data to work with.

## Implementation Notes (Post-Implementation)

Deviations from plan:
- Replaced existing `scopeColumnsToCompletionItems` (from plan 01) instead of creating alongside it. The old function deduplicated; the new `getScopeColumnCompletionItems` does not (intentional for JOIN scenarios).
- Removed `completionKind` import from `state.go` (no longer needed after removing `scopeColumnsToCompletionItems`).
- Source columns inside CTEs are not resolved — known limitation. The parser's `resolveSourceColumns` (resolve.go:49) also returns nil for SourceKindSource. Fixing requires scope traversal refactoring.
- `resolveSourceColumns` on State uses top-level `doc.Scope`, not position-aware scope. Same root cause as the CTE limitation.

### Files modified/created
- `analysis/get_completion_items.go` — MODIFIED (added `getScopeColumnCompletionItems`, added parser import)
- `analysis/state.go` — MODIFIED (replaced `scopeColumnsToCompletionItems` with `resolveSourceColumns`, updated dispatch, removed `completionKind` import)
- `analysis/get_completion_items_test.go` — MODIFIED (added 6 unit subtests)
- `analysis/state_lsp_test.go` — MODIFIED (added 3 integration subtests)

### Test count: 6 unit subtests + 3 integration subtests, all existing tests pass