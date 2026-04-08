Now I have all the context needed. Let me generate the section.

# Section 04: Dot-Qualified Completion

## Overview

This section implements dot-qualified column completion: when a user types `o.` after an alias like `FROM orders o`, the server suggests only columns belonging to that aliased source. This requires detecting the dot context, excluding Jinja blocks, filtering scope columns by alias, setting `FilterText`/`TextEdit` on returned items, and registering `.` as a trigger character.

## Dependencies

- **section-02-lsp-types**: Adds `FilterText` and `TextEdit` fields to `CompletionItem`, and changes `CompletionProvider` from `map[string]any` to typed `CompletionOptions` with trigger characters.
- **section-03-scope-completion**: Provides `getScopeColumnCompletionItems()` and scope-based column resolution via `ResolveColumnsAtPosition`. The dot-qualified path calls the same scope resolution but filters by alias.
- **Split 01 (QueryScope)**: `Document.Scope`, `State.ResolveColumnsAtPosition(uri, position)`, `ScopeColumn` with `Name`, `SourceName`, `SourceAlias`.

## Files to Modify

| File | Change |
|------|--------|
| `analysis/state.go` | Add dot-trigger detection at top of `TextDocumentCompletion` dispatch chain |
| `analysis/get_completion_items.go` | Add helper to build dot-qualified completion items with `FilterText` and `TextEdit` |
| `lsp/initialize.go` | Change `CompletionProvider` field type and value (covered by section-02, listed here for awareness) |

## Tests (Write First)

All tests go in `/home/theotime.poulain/dbt-language-server/analysis/state_lsp_test.go`, added to the existing `TestTextDocumentCompletion` table or as a new test function.

### Dot trigger tests

Table-driven tests following the existing pattern. Each test calls `state.parseDocument(uri, sql)` then `state.TextDocumentCompletion(1, uri, pos)` and checks the returned items.

**Test cases to add:**

1. **`o.` after `FROM orders o`** -- returns only columns from the "orders" source (alias "o"). Verify items have `Kind == completionKind.Field`. Verify no columns from other sources appear.

2. **`c.` after JOIN with `customers c`** -- returns only columns from the "customers" source (alias "c").

3. **`{{ dbt_utils.` is NOT treated as dot-qualified** -- cursor is inside a Jinja block. Should return jinja/macro completions, not column completions. Verify no `Field` kind items are returned.

4. **`{{ config.` is NOT treated as dot-qualified** -- same Jinja exclusion logic. No column items.

5. **Dot after unknown alias** -- e.g., `x.` where `x` is not a known alias in scope. Returns empty items (no crash).

6. **FilterText set correctly** -- for each returned item, `FilterText` equals `"alias.column_name"` (e.g., `"o.customer_id"`).

7. **TextEdit present with correct range** -- each item has a non-nil `TextEdit` whose `Range` starts after the `.` character and whose `NewText` is the column name.

These tests require the scope engine from split 01 to be functional. If scope is nil, the dot-qualified path should not activate (backward compat from section-03 handles this).

### Test structure sketch

```go
// In state_lsp_test.go, new test or appended to TestTextDocumentCompletion

// Test: dot-qualified completion filters by alias
// SQL: "select o. from {{ ref('orders') }} o"
// Position: after "o." (Character offset at the dot+1)
// Check: all returned Field items have FilterText starting with "o."

// Test: Jinja dot is NOT column completion
// SQL: "select {{ dbt_utils."
// Position: after the dot
// Check: no Field kind items returned

// Test: unknown alias returns empty
// SQL: "select x. from {{ ref('orders') }}"
// Position: after "x."
// Check: no Field items returned (x is not a valid alias)
```

## Implementation Details

### 1. Dot Context Detection

In `TextDocumentCompletion` (`analysis/state.go`), add a dot-trigger check as the **first** check in the dispatch chain (before `refTriggerRegex`). This matches the dispatch order specified in the plan:

```
1. Dot-trigger check (textBeforeCursor ends with "identifier.", NOT inside {{ }})
2. Macro-inside-call check  (section-05)
3. refTriggerRegex
4. sourceTriggerRegex
...
```

Detection logic:

1. Use a regex like `\b([a-zA-Z_][a-zA-Z0-9_]*)\.$` on `textBeforeCursor` to check if it ends with `identifier.`
2. If matched, check whether the cursor is inside a Jinja block by counting unmatched `{{` vs `}}` in `textBeforeCursor`. If `count({{) > count(}})`, the cursor is inside Jinja -- skip dot-qualified handling entirely and fall through to the normal dispatch chain.
3. If valid SQL dot context, extract the alias (capture group 1 from the regex).

### 2. Jinja Block Exclusion

