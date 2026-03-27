Now I have all the context needed. Let me produce the section content.

# Section 06: Resolution Engine

## Overview

This section implements the column resolution engine in a new file `analysis/parser/resolve.go`. Given a `QueryScope` (built by sections 03-05), a cursor position, and model/source metadata maps, it resolves which columns are available for completion at that position.

**Dependencies**: Sections 01 (data types), 03 (select parsing), 04 (from/join parsing), 05 (CTE parsing) must be complete. The `QueryScope`, `SourceRef`, `CTEDef`, `SelectItem`, `ScopeColumn`, `ClauseKind`, and `SourceKind` types from section 01 are used directly. The scope is populated by sections 03-05.

**Output**: A new file `/home/theotime.poulain/dbt-language-server/analysis/parser/resolve.go` containing the `ResolveColumnsAtCursor` function.

---

## Tests (resolve_test.go)

File: `/home/theotime.poulain/dbt-language-server/analysis/parser/resolve_test.go`

All tests are table-driven. They construct `QueryScope` structs manually (no parsing) and call `ResolveColumnsAtCursor` with mock model/source maps.

### Test cases

1. **Resolve columns from ref'd model**: Scope has one `SourceRef{Kind: SourceKindRef, Name: "orders"}`. Mock `ModelDetailMap` has `"orders"` with columns `["id", "status", "amount"]`. Verify `ResolveColumnsAtCursor` returns 3 `ScopeColumn` entries with correct `Name`, `Source`, and `Qualified` fields.

2. **Resolve columns from CTE**: Scope has one `SourceRef{Kind: SourceKindCTE, Name: "my_cte"}`. Scope's `CTEs` map has `"my_cte"` with `Columns: ["id", "total"]`. Verify 2 `ScopeColumn` entries returned with `Source: "my_cte"`.

3. **Lazy SELECT * expansion through CTE**: CTE has `Columns: ["*"]` and its internal scope has a `SourceRef{Kind: SourceKindRef, Name: "orders"}`. Mock `ModelDetailMap` has `"orders"` with columns `["id", "status"]`. Verify the `"*"` is expanded to the model's columns.

4. **Alias-qualified access**: Scope has source with `Alias: "o"`, `Name: "orders"`. Call resolution with alias prefix `"o"`. Verify only columns from that source are returned.

5. **Multiple sources merged (JOIN)**: Scope has 2 sources — `orders` (alias `o`) and `customers` (alias `c`). Both have columns in `ModelDetailMap`. Verify all columns from both sources are returned.

6. **Fallback — unknown table source**: Scope has `SourceRef{Kind: SourceKindTable, Name: "some_table"}`. No entry in any map. Verify empty columns for that source (no panic).

7. **Source columns return empty**: `SourceRef{Kind: SourceKindSource, Name: "payments", SourceName: "stripe"}`. Verify zero columns returned (known limitation — Source struct lacks per-table column data).

8. **Case-insensitive column matching**: Model has column `"ID"`. Verify it appears in results. The resolution does not deduplicate — it returns columns as-is from the model map.

9. **Qualified and unqualified forms**: For source with alias `"o"` and column `"id"`, verify the returned `ScopeColumn` has `Name: "id"`, `Source: "o"`, `Qualified: "o.id"`.

10. **Cursor clause identification**: Scope has `ClauseRanges` with `ClauseSelect` at line 1 and `ClauseFrom` at line 2. Cursor at line 1 col 10 is in SELECT clause. Cursor at line 2 col 5 is in FROM clause. Verify the function identifies the correct clause (this is internal but testable via the resolution results — in both clauses, columns from FROM sources should be available).

---

## Implementation Details

### File: `/home/theotime.poulain/dbt-language-server/analysis/parser/resolve.go`

### Function Signature

```go
func ResolveColumnsAtCursor(
    scope *QueryScope,
    cursorLine int,
    cursorCol int,
    modelMap map[string]ModelDetails,
    sourceMap map[string]Source,
) []ScopeColumn
```

Note: This function lives in the `parser` package but takes `ModelDetails` and `Source` types from the `analysis` package. Since `parser` is a sub-package of `analysis`, this creates an import cycle. There are two options:

- **Option A**: Define the function to accept generic column-providing interfaces/maps instead (e.g., `map[string][]ColumnInfo` where `ColumnInfo` is defined in the parser package).
- **Option B**: Move the function to the `analysis` package (in `resolve.go` at the analysis level).

The plan specifies the function in `analysis/parser/resolve.go`, so use Option A: accept `map[string][]ScopeColumn` as the pre-resolved column lookup for models, and `map[string][]ScopeColumn` for sources. The caller (State method in section 07) converts `ModelDetailMap` into this format before calling.

