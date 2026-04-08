Now I have enough context. Let me produce the section content.

# Section 03: Undefined Reference Checks

## Overview

This section implements four reference-checking functions in `/home/theotime.poulain/dbt-language-server/analysis/diagnostics/refs.go`. Each function is a `Checker` (signature: `func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic`) that walks the document's `TokenIndex` looking for undefined refs, sources, vars, and macros.

**Depends on**: section-02-engine (provides the `Checker` type and `Engine` that registers/runs these checkers).

**File to create**: `/home/theotime.poulain/dbt-language-server/analysis/diagnostics/refs.go`
**Test file to create**: `/home/theotime.poulain/dbt-language-server/analysis/diagnostics/refs_test.go`

---

## Tests First

File: `/home/theotime.poulain/dbt-language-server/analysis/diagnostics/refs_test.go`

```go
// Test: valid ref -- ref('stg_orders') with stg_orders in ModelDetailMap -> no diagnostic
// Test: invalid ref -- ref('nonexistent') not in ModelDetailMap -> Error diagnostic
// Test: ref keyword filtering -- token with Literal=="ref" is skipped (not flagged)
// Test: diagnostic range spans the model name literal
// Test: multiple invalid refs in same file -> multiple diagnostics
// Test: message format is 'Model "nonexistent" not found'

// Test: valid source -- source('src', 'tbl') both exist -> no diagnostic
// Test: invalid source name -- source('bad_src', 'tbl') -> Error at SOURCE token
// Test: invalid table name -- source('src', 'bad_tbl') -> Error at SOURCE_TABLE token
// Test: source message format: 'Source "bad_src" not found'
// Test: table message format: 'Table "bad_tbl" not found in source "src"'
// Test: lookback from SOURCE_TABLE correctly finds associated SOURCE token

// Test: valid var -- var('my_var') in VariableDetailMap -> no diagnostic
// Test: valid var from set -- var defined via {% set %} in DefTokens -> no diagnostic
// Test: invalid var -- not in either map -> Warning diagnostic
// Test: severity is Warning not Error

// Test: valid macro -- my_macro() in MacroDetailMap -> no diagnostic
// Test: macro found in different package -> no diagnostic
// Test: invalid macro -- unknown_macro() not in any package -> Warning
// Test: Jinja builtin (if, for, set) -> skipped, no diagnostic
// Test: dbt builtin (ref, source, config, adapter) -> skipped, no diagnostic
// Test: severity is Warning not Error
```

All tests should be table-driven. Each test case constructs a `analysis.Document` with a parsed `TokenIndex` (by running `parser.Parse(sqlText, dialect)` on crafted SQL input) and an `analysis.DbtContext` with the relevant detail maps populated or empty as needed. The checker function is called directly; the returned `[]lsp.Diagnostic` slice is asserted on.

Test helper pattern:

```go
func makeDoc(sql string) analysis.Document {
    p := parser.Parse(sql, "duckdb")
    return analysis.Document{
        Text:      sql,
        Tokens:    p.CreateTokenIndex(),
        DefTokens: p.CreateTokenNameMap(),
    }
}
```

LSP severity constants: `1` = Error, `2` = Warning.

---

## Implementation Details

### Token Walking Pattern

All four checkers iterate the document's `TokenIndex` using the same pattern already established in `state.go` (`getReferencedModels`, `SemanticTokensFull`, `Hover`):

```go
for _, lineTokens := range doc.Tokens.LineTokens() {
    for _, tll := range lineTokens {
        // inspect tll.Token.Type and tll.Token.Literal
    }
}
```

`LineTokens()` returns `map[int][]TokenLL`. Each `TokenLL` has a `Token` field (with `Type`, `Literal`, `Line`, `Column`) and a `PrevToken *TokenLL` pointer for lookback.

### Diagnostic Range Construction

For each flagged token, build an `lsp.Range` spanning the literal:

```go
lsp.Range{
    Start: lsp.Position{Line: tok.Line, Character: tok.Column},
    End:   lsp.Position{Line: tok.Line, Character: tok.Column + len(tok.Literal)},
}
```

All diagnostics use `Source: "dbt-ls"`.

---

### CheckRefs (ref checker)

Walk all tokens. For each token where `Type == parser.REF`:

1. **Skip the `ref` keyword itself**: if `Literal == "ref"`, continue. The parser assigns type `REF` to both the `ref` keyword and the model name argument. This filter matches the existing pattern in `getReferencedModels` in `state.go`.
2. Look up `Literal` in `ctx.ModelDetailMap`.
3. If not found, emit an Error diagnostic with message: `Model "<literal>" not found`.

### CheckSources (source checker)

Walk all tokens. For each token where `Type == parser.SOURCE_TABLE`:

