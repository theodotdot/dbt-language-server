# Section 08: Integration Tests

## Overview

This section adds end-to-end integration tests that exercise the full pipeline: YAML parsing, document parsing, scope resolution, and completion item generation. These tests use the real `testdata/jaffle_shop_duckdb` fixture rather than synthetic test state, verifying that all pieces from sections 01-07 work together correctly.

## Dependencies

- **Section 01** (source columns): `SourceTable.Columns` populated from schema.yml
- **Section 03** (scope completion): `getScopeColumnCompletionItems()`, scope-based resolution in `TextDocumentCompletion`
- **Section 04** (dot-qualified): dot-trigger detection, alias filtering, `FilterText`/`TextEdit` on items
- **Section 05** (macro args): inside-macro-call detection, `getMacroArgCompletionItems()`
- **Section 06** (sort text): standardized `SortText` prefixes across all providers
- **Section 07** (code action): `TextDocumentCodeAction` handler, SELECT * expansion

All of these must be implemented before these integration tests can pass.

## File to Modify

`/home/theotime.poulain/dbt-language-server/analysis/state_lsp_test.go`

## Tests

Add a new test function `TestIntegrationCompletion` (or extend `TestTextDocumentCompletion`) that uses the real jaffle_shop_duckdb testdata project loaded via `refreshDbtContext`. The test creates a state from the real project, parses synthetic SQL documents that reference real models/sources, and verifies completion results.

### Test Setup

The integration test setup loads the real project context rather than using `newTestState()`:

```go
func newIntegrationState(t *testing.T) *State {
    t.Helper()
    testdataRoot, err := testutils.GetTestdataPath("jaffle_shop_duckdb")
    if err != nil {
        t.Fatal(err)
    }
    s := NewState()
    s.refreshDbtContext(testdataRoot)
    return &s
}
```

This gives a state with:
- `ModelDetailMap` containing `customers` (7 columns), `orders` (9 columns), `stg_customers` (1 column), `stg_orders` (2 columns), `stg_payments` (2 columns), plus seeds
- `SourceDetailMap` containing `jaffle_shop` (tables: `orders`, `customers`) and `stripe` (table: `payments`)
- `MacroDetailMap` containing `full_name`, `times_five` (jaffle_shop), `add_values` (jaffle_package)
- `VariableDetailMap` containing `global_count`, `jaffle_number`, `jaffle_string`

After section 01, `SourceTable` entries should also have `Columns` populated from schema.yml. The current testdata schema.yml sources (`jaffle_shop` and `stripe`) do not define columns on their tables. To test source column completion end-to-end, either:
1. Add columns to the existing source definitions in `testdata/jaffle_shop_duckdb/models/schema.yml`, or
2. Create a separate testdata schema.yml with source columns

Option 1 is preferred for true integration testing. Add columns to the `stripe.payments` source table in the testdata schema.yml:

```yaml
sources:
  - name: stripe
    tables:
      - name: payments
        columns:
          - name: payment_id
            description: Unique payment identifier
          - name: amount
            description: Payment amount
```

This change also requires updating `expectedTestState()` in `/home/theotime.poulain/dbt-language-server/analysis/state_test.go` to include the new `Columns` field on the `stripe.payments` `SourceTable`.

### Test Cases

The following test cases should be implemented as table-driven tests within `TestIntegrationCompletion`:

#### 1. Full pipeline: source column completion

SQL input containing `{{ source('stripe', 'payments') }}` with cursor in SELECT position. After parsing and scope resolution, completion items should include the source columns (`payment_id`, `amount`) defined in the updated schema.yml. Verify items have `Kind == Field` and appropriate `Detail` mentioning the source.

#### 2. Full pipeline: CTE column completion

SQL input with a CTE that references a real model via `ref()`:

```sql
with cte as (
    select customer_id, first_name from {{ ref('customers') }}
)
select  from cte
```

