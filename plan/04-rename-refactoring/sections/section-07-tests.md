I have enough context. Let me generate the section content.

# Section 7: Tests

## Overview

This section creates `analysis/rename_test.go` with comprehensive unit tests for all rename/prepareRename functionality implemented in sections 1-6. It also defines test fixture files under `testdata/` for on-disk reference scanning tests.

**Dependencies:** All prior sections (01-06) must be complete. The tests exercise `PrepareRename()`, `Rename()`, and internal helpers like reference scanning and name validation.

**Files to create:**
- `/home/theotime.poulain/dbt-language-server/analysis/rename_test.go`
- `/home/theotime.poulain/dbt-language-server/testdata/rename_fixtures/models/target_model.sql`
- `/home/theotime.poulain/dbt-language-server/testdata/rename_fixtures/models/multi_ref.sql`
- `/home/theotime.poulain/dbt-language-server/testdata/rename_fixtures/models/no_ref.sql`
- `/home/theotime.poulain/dbt-language-server/testdata/rename_fixtures/models/cross_project_ref.sql`
- `/home/theotime.poulain/dbt-language-server/testdata/rename_fixtures/dbt_project.yml`

## Background: Existing Test Patterns

The project uses Go's built-in `testing` package. The established pattern from `/home/theotime.poulain/dbt-language-server/analysis/state_lsp_test.go` is:

- `newTestState()` builds a `State` with fixture data (ModelDetailMap, SourceDetailMap, etc.) using synthetic paths like `/test/models/customers.sql`
- Documents are parsed in-memory via `state.parseDocument(uri, sqlText)`
- Table-driven tests with `t.Run()` subtests
- Assertions compare structs or specific fields directly

The `testutils.GetTestdataPath()` function resolves absolute paths to the `testdata/` directory relative to the calling test file.

## Test Fixtures

### On-Disk SQL Files for Reference Scanning

Create these under `testdata/rename_fixtures/models/`:

**`target_model.sql`** -- a model that references the rename target once:
```sql
select * from {{ ref('orders') }}
```

**`multi_ref.sql`** -- multiple ref calls, some matching, some not:
```sql
select * from {{ ref('orders') }}
join {{ ref('customers') }} on true
union all
select * from {{ ref('orders') }}
```

**`no_ref.sql`** -- no ref calls at all:
```sql
select 1 as id
```

**`cross_project_ref.sql`** -- cross-project ref that must be excluded:
```sql
select * from {{ ref('other_pkg', 'orders') }}
```

**`dbt_project.yml`** -- minimal project config:
```yaml
name: rename_test
model-paths: ["models"]
```

## Test File: `analysis/rename_test.go`

All tests go in a single file. The tests are organized into logical groups via subtests.

### Test Helper: `newRenameTestState()`

Extend the `newTestState()` pattern with rename-specific data. This helper should set up:

- `DbtContext.ProjectRoot` pointing to `testdata/rename_fixtures`
- `DbtContext.ProjectYaml.ProjectName.Value` = `"test_project"` and `ModelPaths.Value` = `["models"]`
- `ModelDetailMap` entries for:
  - `"orders"`: with `URI`, `ProjectName: "test_project"`, `SchemaURI` pointing to a schema file, `SchemaRange` with `Start: {Line: 5, Character: 10}`, and columns including `{Name: "order_id", Position: {Line: 7, Character: 8}}` and `{Name: "status", Position: {Line: 8, Character: 8}}`
  - `"customers"`: with `URI`, `ProjectName: "test_project"`, `SchemaURI`, `SchemaRange`, and columns including `{Name: "customer_id", Position: {Line: 12, Character: 8}}`
  - `"stg_external"`: with `ProjectName: "other_pkg"` (cross-project model, used to test filtering)
- At least one document parsed via `state.parseDocument()` containing `ref('orders')` for in-memory token testing

### Test Group 1: PrepareRename

Tests call `state.PrepareRename(id, uri, position)` and verify the returned `PrepareRenameResponse`.