Alternatively, accept a simple column map:

```go
// ModelColumns maps model name → list of column name/description pairs.
// Built by the caller from ModelDetailMap/SourceDetailMap before calling resolution.
type ModelColumns map[string][]ColumnInfo

type ColumnInfo struct {
    Name        string
    Description string
}

func ResolveColumnsAtCursor(
    scope *QueryScope,
    cursorLine int,
    cursorCol int,
    modelCols ModelColumns,
) []ScopeColumn
```

This avoids the import cycle entirely. The `analysis.State` method converts its maps before calling.

### Algorithm

**Step 1 — Identify cursor clause**: Sort `scope.ClauseRanges` by position. The cursor is in the last clause whose start token position is before `(cursorLine, cursorCol)`. This determines context but does not currently change behavior (columns from FROM/JOIN sources are available in all clauses: SELECT, WHERE, GROUP BY, etc.).

**Step 2 — Collect sources**: Iterate `scope.Sources`. For each source, resolve its columns based on `Kind`:

- **SourceKindRef**: Look up `modelCols[source.Name]`. Each entry becomes a `ScopeColumn`.
- **SourceKindCTE**: Look up `scope.CTEs[source.Name]`. Iterate `CTEDef.Columns`:
  - If the column is `"*"`, lazily expand by recursing into the CTE's `Scope` — iterate its sources and resolve their columns via `modelCols`. This handles `WITH cte AS (SELECT * FROM {{ ref('orders') }})`.
  - Otherwise, create a `ScopeColumn{Name: column}`.
- **SourceKindSource**: Return no columns (known limitation). Source struct does not have per-table column data yet.
- **SourceKindTable**: Return no columns (would need DB connection).

**Step 3 — Build ScopeColumn list**: For each resolved column from each source:
- `Name`: the column name
- `Source`: the source's alias if set, otherwise its name
- `Qualified`: `Source + "." + Name`
- `Description`: from the `ColumnInfo` if available

**Step 4 — Alias-qualified filtering**: The function itself returns all columns. Filtering by alias prefix (e.g., user typed `o.`) is handled by the caller or via an optional `aliasPrefix` parameter. If an `aliasPrefix` parameter is added, use `QueryScope.FindSourceByAlias(aliasPrefix)` to locate the source and return only its columns.

Consider adding the alias prefix as a parameter:

```go
func ResolveColumnsAtCursor(
    scope *QueryScope,
    cursorLine int,
    cursorCol int,
    modelCols ModelColumns,
    aliasPrefix string, // empty string means return all columns
) []ScopeColumn
```

When `aliasPrefix` is non-empty, find the matching source via `FindSourceByAlias` and resolve only that source's columns.

### Lazy CTE Star Expansion

The `"*"` expansion for CTEs is recursive but bounded — a CTE's scope references concrete sources (refs, other CTEs). To prevent infinite recursion from malformed input, track visited CTE names:

```go
func resolveCTEColumns(
    scope *QueryScope,
    cteName string,
    modelCols ModelColumns,
    visited map[string]bool,
) []ScopeColumn
```

If `visited[cteName]` is true, return empty (cycle detected). Otherwise mark visited, resolve the CTE's scope sources, expand any nested `"*"` entries recursively.

### Fallback Behavior

When a source has no resolvable columns (unknown table, missing schema.yml, parse errors), return zero columns for that source. The caller still gets columns from other sources in scope. This ensures partial results are always available.

### Edge Cases

- **Nil scope**: Return empty slice immediately.
- **Empty Sources**: Return empty slice.
- **CTE not found in CTEs map**: Skip that source (defensive — should not happen with correct parsing).
- **Self-join**: Same model appears twice with different aliases. Each alias produces its own set of `ScopeColumn` entries with the respective alias in `Source` and `Qualified`.
- **No alias on source**: Use the source's `Name` as the `Source` field in `ScopeColumn`.

---

## Implementation Notes

**Status:** Completed

**Files created:**
- `analysis/parser/resolve.go` — `ColumnInfo`, `ModelColumns`, `ResolveColumnsAtCursor`, `resolveSourceColumns`, `resolveCTEColumns`, `buildScopeColumns`
- `analysis/parser/resolve_test.go` — 10 tests

**Deviations from plan:**
- Dropped `cursorLine`/`cursorCol` params — clause identification deferred since columns are available in all clauses. Can be added later.
- Used `aliasPrefix` as third param instead of separate function. Empty string returns all columns.
- No `Source` type from analysis package needed — `SourceKindSource` returns empty columns (known limitation).