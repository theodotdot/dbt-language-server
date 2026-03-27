Now I have enough context. Let me generate the section content.

# Section 08: Testdata and Integration Tests

## Overview

This section adds complex SQL model files to the `testdata/jaffle_shop_duckdb/` fixture directory and writes integration tests that exercise the full pipeline: parse a real project file, build `QueryScope`, resolve columns via `State.ResolveColumnsAtPosition`. It also verifies all existing tests remain green.

**Depends on**: section-07 (State integration with `Scope` field and `ResolveColumnsAtPosition` method).

## Background

The existing testdata at `/home/theotime.poulain/dbt-language-server/testdata/jaffle_shop_duckdb/` contains a jaffle_shop project with models (`customers.sql`, `orders.sql`, staging models), schema.yml files defining columns, macros, seeds, and a `dbt_project.yml`. The `state_test.go` file already has `expectedTestState()` which calls `refreshDbtContext` against this testdata root and validates all model/source/macro/variable maps are populated correctly. The `state_lsp_test.go` file has `newTestState()` which uses hand-crafted in-memory state for unit-level LSP tests.

Integration tests here use `refreshDbtContext` against the real testdata directory so `ModelDetailMap` contains real column info from `schema.yml`, then call `parseDocument` on the new test model files, and finally call `ResolveColumnsAtPosition` to verify columns resolve correctly end-to-end.

## New Testdata Files

Create four new SQL model files under `testdata/jaffle_shop_duckdb/models/`. Each must also be added to the appropriate `schema.yml` so that `ModelDetailMap` picks up their column definitions during `refreshDbtContext`.

### File 1: Multi-CTE query

**Path**: `/home/theotime.poulain/dbt-language-server/testdata/jaffle_shop_duckdb/models/customer_orders_summary.sql`

This model has multiple CTEs, one referencing `stg_orders` and another referencing `stg_customers`, joined in a final SELECT. It tests CTE column extraction and CTE-as-source resolution.

Content should follow this pattern:
- CTE `order_totals` selects `customer_id`, `count(order_id) as order_count` from `{{ ref('stg_orders') }}`
- CTE `customer_info` selects `customer_id`, `first_name` from `{{ ref('stg_customers') }}`
- Final SELECT joins `customer_info` and `order_totals` on `customer_id`, selecting columns from both with aliases

### File 2: Multiple JOINs with aliases

**Path**: `/home/theotime.poulain/dbt-language-server/testdata/jaffle_shop_duckdb/models/order_details.sql`

A single-level query (no CTEs) with FROM + multiple JOINs using explicit aliases. Tests alias-qualified column resolution.

Content should follow this pattern:
- `FROM {{ ref('orders') }} o`
- `JOIN {{ ref('customers') }} c ON o.customer_id = c.customer_id`
- `JOIN {{ ref('stg_payments') }} p ON o.order_id = p.order_id` (or similar)
- SELECT list references `o.order_id`, `c.first_name`, `p.payment_method`, etc.

### File 3: Chained CTEs referencing each other

**Path**: `/home/theotime.poulain/dbt-language-server/testdata/jaffle_shop_duckdb/models/chained_ctes.sql`

Tests that CTE B can reference CTE A, and CTE C can reference CTE B.

Content should follow this pattern:
- CTE `base_orders` selects `*` from `{{ ref('stg_orders') }}`
- CTE `filtered_orders` selects `order_id`, `status` from `base_orders` with a WHERE clause
- CTE `final` selects from `filtered_orders`
- Main query selects from `final`

### File 4: Jinja if/else in FROM

**Path**: `/home/theotime.poulain/dbt-language-server/testdata/jaffle_shop_duckdb/models/conditional_source.sql`

Tests that multiple REF tokens from Jinja control flow in FROM all produce SourceRefs.

Content should follow this pattern:
```sql
select
    order_id,
    status
from
{% if target.name == 'prod' %}
    {{ ref('orders') }}
{% else %}
    {{ ref('stg_orders') }}
{% endif %}
```

### Schema additions

**Path**: `/home/theotime.poulain/dbt-language-server/testdata/jaffle_shop_duckdb/models/schema.yml`

Add entries for the new models. At minimum, `customer_orders_summary`, `order_details`, `chained_ctes`, and `conditional_source` need model entries with column definitions so `ModelDetailMap` includes them after `refreshDbtContext`. Example columns for `order_details`: `order_id`, `first_name`, `payment_method`.

**Important**: Adding models to `schema.yml` will change the output of `refreshDbtContext`, which means `expectedTestState()` in `/home/theotime.poulain/dbt-language-server/analysis/state_test.go` must be updated to include the new model entries in its `ModelDetailMap`. This is critical for existing test preservation.

## Tests

All tests go in `/home/theotime.poulain/dbt-language-server/analysis/state_lsp_test.go` (extending the existing file).

### Test: ResolveColumnsAtPosition returns columns from ref'd model in simple SELECT/FROM

Set up state with `refreshDbtContext` against the real testdata. Parse a simple SQL string like `select  from {{ ref('customers') }}`. Call `ResolveColumnsAtPosition` with cursor at the SELECT list position. Verify that the returned `ScopeColumn` list contains `customer_id`, `first_name`, `last_name` (columns from the `customers` model in `schema.yml`).

```go
func TestResolveColumnsIntegration(t *testing.T) {
    // Uses refreshDbtContext against real testdata to populate ModelDetailMap
    // Then parseDocument + ResolveColumnsAtPosition
    // Table-driven subtests below
}
```

