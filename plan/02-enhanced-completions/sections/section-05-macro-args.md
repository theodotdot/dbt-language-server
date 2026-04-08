Now I have all the context needed to write the section.

# Section 05: Macro Argument Completion

## Overview

This section adds completion of macro argument names when the cursor is inside an existing macro call within a Jinja `{{ }}` block. For example, with cursor at `|` in `{{ my_macro(|) }}`, the server suggests `arg1=`, `arg2=`, etc.

Depends on **section-03-scope-completion** (for the updated dispatch chain ordering -- macro arg check must be inserted before `jinjaBlockTriggerRegex`).

## Key Files

| File | Action |
|------|--------|
| `/home/theotime.poulain/dbt-language-server/analysis/get_completion_items.go` | Add `getMacroArgCompletionItems()` |
| `/home/theotime.poulain/dbt-language-server/analysis/state.go` | Add macro-inside-call detection + dispatch in `TextDocumentCompletion` |
| `/home/theotime.poulain/dbt-language-server/analysis/get_completion_items_test.go` | Unit tests for `getMacroArgCompletionItems` |
| `/home/theotime.poulain/dbt-language-server/analysis/state_lsp_test.go` | Integration tests for macro arg context detection |

## Existing Structures

The `Macro` type (in `analysis/parse_macros.go`) already has the data needed:

```go
type MacroArg struct {
    Name    string
    Default string
}

type Macro struct {
    Name        string
    ProjectName Package
    Description string
    Arguments   []MacroArg
    URI         string
    Range       lsp.Range
}
```

The `MacroDetailMap` in `DbtContext` is typed `map[Package]map[string]Macro`, keyed by package then macro name.

The dispatch chain in `TextDocumentCompletion` (in `analysis/state.go:440-484`) currently checks `refTriggerRegex`, `sourceTriggerRegex`, `varTriggerRegex`, `jinjaBlockTriggerRegex` in order, then falls through to column completion.

The `jinjaBlockTriggerRegex` is `\{\{\s*` -- this would match `{{ my_macro(|)` since it starts with `{{ `. The macro-arg check must therefore be inserted **before** `jinjaBlockTriggerRegex` in the dispatch chain.

## Tests

### Unit tests: `get_completion_items_test.go`

Add a `TestGetMacroArgCompletionItems` test function with table-driven cases:

- **3 args, 0 provided**: Pass a `Macro` with 3 arguments and an empty `providedArgs` set. Expect 3 completion items returned.
- **1 arg already provided (keyword)**: Pass a `Macro` with 3 arguments and `providedArgs` containing one name. Expect 2 items returned, excluding the already-provided arg.
- **All args provided**: All argument names in `providedArgs`. Expect empty result.
- **InsertText format**: Each returned item must have `InsertText` ending with `=` (e.g., `"arg_name="`).
- **Kind**: Each item must have `Kind` equal to `completionKind.Variable` (6).
- **SortText prefix**: Each item's `SortText` must start with `"00"`.
- **Detail field**: Items with a default value show the default in `Detail`; items without show `"required"`.

### Integration tests: `state_lsp_test.go`

Add cases to `TestTextDocumentCompletion` (or a new test function):

- **Cursor inside `{{ my_macro(|) }}`**: Parse the SQL, set cursor after the `(`. Expect completion items matching `my_macro`'s arguments (the test state already has `my_macro` with `arg1`).
- **Cursor inside nested call `{{ my_macro(func(x), |) }}`**: Paren-counting must correctly identify `my_macro` as the outer function. Expect `my_macro`'s args.
- **Cursor after closed call `{{ my_macro() }}|`**: Cursor is outside the parens. Must NOT trigger macro arg completion.
- **Cursor inside `{{ ref('|') }}`**: The `ref` trigger regex fires first. Must NOT fall through to macro arg completion. (This is already handled by dispatch order.)
- **Cursor inside packaged macro `{{ other_pkg.ext_macro(|) }}`**: Must resolve `ext_macro` from `other_pkg` in `MacroDetailMap`. The macro name extraction must handle the `package.name(` pattern -- extract only the name portion after the dot.