The Jinja exclusion is critical to avoid false positives. Examples that must NOT trigger dot-qualified:
- `{{ dbt_utils.star(...) }}` -- `dbt_utils.` is a package qualifier
- `{{ config.get(...) }}` -- Jinja object method
- `{{ loop.index }}` -- Jinja loop variable

Implementation: count occurrences of `{{` and `}}` in `textBeforeCursor`. If `{{` count exceeds `}}` count, cursor is inside a Jinja expression block. This is a simple string count, not regex.

A helper function `isInsideJinjaBlock(text string) bool` can be extracted for reuse (section-05 needs the same check for macro arg detection).

### 3. Alias-Filtered Scope Resolution

Once a valid alias is extracted:

1. Call `State.ResolveColumnsAtPosition(uri, position)` (or equivalent scope resolution from section-03).
2. Filter the returned `[]ScopeColumn` to only those where `ScopeColumn.SourceAlias == alias`.
3. If no columns match the alias, return an empty completion list.

### 4. Building Dot-Qualified Completion Items

Create a function (or extend `getScopeColumnCompletionItems`) that sets dot-qualified fields on each item:

For each matching `ScopeColumn`:
- `Label`: column name (e.g., `"customer_id"`)
- `Kind`: `completionKind.Field`
- `Detail`: source attribution (e.g., `"Column from orders (o)"`)
- `FilterText`: `alias + "." + columnName` (e.g., `"o.customer_id"`) -- allows the client to filter correctly since the trigger was on `.`
- `TextEdit`: a `lsp.TextEdit` with:
  - `Range`: from `(line, dot_position + 1)` to `(line, cursor_position)` -- replaces any typed prefix after the dot
  - `NewText`: the column name
- `SortText`: `"0" + columnName` (same priority as regular columns)
- `InsertText`: can be left empty when `TextEdit` is present (LSP spec: `TextEdit` takes precedence)

### 5. Trigger Character Registration

Section-02 handles changing `CompletionProvider` in `lsp/initialize.go` from `map[string]any{}` to:

```go
CompletionProvider: CompletionOptions{
    TriggerCharacters: []string{"."},
},
```

This section depends on that change being in place. The `.` trigger character causes the client to send a completion request when the user types `.`, which is what makes `textBeforeCursor` end with `identifier.` at the right time.

### 6. TextEdit Range Calculation

The `TextEdit` range needs to be computed from the cursor position:
- The dot is at `position.Character - 1` (since `textBeforeCursor` ends with `.`)
- The range starts at `position.Character` (right after the dot) -- or if the user has typed partial text after the dot, the range extends from dot+1 to cursor
- For the trigger-on-dot case (no text after dot yet), the range is zero-width at cursor position

In practice, since `textBeforeCursor` ends exactly with `.`, the start column for the TextEdit is `position.Character` and the end column is also `position.Character` (zero-width range, pure insertion). The `NewText` is the column name.

### 7. Dispatch Chain Integration

The dot-trigger check must be the very first check. If the check determines we are NOT in a dot context (no match, or inside Jinja), execution falls through to the existing dispatch chain unchanged. This ensures no regression on existing completion behavior.

Pseudocode for the dispatch:

```go
func (s *State) TextDocumentCompletion(...) {
    // ... existing text extraction ...

    // NEW: Dot-qualified check (first priority)
    if alias, ok := detectDotContext(textBeforeCursor); ok {
        // resolve columns filtered by alias
        // build items with FilterText and TextEdit
        // return response
    }

    // ... existing dispatch chain unchanged ...
    if refTriggerRegex.MatchString(textBeforeCursor) {
        // ...
    } else if sourceTriggerRegex.MatchString(textBeforeCursor) {
        // ...
    }
    // ...
}
```

The `detectDotContext` function returns the alias and a boolean. It returns `false` if no dot pattern is found or if the cursor is inside a Jinja block.

## Implementation Notes (Post-Implementation)

Deviations from plan:
- Detail format uses `col.Source` directly (e.g., "Column from o") instead of plan's "Column from orders (o)". Source is the alias when present, sufficient for disambiguation.
- `isInsideJinjaBlock` only counts `{{ }}`, not `{% %}`. Known limitation.
- Added nil-scope guard before `resolveSourceColumns` call (code review fix).

### Files modified
- `analysis/state.go` — MODIFIED (added `isInsideJinjaBlock`, `detectDotContext`, `getDotQualifiedCompletionItems`, dot-trigger dispatch, `dotTriggerRegex`, re-added `completionKind` import)
- `analysis/state_lsp_test.go` — MODIFIED (added 7 subtests in TestDotQualifiedCompletion)

### Test count: 7 subtests, all existing tests pass