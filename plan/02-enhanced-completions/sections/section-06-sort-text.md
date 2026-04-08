Now I have all the context needed. Let me generate the section content.

# Section 06: Sort Text Ordering

## Overview

Standardize `SortText` prefixes across all completion providers to establish a consistent priority ordering. Currently, all providers use the raw label as `SortText`, meaning items sort alphabetically with no priority distinction. This section adds prefix-based priority tiers so columns (most contextually relevant) appear before ref/source/variable completions, which appear before SQL functions.

This section has **no dependencies** on other sections and can be implemented in parallel with any other section.

## Priority Scheme

| Priority | Prefix | Used For |
|----------|--------|----------|
| Highest | `"00"` | Macro args (when inside call) -- implemented in section-05 |
| High | `"0"` | Columns (most contextually relevant) |
| Medium | `"1"` | Ref/source/variable/macro completions |
| Low | `"2"` | SQL functions |
| Lowest | `"9"` | Keywords (future) |

The `SortText` value becomes `prefix + label`, e.g., `"1orders"` for a ref completion or `"0customer_id"` for a column completion. LSP clients sort completion items lexicographically by `SortText`, so lower prefix values appear first.

## Files to Modify

### `/home/theotime.poulain/dbt-language-server/analysis/get_completion_items.go`

Six functions need their `SortText` field updated:

1. **`getRefCompletionItems`** (line 23): Change `SortText: k` to `SortText: "1" + k`
2. **`getSourceCompletionItems`** (line 186): Change `SortText: k` to `SortText: "1" + k`
3. **`getVariableCompletionItems`** (line 137): Change `SortText: k` to `SortText: "1" + k`
4. **`getMacroCompletionItems`** (line 117): Change `SortText: k` to `SortText: "1" + k`
5. **`getColumnCompletionItems`** (line 165): Change `SortText: col.Name` to `SortText: "0" + col.Name`

If `getScopeColumnCompletionItems` exists (from section-03), it should also use prefix `"0"`.

### `/home/theotime.poulain/dbt-language-server/docs/function_docs.go`

6. **`FunctionCompletionItems`** (line 32): Change `SortText: k` to `SortText: "2" + k`

## Tests

All tests go in `/home/theotime.poulain/dbt-language-server/analysis/get_completion_items_test.go`.

### Update Existing Tests

The existing `TestGetMacroCompletionItems` test constructs expected `CompletionItem` values with `SortText` set to the raw label (e.g., `SortText: "add_values"`). These must be updated to include the `"1"` prefix (e.g., `SortText: "1add_values"`). Same for `TestGetColumnCompletionItems` subtests that check `SortText` values -- update to `"0"` prefix.

### New Test: SortText Priority Prefixes

Add a test `TestSortTextPrefixes` that verifies each provider function produces items with the correct prefix. Table-driven structure with one subtest per provider:

- **getRefCompletionItems**: build a minimal `modelMap`, call the function, assert every returned item's `SortText` starts with `"1"`
- **getSourceCompletionItems**: build a minimal `sources` map, call the function, assert `SortText` starts with `"1"`
- **getVariableCompletionItems**: build a minimal `variables` map, call the function, assert `SortText` starts with `"1"`
- **getMacroCompletionItems**: use `expectedTestState()` or build minimal macro map, assert `SortText` starts with `"1"`
- **getColumnCompletionItems**: build a minimal `modelMap` with columns, assert `SortText` starts with `"0"`

For `FunctionCompletionItems` in the `docs` package, add a test in `/home/theotime.poulain/dbt-language-server/docs/function_docs_test.go` (create if it does not exist):

- Instantiate a `Dialect` (e.g., `"snowflake"`), call `FunctionCompletionItems()`, assert every returned item's `SortText` starts with `"2"`

### Test Stub Signatures

```go
// In analysis/get_completion_items_test.go
func TestSortTextPrefixes(t *testing.T) {
    // Subtests: "ref", "source", "variable", "macro", "column"
    // Each subtest: build minimal input, call provider, check SortText prefix
}
```

```go
// In docs/function_docs_test.go
func TestFunctionCompletionItemsSortText(t *testing.T) {
    // Instantiate Dialect("snowflake"), call FunctionCompletionItems()
    // Assert all items have SortText starting with "2"
}
```

### Verification Approach

For each subtest, iterate over all returned items and check `strings.HasPrefix(item.SortText, expectedPrefix)`. Also verify that `item.SortText == expectedPrefix + item.Label` (or the appropriate key used as the label basis) to ensure the full sort key is correct.

## Implementation Notes

- The change is purely mechanical: prepend a string literal to each `SortText` assignment.
- No new types, functions, or imports are needed in the production code (just string concatenation).
- The `docs` package change (`function_docs.go`) is in a different package from the `analysis` tests, so it needs its own test file or must be tested indirectly through integration tests in `analysis`.
- The `TestGetMacroCompletionItems` test in `get_completion_items_test.go` uses `reflect.DeepEqual` and will fail immediately if the `SortText` values are not updated in the expected items. This is the first thing to fix after changing the production code.

## Implementation Notes (Post-Implementation)

No deviations from plan. Purely mechanical changes.

### Files modified/created
- `analysis/get_completion_items.go` — MODIFIED (5 SortText prefix changes)
- `analysis/get_completion_items_test.go` — MODIFIED (updated TestGetMacroCompletionItems expected values, added TestSortTextPrefixes with 6 subtests)
- `docs/function_docs.go` — MODIFIED (1 SortText prefix change)
- `docs/function_docs_test.go` — NEW (1 test for function sort text)

### Test count: 6 subtests in TestSortTextPrefixes + 1 function docs test + existing tests updated