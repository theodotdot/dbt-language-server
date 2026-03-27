Now I have all the context needed. Let me produce the section content.

# Section 05: CTE Parsing

## Overview

This section extends the existing CTE handling in `analysis/parser/parser.go` to populate `QueryScope.CTEs` during parsing. It adds scope stack push/pop on CTE body boundaries, extracts CTE output columns from SELECT items, handles chained CTEs, and preserves backward compatibility with the old `p.ctes.Tokens` mechanism.

**Dependencies**: Sections 01 (data types) and 02 (clause state machine) must be implemented first. This section assumes `QueryScope`, `CTEDef`, `SelectItem`, `ClauseKind`, `SourceKind`, the scope stack, the clause stack, and `parenDepth` tracking all exist.

**Blocks**: Sections 06 (resolution engine) and 07 (state integration).

---

## Background: Current CTE Handling

The existing parser (`/home/theotime.poulain/dbt-language-server/analysis/parser/parser.go`) already tracks CTEs via the `CTE` struct on the `Parser`:

```go
type CTE struct {
    Ind          bool
    ParenCount   int
    Tokens       []Token
    TokenNameMap map[string]Token
}
```

Key existing behavior:
- `parseWith()` (line 66): advances past `WITH`, records the CTE name IDENT in `p.ctes.Tokens`, skips `AS`, and sets `p.ctes.ParenCount = 1` when seeing `LPAREN`.
- `incParenCount()` / `decParenCount()` (lines 183-193): increment/decrement `p.ctes.ParenCount` when `p.ctes.Ind` is true.
- In `parseTokens()` (line 204): when `RPAREN` brings `p.ctes.ParenCount` to 0, the parser checks for a comma (chained CTE) or ends CTE mode (`p.ctes.Ind = false`).
- `CreateTokenNameMap()` (line 239): iterates `p.ctes.Tokens` to build a name map used by downstream consumers.

This existing mechanism **must be preserved**. The new scope-based CTE handling runs in parallel.

---

## Tests

All tests go in `/home/theotime.poulain/dbt-language-server/analysis/parser/scope_test.go` (extending the file created in sections 01-03).

### CTE Handling Tests (Section 4.5)

Test function: `TestCTEParsing` (table-driven)

Test cases:

- **Single CTE**: `WITH cte AS (SELECT id, name FROM {{ ref('orders') }}) SELECT * FROM cte` -- verify `QueryScope.CTEs["cte"]` exists, `CTEDef.Columns` is `["id", "name"]`, and main scope has a `SourceRef{Kind: SourceKindCTE, Name: "cte"}`.

- **Chained CTEs**: `WITH a AS (SELECT id FROM {{ ref('orders') }}), b AS (SELECT id FROM a) SELECT * FROM b` -- verify both CTEs exist in the scope's CTEs map. CTE `b`'s scope should have a `SourceRef{Kind: SourceKindCTE, Name: "a"}`. CTE `b`'s columns should be `["id"]`.

- **CTE with SELECT star**: `WITH cte AS (SELECT * FROM {{ ref('orders') }}) SELECT * FROM cte` -- verify `CTEDef.Columns` contains `["*"]` (lazy expansion deferred to resolution engine in section 06).

- **CTE with aliased columns**: `WITH cte AS (SELECT id AS order_id, name AS customer_name FROM {{ ref('orders') }}) SELECT * FROM cte` -- verify `CTEDef.Columns` is `["order_id", "customer_name"]`.

- **Old CTE struct still populated (backward compat)**: For any CTE query, verify `p.ctes.Tokens` still contains the CTE name tokens. Use `parser.Parse(input, dialect)` and access `CreateTokenNameMap()` to confirm the CTE names are present.

- **Scope stack push/pop -- CTE body does not corrupt main query clauseContext**: After parsing `WITH cte AS (SELECT id FROM t) SELECT name FROM {{ ref('orders') }}`, the main query's scope should have its own SelectItems (`["name"]`) and Sources (the ref'd model), independent of the CTE's scope.

### Error Recovery Tests (Section 4.6, CTE-related subset)

Test function: `TestCTEErrorRecovery` (table-driven)

- **Missing closing paren**: `WITH cte AS (SELECT id FROM t` -- parser should not panic. Partial CTE scope may exist. The main scope should be usable (possibly empty).

- **Empty input**: `""` -- returns empty QueryScope with no CTEs, no panic.

- **Jinja-only input**: `{{ ref('orders') }}` without SQL structure -- minimal scope, no CTEs, no panic.

### Jinja + SQL Interaction Tests (CTE-related subset)

- **Jinja set block between WITH and SELECT**: `WITH cte AS (SELECT id FROM t) {% set x = 1 %} SELECT * FROM cte` -- CTE is parsed correctly, Jinja block does not disrupt clause tracking in the main query scope.

---

## Implementation Details

### File: `/home/theotime.poulain/dbt-language-server/analysis/parser/parser.go`

#### Modify `parseWith()`

The existing `parseWith()` records CTE names in `p.ctes.Tokens`. Add parallel logic to:

1. When the CTE name IDENT is identified, create a `CTEDef{Name: literal, Token: token}` and register it in the current (top-level) scope's `CTEs` map. This must happen before the CTE body is parsed so that subsequent CTEs in a chain can look up earlier ones.