**Test cases:**

1. **Cursor on REF token in SQL** -- parse `select * from {{ ref('orders') }}`, position cursor on `orders` (within the REF token range). Expect response with range covering `orders` and placeholder `"orders"`.

2. **Cursor on model name in schema.yml** -- set up a ModelDetails entry where `SchemaURI` matches the test URI and `SchemaRange.Start` is at `{Line: 5, Character: 10}`. Position cursor at `{Line: 5, Character: 12}` (within `Start.Character` to `Start.Character + len("orders")`). Expect range and placeholder `"orders"`.

3. **Cursor on column name in schema.yml** -- similar to model name but cursor on a column's Position. Expect column name as placeholder.

4. **Cursor on cross-project ref** -- parse `select * from {{ ref('other_pkg', 'orders') }}`, cursor on the REF token. Expect null/empty response (PACKAGE token precedes REF).

5. **Cursor on non-renameable position** -- cursor on SQL keyword (`select`), whitespace, or `source()` call. Expect null response.

6. **Cursor on REF for non-existent model** -- parse `select * from {{ ref('nonexistent') }}`, model not in ModelDetailMap. Expect null response.

7. **Position end computation** -- verify that the returned range's End equals `{Line: Start.Line, Character: Start.Character + len(modelName)}`.

### Test Group 2: Name Validation

Test the identifier validation regex `^[a-zA-Z_][a-zA-Z0-9_]*$`.

**Test cases (table-driven):**

| Input | Valid |
|-------|-------|
| `"valid_name"` | true |
| `"_leading_underscore"` | true |
| `"Name123"` | true |
| `""` (empty) | false |
| `"123start"` | false |
| `"has space"` | false |
| `"special!char"` | false |
| `"has-dash"` | false |

These can test a standalone validation helper or be tested via `Rename()` returning an error response.

### Test Group 3: Rename Conflict Detection

Tests call `state.Rename(id, uri, position, newName)` and verify error responses.

1. **Rename model to existing name** -- rename `orders` to `customers` (which exists in ModelDetailMap). Expect `ResponseError` with code `-32602` and message containing `"customers"` and `"already exists"`.

2. **Rename column to existing name in same model** -- rename column `order_id` to `status` in the `orders` model. Expect `ResponseError`.

3. **Rename with invalid identifier** -- use `newName = "123bad"`. Expect `ResponseError`.

4. **Rename with empty newName** -- use `newName = ""`. Expect `ResponseError`.

### Test Group 4: Model Rename WorkspaceEdit Construction

Tests call `state.Rename()` for a model and verify the `WorkspaceEdit.DocumentChanges` array.

1. **RenameFile entry present** -- verify `documentChanges` includes a `RenameFile` with `OldURI = "file://" + oldPath` and `NewURI = "file://" + newDir + "/new_orders" + ext`. The new path preserves directory and changes only the filename.

2. **Schema TextDocumentEdit present** -- verify a `TextDocumentEdit` targeting `"file://" + schemaURI` with a `TextEdit` whose range covers the model name at `SchemaRange.Start` and whose `NewText` is the new name.

3. **No schema edit when SchemaURI is empty** -- set up a model with `SchemaURI: ""`. Verify no schema `TextDocumentEdit` in `documentChanges`.

4. **All URIs have file:// prefix** -- iterate all entries in `documentChanges`, verify every URI string starts with `"file://"`.

5. **newName equals oldName** -- rename `orders` to `orders`. Expect empty `WorkspaceEdit` (no-op, no `documentChanges`).

### Test Group 5: Column Rename WorkspaceEdit

1. **Single TextDocumentEdit** -- rename column `order_id` to `id`. Verify `documentChanges` has exactly one `TextDocumentEdit` for the schema.yml file, with a `TextEdit` at the column's Position range, and `NewText = "id"`.

2. **No RenameFile** -- verify no `RenameFile` entries exist in `documentChanges`.

