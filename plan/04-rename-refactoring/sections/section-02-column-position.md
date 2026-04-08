Now I have everything needed to write the section.

# Section 2: Column Position Propagation

## Overview

The `Column` struct (in `analysis/agg_model_details.go`) currently stores only `Name` and `Description`. To support rename of column names in `schema.yml`, the struct needs a `Position lsp.Position` field that records where the column name appears in the YAML file. This position is already captured during YAML parsing via `AnnotatedField[string]` on `ColumnProperties.Name` but is discarded during model aggregation.

This section adds the `Position` field to `Column` and propagates it from `ColumnProperties.Name.Position` during aggregation.

**No dependencies on other sections.** Sections 04 (PrepareRename) and 06 (Rename) depend on this section's output.

## Files to Modify

- `/home/theotime.poulain/dbt-language-server/analysis/agg_model_details.go` -- add `Position` field to `Column` struct and propagate it during aggregation

## Background: How Positions Are Already Captured

The YAML parser uses `AnnotatedField[T]` (defined in `analysis/parse_dbt_yaml.go`) which captures `Value T` and `Position lsp.Position` during `UnmarshalYAML`. The `ColumnProperties` struct uses `AnnotatedField[string]` for its `Name` field, so `col.Name.Position` already contains the 0-indexed line and character where the column name appears in the YAML source.

Currently, the aggregation loop in `getModelDetails()` (lines 66-70 of `agg_model_details.go`) builds `Column` structs but only copies `Name` and `Description`:

```go
for _, col := range schemaDetails.Columns {
    columns = append(columns, Column{
        Name:        col.Name.Value,
        Description: col.Description.Value,
    })
}
```

## Tests

Write tests in `/home/theotime.poulain/dbt-language-server/analysis/agg_model_details_test.go` (or add to an existing test file if one already covers `getModelDetails`).

### Test: Column struct populated with correct Position from AnnotatedField during model aggregation

Set up a `State` with a `DbtContext` pointing at the existing test fixture directory (`testdata/` used by `parse_dbt_yaml_test.go`). Call `getModelDetails()` and verify that the resulting `Column` entries have non-zero `Position` fields matching the YAML source positions.

Specifically, check against the known positions from the existing parse test expectations (from `parse_dbt_yaml_test.go`):
- Column `customer_id` at position `{Line: 7, Character: 14}`
- Column `first_name` at position `{Line: 13, Character: 14}`

The test should assert that `modelDetails.Columns[i].Position` equals the expected `lsp.Position`.

### Test: Position line/column match the YAML source positions (0-indexed)

This is implicitly covered by the above test. Verify that positions are 0-indexed (as `yaml.Node` provides them), matching what `AnnotatedField.UnmarshalYAML` stores. The existing `parse_dbt_yaml_test.go` already validates that `ColumnProperties.Name.Position` has correct 0-indexed values; this test confirms those values survive aggregation into `Column.Position`.

## Implementation

### Step 1: Add Position field to Column struct

In `/home/theotime.poulain/dbt-language-server/analysis/agg_model_details.go`, modify the `Column` struct:

```go
type Column struct {
	Name        string
	Description string
	Position    lsp.Position
}
```

### Step 2: Propagate position during aggregation

In the same file, in the `getModelDetails()` method, update the column-building loop (around line 66) to include the position:

```go
for _, col := range schemaDetails.Columns {
    columns = append(columns, Column{
        Name:        col.Name.Value,
        Description: col.Description.Value,
        Position:    col.Name.Position,
    })
}
```

This is a one-line addition. The `col.Name` is an `AnnotatedField[string]` whose `Position` field is already populated by the YAML unmarshaler.

### Step 3: Verify no downstream breakage

The `Column` struct is used in:
- `getColumnCompletionItems()` in completion logic -- only accesses `Name` and `Description`, so adding a new field is backward-compatible
- `ModelDetails.Columns` -- consumers that iterate columns won't break since the new field has a zero value by default

No other files need modification. The new field is purely additive.

## Implementation Notes (Post-Implementation)

Deviations from plan:
- Also propagated Position for source table columns in `parse_dbt_yaml.go` (plan only mentioned model columns in `agg_model_details.go`)
- Updated `expectedTestState()` in `state_test.go` with actual YAML positions for all Column entries (models + sources)

### Files modified/created
- `analysis/agg_model_details.go` — MODIFIED (added Position field, propagation)
- `analysis/parse_dbt_yaml.go` — MODIFIED (source table column Position propagation)
- `analysis/agg_model_details_test.go` — NEW (1 test)
- `analysis/state_test.go` — MODIFIED (position values in expectedTestState)

### Test count: 1 new test + all existing tests pass
- `TestColumnPositionPropagation`