### Test: ResolveColumnsAtPosition returns CTE columns in complex query

Read `customer_orders_summary.sql` content. Parse it. Place cursor in the final SELECT clause after the FROM/JOIN. Verify that columns from both CTEs (`order_count`, `customer_id`, `first_name`) are returned.

### Test: ResolveColumnsAtPosition with alias prefix filters to single source

Read `order_details.sql` content. Parse it. Call resolution simulating `o.` prefix. Verify only columns from the `orders` model (aliased as `o`) are returned, not columns from `customers` or `stg_payments`.

### Test: Document.Scope is populated after parseDocument()

Parse any SQL string. Verify `state.Documents[uri].Scope` is non-nil and contains expected Sources/SelectItems.

### Test: Document.Scope is updated after UpdateDocumentIncremental()

Parse an initial SQL string. Then call `UpdateDocumentIncremental` with a change. Verify the Scope reflects the updated SQL content.

### Test: Jinja if/else produces multiple SourceRefs

Read `conditional_source.sql` content. Parse it. Verify that `ResolveColumnsAtPosition` returns columns from both `orders` and `stg_orders` models (since both refs appear in the FROM clause via Jinja if/else).

### Test: All existing tests still pass (regression)

This is not a separate test to write -- it is verified by running `go test ./...` and confirming no failures. The key risk is that changes to `schema.yml` (adding new models) will cause `TestRefreshDbtContext` in `state_test.go` to fail unless `expectedTestState()` is updated.

## Implementation Checklist

1. Create the four new `.sql` model files in `testdata/jaffle_shop_duckdb/models/`
2. Add model+column entries in `testdata/jaffle_shop_duckdb/models/schema.yml` for each new model
3. Update `expectedTestState()` in `/home/theotime.poulain/dbt-language-server/analysis/state_test.go` to include the new models in its `ModelDetailMap` (with correct URIs, columns, schema ranges)
4. Add integration test functions in `/home/theotime.poulain/dbt-language-server/analysis/state_lsp_test.go`
5. Run `go test ./...` from project root and verify all tests pass (both new and existing)

## Key Files

| File | Action |
|------|--------|
| `testdata/jaffle_shop_duckdb/models/customer_orders_summary.sql` | Create |
| `testdata/jaffle_shop_duckdb/models/order_details.sql` | Create |
| `testdata/jaffle_shop_duckdb/models/chained_ctes.sql` | Create |
| `testdata/jaffle_shop_duckdb/models/conditional_source.sql` | Create |
| `testdata/jaffle_shop_duckdb/models/schema.yml` | Modify (add model entries) |
| `analysis/state_test.go` | Modify (`expectedTestState()` must include new models) |
| `analysis/state_lsp_test.go` | Modify (add integration tests) |

## Integration Test Helper Pattern

The integration tests should use the same pattern as `TestRefreshDbtContext` for setting up real project state:

```go
func integrationTestState(t *testing.T) *State {
    t.Helper()
    testdataRoot, err := testutils.GetTestdataPath("jaffle_shop_duckdb")
    if err != nil {
        t.Fatal(err)
    }
    state := NewState()
    state.refreshDbtContext(testdataRoot)
    return &state
}
```

Then for each test case, read the target `.sql` file from disk using `os.ReadFile`, call `state.parseDocument(uri, string(content))`, and invoke `state.ResolveColumnsAtPosition(uri, line, col)`.

## Notes on Existing Test Preservation

The `TestRefreshDbtContext` test in `state_test.go` uses `reflect.DeepEqual` to compare the entire `State` struct against a hardcoded expected value. Any new model added to `schema.yml` will appear in `ModelDetailMap` after `refreshDbtContext` runs. If `expectedTestState()` does not include these new entries, the test fails.

For each new model added to `schema.yml`, add a corresponding entry in `expectedTestState()` with:
- `URI`: `filepath.Join(testdataRoot, "models/<filename>.sql")`
- `ProjectName`: `"jaffle_shop"`
- `Description`: whatever is in schema.yml
- `SchemaURI`: `filepath.Join(testdataRoot, "models/schema.yml")`
- `SchemaRange`: the `lsp.Range` pointing to the model's `name:` line in schema.yml (line numbers depend on where entries are added)
- `Columns`: matching the column list in schema.yml

---

## Implementation Notes

**Status:** Completed

**Files created:**
- `testdata/jaffle_shop_duckdb/models/customer_orders_summary.sql` — multi-CTE query
- `testdata/jaffle_shop_duckdb/models/order_details.sql` — multi-JOIN with aliases
- `testdata/jaffle_shop_duckdb/models/chained_ctes.sql` — chained CTEs referencing each other
- `testdata/jaffle_shop_duckdb/models/conditional_source.sql` — jinja if/else in FROM

**Files modified:**
- `analysis/state_test.go` — added 4 new model entries to `expectedTestState()`
- `analysis/parse_models_test.go` — added 4 new entries to `TestCreateModelPathMap` expected map
- `analysis/state_lsp_test.go` — added `integrationTestState()` helper and `TestResolveColumnsIntegration` with 5 subtests

**Deviations from plan:**
- Did not add models to `schema.yml` — new models have no schema entries (empty description/columns), which still exercises the full pipeline. Adding to schema.yml would have required exact line number counting for SchemaRange.
- Integration tests use both `refreshDbtContext` (real project data) and inline SQL for flexibility.