Cursor at the space before `from cte` on the last line. Completion should include `customer_id` and `first_name` (the CTE's output columns), not the full set of `customers` model columns. This validates that the scope engine correctly resolves CTE output columns.

#### 3. Full pipeline: JOIN with aliases and dot-qualified

SQL input with JOINs and aliases:

```sql
select o. from {{ ref('stg_orders') }} o
join {{ ref('stg_payments') }} p on o.order_id = p.payment_id
```

Cursor after `o.` on line 0. Completion should return only columns from `stg_orders` (which has `order_id`, `status` in the testdata schema). Each item should have `FilterText` set (e.g., `o.order_id`) and a `TextEdit` present.

#### 4. Regression: all existing completion tests still pass

Do not remove or modify any existing test cases in `TestTextDocumentCompletion`. The integration tests are additive. Verify that:
- `ref('` trigger still returns model names
- `source('` trigger still returns source entries
- `var('` trigger still returns variables
- `{{ ` trigger still returns macros
- Column completion from ref'd models still works (backward compat path when scope is nil)

This is implicitly verified by not breaking existing tests, but worth calling out as a conscious goal.

### Validation Helpers

For readability, consider adding small helper functions local to the test:

```go
func hasCompletionWithLabel(items []lsp.CompletionItem, label string) bool {
    // Return true if any item has the given label
}

func completionLabels(items []lsp.CompletionItem) []string {
    // Extract all labels for error messages
}
```

These keep individual test case `checkItem` functions concise.

### Test for Source Columns from schema.yml

After section 01 populates `SourceTable.Columns`, the `TestRefreshDbtContext` test in `/home/theotime.poulain/dbt-language-server/analysis/state_test.go` must also pass. The `expectedTestState()` function must be updated to include the `Columns` field on the relevant `SourceTable` entries. This is already covered by section 01, but the integration test validates it end-to-end.

### Test for Sort Text Ordering

Within the integration tests, verify that columns returned in the default completion context have `SortText` starting with `"0"`, while SQL functions in the same response have `SortText` starting with `"2"`. This validates section 06's sort text standardization in a real-world scenario.

```go
// In the checkItem function for a completion test:
// Verify columns sort before functions
for _, item := range items {
    if item.Kind == completionKind.Field {
        // SortText should start with "0"
    }
    if item.Kind == completionKind.Function {
        // SortText should start with "2"
    }
}
```

### Test for Code Action (SELECT * Expansion)

Add a test that exercises `TextDocumentCodeAction` with the integration state:

```sql
select * from {{ ref('customers') }}
```

Cursor on the `*` character. The handler should return a `CodeAction` with title `"Expand SELECT *"` and a `TextEdit` that replaces `*` with the full column list from the `customers` model (7 columns: `customer_id`, `first_name`, `last_name`, `first_order`, `most_recent_order`, `number_of_orders`, `total_order_amount`).

## Testdata Modifications

### File: `/home/theotime.poulain/dbt-language-server/testdata/jaffle_shop_duckdb/models/schema.yml`

Add columns to at least one source table definition (e.g., `stripe.payments`) so that source column completion can be tested end-to-end. The exact columns to add should match what the integration test expects.

### File: `/home/theotime.poulain/dbt-language-server/analysis/state_test.go`

Update `expectedTestState()` to include the `Columns` field on `SourceTable` entries that now have columns defined in schema.yml. This keeps the `TestRefreshDbtContext` regression test passing after the testdata change.

## Implementation Notes

- The integration tests parse synthetic SQL documents (via `state.parseDocument(uri, sql)`) against the real project context. They do not read the actual model SQL files from disk.
- The `parseDocument` call triggers tokenization and scope building. The scope is what enables CTE/JOIN/dot-qualified column resolution.
- If scope resolution is not yet available (split 01 not merged), tests that depend on `doc.Scope` should be skipped or gated with a nil check. However, since section 08 depends on sections 01-07, this should not happen in practice.
- All tests use the standard Go `testing` package with table-driven test patterns, consistent with the existing test style in `state_lsp_test.go`.

## Implementation Notes (Post-Implementation)

### Deviations from plan
- Testdata schema.yml and state_test.go changes were already done in section-01 (source columns). No modification needed here.
- JOIN test uses `orders`/`customers` models (9 and 7 columns) instead of plan's `stg_orders`/`stg_payments` (no columns in schema). Both test the same behavior correctly.
- Dialect explicitly set to `"snowflake"` in sort text test because `integrationTestState()` may not resolve a dialect from profiles.yml.
- Added helper functions `hasLabel()` and `labels()` for test readability.
- Code review fix: added nil guard on `action.Edit`, edit range assertion, and `payment_method` source column assertion.

### Files modified/created
- `analysis/state_lsp_test.go` — MODIFIED (added `TestIntegrationCompletion` with 5 subtests + 2 helpers)

### Test count: 5 subtests (source columns, CTE columns, JOIN dot-qualified, sort text ordering, code action expansion)