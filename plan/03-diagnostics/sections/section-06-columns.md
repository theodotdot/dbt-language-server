# Section 6: Missing Column Detection

## Overview

This section creates `analysis/diagnostics/columns.go` — a diagnostic checker that detects references to columns that do not exist in the schema of referenced models. This checker is **blocked on the column lineage engine from split 01 (01-sql-column-lineage)**, which does not yet exist. The deliverable for now is a stub file with the checker registration pattern, ready to be filled in once the lineage engine lands.

## Dependencies

- **section-02-engine** must be complete (provides `Engine`, `Checker` type, checker registration)
- **External dependency**: column lineage engine from split 01 (`01-sql-column-lineage`) — does not exist yet

## Background Context

### Checker Type (from section-02-engine)

The diagnostic engine defines a `Checker` function type:

```go
type Checker func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic
```

Checkers receive immutable snapshots of document and dbt context, never live `State`. They return a slice of `lsp.Diagnostic`.

### Relevant Data Structures

The `ModelDetails` struct (in `/home/theotime.poulain/dbt-language-server/analysis/agg_model_details.go`) has a `Columns []Column` field, where `Column` has `Name` and `Description` string fields. The column checker will compare resolved column references against this list.

The `Source` struct (in `/home/theotime.poulain/dbt-language-server/analysis/parse_dbt_yaml.go`) holds source metadata but does not currently carry column-level information for source tables.

### Design Intent

The column checker will:

1. Use the lineage engine to resolve which model/source each column reference in a SQL file belongs to
2. Look up whether that column exists in the model's `Columns` list from `ModelDetails`
3. Only check columns that can be resolved to a specific model/source — unresolvable columns (e.g., from CTEs not backed by a ref/source) are silently skipped to avoid false positives
4. Emit `Warning` severity (not Error), since `schema.yml` may not list all columns
5. Message format: `Column "order_date" not found in model "stg_orders"`

### Registration Pattern

The column checker is registered with the Engine **only when the lineage engine is available**. A nil-check or feature flag determines this. When instantiating the Engine in `main.go`, the column checker is conditionally appended to the checkers slice.

## Tests

File: `/home/theotime.poulain/dbt-language-server/analysis/diagnostics/columns_test.go`

These tests are stubs. They cannot pass until the lineage engine exists. They document the expected behavior for when the implementation is unblocked.

```go
// Test: column exists in referenced model → no diagnostic
// Test: column missing from referenced model → Warning diagnostic
// Test: column in unresolvable CTE → skipped, no diagnostic
// Test: severity is Warning
// Test: message format: 'Column "order_date" not found in model "stg_orders"'
// Test: checker not registered when lineage engine is nil
```

The test file should contain a package declaration, imports, and skeleton test functions with `t.Skip("blocked on column lineage engine")` at the top of each test body. Use table-driven test structure where applicable.

Key test scenarios:

- **Column exists**: Build a `Document` with tokens referencing a column `id`, and a `DbtContext` with `ModelDetailMap` containing that model with `Columns: []Column{{Name: "id"}}`. The checker should return an empty slice.
- **Column missing**: Same setup but the column name in the token does not appear in `Columns`. Expect one `lsp.Diagnostic` with `Severity: 2` (Warning) and message matching `Column "bad_col" not found in model "my_model"`.
- **Unresolvable CTE**: Column reference that the lineage engine cannot trace to a model/source. Checker returns no diagnostic.
- **Registration guard**: When lineage engine is nil, the column checker function should not be included in the Engine's checkers slice. This is a `main.go`-level integration concern but should be tested at the registration callsite.

## Implementation

File: `/home/theotime.poulain/dbt-language-server/analysis/diagnostics/columns.go`

Create a stub file with:

1. Package declaration `package diagnostics`
2. A `CheckColumns` function matching the `Checker` signature that currently returns `nil`
3. A comment documenting the dependency on the lineage engine and what the function will do once unblocked

The function signature:

```go
// CheckColumns detects references to columns not found in the schema of
// referenced models. Blocked on the column lineage engine from split 01.
func CheckColumns(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic
```

The function body returns `nil` for now. When the lineage engine lands, the implementation will:

1. Use the lineage engine to build a mapping of column references to their originating model/source
2. For each resolved column reference, check if the column name exists in `ctx.ModelDetailMap[modelName].Columns`
3. If not found, append a `lsp.Diagnostic` with:
   - `Range`: position of the column token in the document
   - `Severity`: `2` (Warning)
   - `Source`: `"dbt-ls"`
   - `Message`: `Column "<col_name>" not found in model "<model_name>"`

### Conditional Registration (in main.go, covered by section-05-lifecycle)

When the lineage engine is available, the checker is added to the Engine:

```go
checkers := []diagnostics.Checker{
    diagnostics.CheckRefs,
    diagnostics.CheckJinja,
}
if lineageEngine != nil {
    checkers = append(checkers, diagnostics.CheckColumns)
}
```

Until the lineage engine exists, `CheckColumns` is never registered, so the stub is safe to ship.

## Files

| File | Action |
|------|--------|
| `/home/theotime.poulain/dbt-language-server/analysis/diagnostics/columns.go` | Create — stub checker |
| `/home/theotime.poulain/dbt-language-server/analysis/diagnostics/columns_test.go` | Create — skeleton tests with `t.Skip` |

## Implementation Notes (Post-Implementation)

### Files created
- `analysis/diagnostics/columns.go` — stub CheckColumns returning nil
- `analysis/diagnostics/columns_test.go` — 3 skipped + 1 passing test

### Test count: 1 passing, 3 skipped (blocked on lineage engine)