1. Use `tll.TokenLookbackMatch(parser.SOURCE, 4)` to find the associated `SOURCE` token. This walks up to 4 tokens back through the `PrevToken` chain looking for a token of type `SOURCE`. The lookback depth of 4 accounts for intervening quote tokens and commas between `source('name', 'table')`. This is the same pattern used by `Hover` in `state.go` (line 233).
2. If no SOURCE match found, skip (can't validate without source name).
3. Look up source name (from lookback literal) in `ctx.SourceDetailMap`.
4. If source not found: Error at the SOURCE token position. Message: `Source "<source_name>" not found`. To get the SOURCE token position, you need to walk back manually through `PrevToken` since `TokenLookbackMatch` only returns the literal, not the position. Alternative: walk back manually instead of using `TokenLookbackMatch`, storing both position and literal.
5. If source found but table name (`tll.Token.Literal`) not in `source.Tables`: Error at the SOURCE_TABLE token position. Message: `Table "<table_name>" not found in source "<source_name>"`.

**Important detail on getting SOURCE token position**: `TokenLookbackMatch` returns `(bool, string)` — the literal but not the token's position. For the "source not found" diagnostic you need the SOURCE token's line/column. Walk back through `PrevToken` manually to find the SOURCE-typed token and use its position directly. Alternatively, emit the error at the SOURCE_TABLE position with a message that includes the source name.

### CheckVars (var checker)

Walk all tokens. For each token where `Type == parser.VAR`:

1. Skip if `Literal == "var"` (same keyword-filtering pattern as refs).
2. Check `ctx.VariableDetailMap` for the var name.
3. Also check `doc.DefTokens` — this map (from `parser.CreateTokenNameMap()`) contains variables defined via `{% set %}` blocks in the same file.
4. If not in either: Warning diagnostic. Message: `Variable "<var_name>" not found`.
5. Severity is Warning (2), not Error (1), because vars can be defined in packages or passed via CLI `--vars`.

### CheckMacros (macro checker)

Walk all tokens. For each token where `Type == parser.MACRO`:

1. Check a builtin skip list. Maintain a package-level `map[string]bool` containing:
   - Jinja builtins: `if`, `for`, `set`, `block`, `macro`, `call`, `filter`, `raw`, `extends`, `include`, `import`, `from`, `do`, `print`, `with`, `autoescape`
   - dbt builtins: `ref`, `source`, `var`, `config`, `log`, `return`, `adapter`, `run_query`, `statement`, `exceptions`, `modules`, `flags`, `graph`, `model`, `this`, `target`, `env_var`, `project_name`
2. If token literal is in the skip list, continue.
3. Search across all packages in `ctx.MacroDetailMap`. The map type is `map[Package]map[string]Macro`. Iterate all packages looking for a match on the macro name:
   ```go
   found := false
   for _, macros := range ctx.MacroDetailMap {
       if _, ok := macros[tll.Token.Literal]; ok {
           found = true
           break
       }
   }
   ```
4. If not found: Warning diagnostic. Message: `Macro "<macro_name>" not found`.

---

## Key Types from the Codebase

These are the types the checker functions receive and interact with. They live in existing packages — do not redefine them.

- `analysis.Document` — has `Text string`, `Tokens *parser.TokenIndex`, `DefTokens map[string]parser.Token`
- `analysis.DbtContext` — has `ModelDetailMap map[string]ModelDetails`, `SourceDetailMap map[string]Source`, `MacroDetailMap map[Package]map[string]Macro`, `VariableDetailMap map[string]Variable`
- `parser.TokenLL` — has `Token parser.Token`, `PrevToken *TokenLL`; method `TokenLookbackMatch(tokenType TokenType, inc int) (bool, string)`
- `parser.Token` — has `Type TokenType`, `Literal string`, `Line int`, `Column int`
- `parser.TokenIndex` — method `LineTokens() map[int][]TokenLL`
- `lsp.Diagnostic` — has `Range lsp.Range`, `Message string`, `Severity int`, `Code string`, `Source string`
- Token type constants: `parser.REF`, `parser.SOURCE`, `parser.SOURCE_TABLE`, `parser.VAR`, `parser.MACRO`
- LSP severity: `1` = Error, `2` = Warning

---

## Registration

The four checker functions (`CheckRefs`, `CheckSources`, `CheckVars`, `CheckMacros`) are exported and registered with the Engine in section-05 (lifecycle integration). Each has the signature matching the `Checker` type defined in section-02:

```go
func CheckRefs(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic
func CheckSources(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic
func CheckVars(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic
func CheckMacros(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic
```

## Implementation Notes (Post-Implementation)

### Deviations from plan
- Test helper `makeDoc` uses `"snowflake"` dialect instead of plan's `"duckdb"` because `"set"` keyword is only in snowflakeKeywords map — duckdb dialect doesn't parse `{% set %}` blocks into `JINJA_SET` tokens.
- `CheckSources` emits diagnostics at SOURCE_TABLE position for both source-not-found and table-not-found (spec's acceptable alternative — avoids manual PrevToken walk for position).
- All diagnostics include `Source: "dbt-ls"` (added during code review, was missing in initial implementation).

### Files created
- `analysis/diagnostics/refs.go` — 4 checker functions + `tokenRange` helper + `macroBuiltins` skip list
- `analysis/diagnostics/refs_test.go` — 15 subtests across 4 test functions

### Test count: 15 (CheckRefs: 4, CheckSources: 3, CheckVars: 3, CheckMacros: 5)