## Implementation Details

### Context Detection via Paren-Counting

Add a helper function (e.g., `detectMacroCallContext`) that takes `textBeforeCursor string` and returns `(macroName string, argsText string, ok bool)`.

Algorithm:
1. Walk backward through `textBeforeCursor` character by character.
2. Track paren depth: `)` increments depth, `(` decrements depth.
3. When depth reaches 0, we found the matching `(` for our cursor position.
4. Extract the identifier immediately before that `(` -- this is the macro name. Use a regex like `[a-zA-Z_][a-zA-Z0-9_]*$` on the text before the `(`.
5. Verify we are inside a `{{ }}` block: count unmatched `{{` vs `}}` in the text before cursor. If the count of `{{` exceeds `}}`, we are inside Jinja.
6. If not inside Jinja, return `ok = false`.
7. The `argsText` is the substring between the opening `(` and the cursor -- used to detect already-provided keyword args.

Handle the edge case where the macro name is package-qualified (e.g., `other_pkg.ext_macro`): the identifier before `(` is `ext_macro`. When looking up in `MacroDetailMap`, search across all packages for a macro with that name.

### Determining Already-Provided Arguments

Scan `argsText` for `identifier=` patterns using a regex like `\b([a-zA-Z_][a-zA-Z0-9_]*)\s*=`. Collect all matched identifiers into a set. These are keyword arguments already provided by the user.

### Building Completion Items

Create `getMacroArgCompletionItems(macro Macro, providedArgs map[string]bool) []lsp.CompletionItem`.

For each argument in `macro.Arguments` not in `providedArgs`:
- `Label`: argument name
- `Kind`: `completionKind.Variable` (6)
- `Detail`: default value if `arg.Default != ""`, otherwise `"required"`
- `InsertText`: `argName + "="` (trailing equals to prompt value entry)
- `SortText`: `"00" + argName` (highest priority)

### Dispatch Integration

In `TextDocumentCompletion` in `analysis/state.go`, insert the macro-arg check **before** the `jinjaBlockTriggerRegex` check. The new dispatch order for the relevant portion:

```
} else if varTriggerRegex.MatchString(textBeforeCursor) {
    ...
} else if macroName, argsText, ok := detectMacroCallContext(textBeforeCursor); ok {
    // Look up macro by name across all packages
    // Extract provided args from argsText
    // Return getMacroArgCompletionItems(macro, providedArgs)
} else if jinjaBlockTriggerRegex.MatchString(textBeforeCursor) {
    ...
```

### Macro Lookup

To find the macro by name, iterate over all packages in `MacroDetailMap`:

```go
func (s *State) findMacroByName(name string) (Macro, bool) {
    for _, macroMap := range s.DbtContext.MacroDetailMap {
        if m, ok := macroMap[name]; ok {
            return m, true
        }
    }
    return Macro{}, false
}
```

If the macro is not found in `MacroDetailMap` (e.g., it is a built-in Jinja function or unknown), return no completions and let the dispatch chain continue to the next check.

## Implementation Notes (Post-Implementation)

Deviations from plan:
- `extractProvidedArgs` uses index-based matching + post-filter to exclude `==` comparisons (Go RE2 doesn't support negative lookahead `(?!=)`)
- When macro not found by `findMacroByName`, falls through to `getMacroCompletionItems` (macro name completions) instead of returning empty
- String literal parens inside macro calls not handled — known limitation

### Files modified
- `analysis/get_completion_items.go` — MODIFIED (added `getMacroArgCompletionItems`)
- `analysis/state.go` — MODIFIED (added `detectMacroCallContext`, `extractProvidedArgs`, `findMacroByName`, `macroNameRegex`, `kwargRegex`, dispatch integration)
- `analysis/get_completion_items_test.go` — MODIFIED (7 unit subtests)
- `analysis/state_lsp_test.go` — MODIFIED (4 integration subtests)

### Test count: 7 unit + 4 integration subtests, all existing tests pass