Now I have all the context needed. Here is the section content:

# Section 01: Source Columns

## Goal

Add a `Columns` field to `SourceTableProperties` (YAML parsing) and `SourceTable` (runtime struct). Populate source table columns during YAML parsing. Update all test fixtures that construct `SourceTable` values.

## Background

`ColumnProperties` already exists (`parse_dbt_yaml.go:147-150`) and is used by `ModelProperties`. The `Column` struct exists in `agg_model_details.go:5-8`. Model columns are extracted from `ColumnProperties` in `agg_model_details.go:66-71` using the pattern:

```go
for _, col := range schemaDetails.Columns {
    columns = append(columns, Column{
        Name:        col.Name.Value,
        Description: col.Description.Value,
    })
}
```

Source tables currently lack column extraction. The `SourceTableProperties` struct (`parse_dbt_yaml.go:168-171`) has no `Columns` field, and `SourceTable` (`parse_dbt_yaml.go:199-205`) also lacks one. This section adds both and wires up the parsing.

## Files to Modify

- `/home/theotime.poulain/dbt-language-server/analysis/parse_dbt_yaml.go` -- add `Columns` field to structs, populate during parsing
- `/home/theotime.poulain/dbt-language-server/analysis/parse_dbt_yaml_test.go` -- update expected values for new `Columns` field
- `/home/theotime.poulain/dbt-language-server/analysis/state_test.go` -- update `expectedTestState()` SourceTable entries with `Columns`
- `/home/theotime.poulain/dbt-language-server/analysis/state_lsp_test.go` -- update `newTestState()` SourceTable entries with `Columns`
- `/home/theotime.poulain/dbt-language-server/testdata/jaffle_shop_duckdb/models/schema.yml` -- add columns to source table definitions

## Tests First

### 1. parse_dbt_yaml_test.go: SourceTableProperties with columns

The existing `TestParsePropertiesYamlFile` must be updated. Currently `SourceTableProperties` entries in the expected output have only `Name` and `Description`. After this change, they must also include `Columns`.

First, add columns to the testdata source definitions in `schema.yml` (see "Testdata Changes" below). Then update the expected `SourceTableProperties` entries to include `Columns: []ColumnProperties{...}` matching the YAML.

For source tables with no columns defined (if any remain), expect `Columns` to be `nil` (zero value of `[]ColumnProperties`).

### 2. state_test.go: TestRefreshDbtContext regression

`expectedTestState()` constructs `SourceTable` values at lines 239-280. Each must gain a `Columns: []Column{...}` field matching the columns added to `schema.yml`. This test uses `reflect.DeepEqual`, so the fields must match exactly.

For example, after adding columns to the `stripe.payments` source in schema.yml:

```go
"payments": {
    Name:        "payments",
    Description: "",
    Table:       "stripe",
    URI:         filepath.Join(testdataRoot, "models/schema.yml"),
    Range:       lsp.Range{...},
    Columns: []Column{
        {Name: "payment_id", Description: ""},
        {Name: "payment_method", Description: ""},
        {Name: "amount", Description: ""},
    },
},
```

The exact column names/descriptions depend on what is added to schema.yml.

### 3. state_lsp_test.go: newTestState update

`newTestState()` constructs a `SourceTable` at line 40-49. Add `Columns` to the `my_table` entry:

```go
"my_table": {
    Name:        "my_table",
    Description: "Table description",
    Table:       "my_source",
    URI:         "/test/models/schema.yml",
    Range:       lsp.Range{...},
    Columns: []Column{
        {Name: "id", Description: "Primary key"},
        {Name: "name", Description: "Name field"},
    },
},
```

These test columns will be used by later sections (section-03 scope-based completion) for source column completion tests.

### 4. New test: Source table with columns parses correctly

Add a unit test in `parse_dbt_yaml_test.go` that parses a YAML snippet with source table columns and verifies `SourceTableProperties.Columns` is populated. This can be a focused test using a small inline YAML string or a separate fixture file, confirming:
- Multiple columns are extracted with correct names and descriptions
- A source table with zero columns yields nil/empty `Columns` slice