3. **Edit range matches column position + len** -- verify the edit range End character equals `Column.Position.Character + len("order_id")`.

### Test Group 6: Reference Scanning

These tests require on-disk fixture files. Use `testutils.GetTestdataPath("rename_fixtures")` to resolve the fixtures directory.

Set up State with:
- `ProjectRoot` pointing to `rename_fixtures`
- `ProjectYaml.ModelPaths.Value = ["models"]`
- `ModelDetailMap["orders"]` with `ProjectName: "test_project"`

1. **Single ref found** -- scan for `orders` references. `target_model.sql` should produce one `TextEdit`.

2. **Multiple refs across files** -- `multi_ref.sql` has two `ref('orders')` calls. Verify both appear as edits, grouped under the same URI.

3. **No ref files skipped** -- `no_ref.sql` should produce no edits.

4. **Cross-project ref excluded** -- `cross_project_ref.sql` has `ref('other_pkg', 'orders')`. Verify zero edits from this file (PACKAGE token lookback filters it out).

5. **In-memory text for open documents** -- add `target_model.sql`'s URI to `State.Documents` with modified text (e.g., `select * from {{ ref('orders') }} limit 1`). Verify the scan uses the in-memory version (edit positions should match the in-memory content, not disk).

6. **Reads from disk for closed files** -- do NOT add `multi_ref.sql` to `State.Documents`. Verify edits are still found (read from disk).

### Test Group 7: Error Serialization

Test JSON marshaling of response types.

1. **ResponseError serializes correctly** -- marshal a response with `Error: &ResponseError{Code: -32602, Message: "conflict"}`. Verify JSON contains `"error":{"code":-32602,"message":"conflict"}`.

2. **Successful rename serializes correctly** -- marshal a rename response with a `WorkspaceEdit`. Verify JSON contains `"result":{"documentChanges":[...]}`.

### Test Group 8: Column Position Propagation

This tests section-02's work. Use the existing `expectedTestState()` pattern from `state_test.go`.

1. **Column has Position field populated** -- after `refreshDbtContext`, check that columns in `ModelDetailMap` have non-zero `Position` values matching their YAML source positions.

2. **Position line/character match YAML** -- verify specific column positions against the known line numbers in `testdata/jaffle_shop_duckdb/models/schema.yml`.

Note: this test requires updating `expectedTestState()` in `state_test.go` to include `Position` fields on `Column` entries. That update is part of section-02 but verifying it belongs here.

## Implementation Notes

- Import `encoding/json` for serialization tests
- Import `github.com/j-clemons/dbt-language-server/testutils` for fixture path resolution
- Import `github.com/j-clemons/dbt-language-server/analysis/parser` for token types
- Import `github.com/j-clemons/dbt-language-server/lsp` for LSP types
- The test file is in `package analysis` (same package testing, access to unexported functions)
- For reference scanning tests, the `Rename()` method internally calls the scanning logic -- test through the public API rather than testing internal scan functions directly unless they are exported
- All assertions should use direct comparison or `reflect.DeepEqual` following existing patterns; no external assertion libraries are used in this project

## Implementation Notes (Post-Implementation)

### Files modified
- `analysis/rename_test.go` — added table-driven validation test (9 cases), 3 integration tests, consolidated path helpers to use `testutils.GetTestdataPath`

### Files created
- `testdata/rename_fixtures/dbt_project.yml`, `testdata/rename_fixtures/models/{target_model,multi_ref,no_ref,cross_project_ref}.sql` — test fixtures

### Deviations from plan
- **Most tests already existed** from TDD in sections 04-06. This section added supplementary tests rather than creating from scratch.
- **Column position propagation** not tested via `refreshDbtContext` (already covered in `state_test.go`); tested via `renameColumn` position usage instead.
- **Used `testutils.GetTestdataPath`** per code review, replacing custom `os.Getwd()` helpers.

### Test count: 40 total passing rename test cases (7 PrepareRename + 8 findModelRefs + 12 Rename + 9 validation + 3 integration + 1 initialize)