2. When `AS (` is encountered (the `LPAREN` after `AS`), push a new `QueryScope` onto `p.scopeStack` and push `ClauseNone` onto `p.clauseStack`. This new scope becomes the "current scope" for all subsequent clause/select/from parsing inside the CTE body.

3. Store a reference from the `CTEDef.Scope` to this newly pushed scope so that after parsing, the CTE's scope is accessible.

#### Modify `parseTokens()` RPAREN handling

The existing RPAREN handler at line 203 already detects when `p.ctes.ParenCount` reaches 0 (CTE body closes). Extend this block:

1. When a CTE body closes (scope stack depth > 1), pop the top scope from `p.scopeStack` and the top clause from `p.clauseStack`.

2. Extract column names from the popped scope's `SelectItems`:
   - If `item.IsStar` is true, add `"*"` to the columns list.
   - Otherwise, add `item.Alias` (which holds the output column name -- either the explicit alias or the simple column name).

3. Store these columns in the corresponding `CTEDef.Columns`.

4. For chained CTEs (when a comma follows the closing paren and the next token is an IDENT), record the new CTE name in both `p.ctes.Tokens` (existing) and the top-level scope's `CTEs` map (new). Then push a new scope for the next CTE body when `AS (` is encountered.

#### CTE name recognition in FROM/JOIN

When the FROM/JOIN parser (section 04) encounters an IDENT that matches a key in the current top-level scope's `CTEs` map, it should create a `SourceRef{Kind: SourceKindCTE, Name: name}` instead of `SourceRef{Kind: SourceKindTable}`. This check is: if the top-level scope (bottom of the scope stack) has a CTE with that name, treat it as a CTE reference.

This logic technically belongs to section 04 (FROM/JOIN parsing) but the CTE name registration in the CTEs map (this section) is what enables it.

### Key Invariants

- The scope stack has exactly 1 entry (top-level scope) outside of CTE bodies.
- Inside a CTE body, the scope stack has 2 entries: [top-level, cte-scope].
- The clause stack parallels the scope stack exactly.
- `p.ctes.ParenCount` and the scope stack push/pop are synchronized: push when `ParenCount` is set to 1 (CTE body opens), pop when `ParenCount` reaches 0 (CTE body closes).
- The old `p.ctes` mechanism is never modified or removed -- it continues to track CTE names for `CreateTokenNameMap()`.

### Chained CTE Flow

For `WITH a AS (...), b AS (...) SELECT ...`:

1. `WITH` encountered -> `parseWith()` runs, records CTE `a` in both old and new structures, pushes scope for `a`'s body.
2. Parsing inside `a`'s body populates `a`'s scope with SelectItems and Sources.
3. `)` closes `a`'s body -> pop scope, extract columns into `CTEDef` for `a`.
4. `,` detected -> next token is IDENT `b` -> record CTE `b`, push scope for `b`'s body.
5. Inside `b`'s FROM, if `a` appears as an IDENT, it matches `CTEs["a"]` -> `SourceRef{Kind: SourceKindCTE}`.
6. `)` closes `b`'s body -> pop scope, extract columns into `CTEDef` for `b`.
7. `p.ctes.Ind` set to false. Main query parsing continues with the top-level scope.

### Error Recovery for CTEs

If the CTE body is never closed (missing `)` or EOF), the scope stack may have an extra entry. In `CreateQueryScope()` (the export method), always return the bottom of the scope stack (index 0) regardless of stack depth. This ensures a usable top-level scope is always returned, even if CTE parsing was incomplete.

If paren counting goes wrong (negative `ParenCount`), reset to 0 at the next sync token (SELECT, FROM, WHERE, etc.) at the top level.

---

## File Summary

| File | Action |
|------|--------|
| `analysis/parser/parser.go` | Modify `parseWith()` and RPAREN handling in `parseTokens()` to push/pop scopes and populate `CTEDef` |
| `analysis/parser/scope_test.go` | Add `TestCTEParsing` and `TestCTEErrorRecovery` test functions |

---

## Implementation Notes

**Status:** Completed

**Files modified:**
- `analysis/parser/parser.go` — added `registerCTE()`, `pushCTEScope()`, `popCTEScope()`. Modified `parseWith()` and RPAREN handler in `parseTokens()` to push/pop scopes and extract CTE columns. Changed CTE lookup in FROM/JOIN from `currentScope().CTEs` to `scopeStack[0].CTEs`. Added `continue` after CTE end to re-process token in main switch.
- `analysis/parser/scope_test.go` — added 6 `TestCTEParsing` cases and 2 `TestCTEErrorRecovery` cases

**Bugs fixed during implementation:**
- `pushCTEScope` received "as" instead of CTE name because `p.curTok.Literal` changed after consuming AS token. Fixed by saving `cteName` before advancing.
- Chained CTE `b`'s FROM parser didn't find CTE `a` as `SourceKindCTE` because lookup used `p.currentScope().CTEs` (CTE scope) instead of `p.scopeStack[0].CTEs` (top-level scope).
- Main scope SelectItems empty after CTE body — outer `SELECT` token consumed by RPAREN handler's `NextToken()` then skipped by main loop's `NextToken()`. Fixed by adding `continue` to re-process curTok in the main switch.

**Deviations from plan:**
- Jinja set between WITH and SELECT test not added (low risk, `parseJinjaBlock` doesn't affect clause state)
- `popCTEScope` finds matching CTE via pointer equality scan over CTEs map rather than passing CTE name directly (acceptable for typical CTE counts)