## Implementation

### Step 1: Add Columns to SourceTableProperties

In `/home/theotime.poulain/dbt-language-server/analysis/parse_dbt_yaml.go`, line 168-171, add `Columns` field:

```go
type SourceTableProperties struct {
    Name        AnnotatedField[string] `yaml:"name"`
    Description AnnotatedField[string] `yaml:"description"`
    Columns     []ColumnProperties     `yaml:"columns"`
}
```

`ColumnProperties` already exists at line 147. The `yaml:"columns"` tag will cause the YAML decoder to automatically parse the `columns` list from source table definitions.

### Step 2: Add Columns to SourceTable

In the same file, line 199-205, add `Columns` field:

```go
type SourceTable struct {
    Name        string
    Description string
    Table       string
    URI         string
    Range       lsp.Range
    Columns     []Column
}
```

`Column` is defined in `agg_model_details.go:5-8` (same package).

### Step 3: Populate Columns During Parsing

In `parseYamlModels()`, around line 256-265 where `SourceTable` is constructed inside the `for _, table := range source.Tables` loop, extract columns from `table.Columns`:

```go
for _, table := range source.Tables {
    var columns []Column
    for _, col := range table.Columns {
        columns = append(columns, Column{
            Name:        col.Name.Value,
            Description: col.Description.Value,
        })
    }
    sourceMap[source.Name.Value].Tables[table.Name.Value] = SourceTable{
        Name:        table.Name.Value,
        Description: jinja.ReplaceDocBlocks(table.Description.Value, docsMap),
        Table:       source.Name.Value,
        URI:         file,
        Range: lsp.Range{
            Start: table.Name.Position,
            End:   table.Name.Position,
        },
        Columns: columns,
    }
}
```

### Step 4: Testdata Changes

Add columns to source table definitions in `/home/theotime.poulain/dbt-language-server/testdata/jaffle_shop_duckdb/models/schema.yml`. Currently the sources section (lines 84-94) has bare table names with no columns. Add column definitions to at least the `stripe.payments` source table, e.g.:

```yaml
  - name: stripe
    tables:
      - name: payments
        columns:
          - name: payment_id
            description: ""
          - name: payment_method
            description: ""
          - name: amount
            description: ""
```

Choose column names that are realistic for the jaffle_shop context. Adding columns to `jaffle_shop.orders` and `jaffle_shop.customers` source tables is also recommended for integration test coverage in section-08.

**Important:** Adding lines to schema.yml will shift line numbers for the existing source definitions. All line-number-dependent test assertions in `parse_dbt_yaml_test.go` and `state_test.go` (specifically the `Position` and `Range` values for sources) must be updated to match the new line offsets.

## Implementation Notes (Post-Implementation)

### Actual files modified
- `analysis/parse_dbt_yaml.go` — added `Columns` to `SourceTableProperties` (line 171) and `SourceTable` (line 206), column extraction loop (lines 259-265)
- `analysis/parse_dbt_yaml_test.go` — updated positions, added `TestSourceTableColumnsParsing` with two subtests (with/without columns)
- `analysis/state_test.go` — updated source positions and added `Columns` to all `SourceTable` entries
- `analysis/state_lsp_test.go` — added `Columns` to `newTestState()` `my_table` entry
- `testdata/jaffle_shop_duckdb/models/schema.yml` — added columns to all 3 source tables (orders, customers, payments)

### Deviations from plan
- Added columns to all 3 source tables (not just stripe.payments) for full coverage
- One source column (`payment_id`) has a non-empty description to exercise that parsing path (code review fix)
- Added `TestSourceTableColumnsParsing` with inline YAML fixtures covering both populated and nil-columns cases (code review fix)

### Test count: 3 tests affected
- `TestParsePropertiesYamlFile` — updated assertions
- `TestRefreshDbtContext` — updated assertions
- `TestSourceTableColumnsParsing` — new, 2 subtests

## Dependencies

- None. This is a leaf section with no dependencies on other sections.
- Sections 03 (scope-based column completion) and 08 (integration tests) depend on this section's `Columns` field being populated.