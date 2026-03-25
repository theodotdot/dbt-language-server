Now I have all the context needed. Here is the section content:

# Section 02: Clause State Machine

## Overview

This section adds the scope stack, clause stack, and paren depth tracking to the `Parser` struct. It extends the `parseTokens()` main loop to detect SQL clause boundary keywords (SELECT, FROM, JOIN, WHERE, GROUP, ORDER, HAVING) at `parenDepth == 0` and update clause context. It also stores clause start positions in `QueryScope.ClauseRanges` and adds the `CreateQueryScope()` export method.

## Dependencies

- **section-01-data-types** must be completed first. This section assumes the following types exist in `/home/theotime.poulain/dbt-language-server/analysis/parser/scope.go`:
  - `QueryScope` with fields: `Sources []*SourceRef`, `CTEs map[string]*CTEDef`, `SelectItems []SelectItem`, `ClauseRanges map[ClauseKind]Token`, `Parent *QueryScope`
  - `ClauseKind` enum with: `ClauseNone`, `ClauseWith`, `ClauseSelect`, `ClauseFrom`, `ClauseJoin`, `ClauseWhere`, `ClauseGroupBy`, `ClauseOrderBy`, `ClauseHaving`, `ClauseOn`
  - `NewQueryScope()` constructor

## Files to Modify

- `/home/theotime.poulain/dbt-language-server/analysis/parser/parser.go` -- add new fields to `Parser` struct, extend `parseTokens()`, add `CreateQueryScope()`

## Files to Create

- `/home/theotime.poulain/dbt-language-server/analysis/parser/scope_test.go` -- clause state machine tests

---

## Tests (Write First)

File: `/home/theotime.poulain/dbt-language-server/analysis/parser/scope_test.go`

All tests use the existing `Parse()` function followed by `CreateQueryScope()` and inspect the resulting scope. Use `docs.DuckDB` (or equivalent dialect constant) for all tests. Table-driven style.

### Test: SELECT keyword sets clauseContext to ClauseSelect

Parse `SELECT id FROM t`. Verify `scope.ClauseRanges[ClauseSelect]` exists and has a valid position (line/column corresponding to the SELECT keyword).

### Test: FROM keyword sets clauseContext to ClauseFrom

Parse `SELECT id FROM t`. Verify `scope.ClauseRanges[ClauseFrom]` exists.

### Test: JOIN keyword sets clauseContext to ClauseJoin

Parse `SELECT id FROM t JOIN t2 ON t.id = t2.id`. Verify `scope.ClauseRanges[ClauseJoin]` exists.

### Test: WHERE keyword sets clauseContext to ClauseWhere

Parse `SELECT id FROM t WHERE id = 1`. Verify `scope.ClauseRanges[ClauseWhere]` exists.

### Test: Clause keywords inside parentheses do NOT change clauseContext

Parse `SELECT id FROM t WHERE id IN (SELECT id FROM t2)`. Verify `scope.ClauseRanges[ClauseFrom]` position corresponds to the outer FROM (not the subquery's FROM). The number of ClauseRanges entries for `ClauseFrom` should reflect only the outer query. The subquery's SELECT/FROM at `parenDepth > 0` must not overwrite the outer clause ranges.

### Test: ClauseRanges stores correct start positions for each clause

Parse a multi-clause query: `SELECT id, name FROM orders WHERE id > 0 GROUP BY name ORDER BY id HAVING count(*) > 1`. Verify all of `ClauseSelect`, `ClauseFrom`, `ClauseWhere`, `ClauseGroupBy`, `ClauseOrderBy`, `ClauseHaving` are present in `ClauseRanges` with monotonically increasing positions.

### Test: Empty input returns empty QueryScope without panic

Parse empty string `""`. Call `CreateQueryScope()`. Verify result is non-nil with empty `Sources`, `SelectItems`, and `ClauseRanges`.

### Test: CreateQueryScope returns non-nil scope

Parse any valid SQL. Verify `CreateQueryScope()` returns a non-nil `*QueryScope`.

---

## Implementation Details

### 1. Add New Fields to Parser Struct

In `/home/theotime.poulain/dbt-language-server/analysis/parser/parser.go`, add three fields to the `Parser` struct:

```go
type Parser struct {
    // ... existing fields (l, curTok, peekTok, tokens, ctes, setVars) ...
    scopeStack    []*QueryScope
    clauseStack   []ClauseKind
    parenDepth    int
}
```

- `scopeStack`: stack of active `QueryScope` pointers. The bottom element is the top-level query scope. Pushing happens when entering CTE bodies (section-05). Popping happens when exiting them.
- `clauseStack`: parallel to `scopeStack`. Each scope level tracks its own current clause context. The top of this stack is the "current clause."
- `parenDepth`: tracks parenthesis nesting within the current scope. Clause boundary detection and alias detection only apply at `parenDepth == 0`.

### 2. Initialize Scope Stack in NewParser

Modify `NewParser()` to initialize the scope stack with one top-level scope and one `ClauseNone` entry:

```go
func NewParser(input string, dialect docs.Dialect) *Parser {
    topScope := NewQueryScope()
    return &Parser{
        l: New(input, dialect),
        ctes: CTE{...},  // existing
        setVars: make(map[string]Token),
        scopeStack:  []*QueryScope{topScope},
        clauseStack: []ClauseKind{ClauseNone},
        parenDepth:  0,
    }
}
```

### 3. Helper Methods on Parser

Add convenience methods to access current scope and clause context:

- `currentScope() *QueryScope` -- returns top of `scopeStack`
- `currentClause() ClauseKind` -- returns top of `clauseStack`
- `setClause(kind ClauseKind)` -- sets the top of `clauseStack` to the given kind; also records the clause start position in `currentScope().ClauseRanges[kind]` using `p.curTok`

### 4. Extend parseTokens() Main Loop

The existing `parseTokens()` switch statement must be extended to detect clause boundary keywords. The critical constraint: clause transitions only happen when `parenDepth == 0`. This prevents subquery keywords from corrupting the outer query's clause state.

Add cases (or extend existing cases) for these keywords in the main `switch p.curTok.Type` block:

| Token Type | Action |
|---|---|
| `SELECT` | If `parenDepth == 0`, call `setClause(ClauseSelect)` |
| `FROM` | If `parenDepth == 0`, call `setClause(ClauseFrom)` |
| `JOIN` | If `parenDepth == 0`, call `setClause(ClauseJoin)`. This covers bare `JOIN`; see below for compound joins. |
| `WHERE` | If `parenDepth == 0`, call `setClause(ClauseWhere)` |
| `GROUP` | If `parenDepth == 0` and `peekTok.Type == BY`, call `setClause(ClauseGroupBy)` |
| `ORDER` | If `parenDepth == 0` and `peekTok.Type == BY`, call `setClause(ClauseOrderBy)` |
| `HAVING` | If `parenDepth == 0`, call `setClause(ClauseHaving)` |
| `ON` | If `parenDepth == 0`, call `setClause(ClauseOn)` |

**Compound JOIN keywords**: `LEFT`, `RIGHT`, `INNER`, `CROSS`, `FULL`, `NATURAL` precede `JOIN`. The clause transition to `ClauseJoin` happens on the `JOIN` token itself, not on the modifier. No special handling needed for modifiers -- they are consumed normally, and when `JOIN` follows, it triggers `setClause(ClauseJoin)`.

**Important**: The `SELECT` token type does not currently exist as a case in the `parseTokens()` switch. It falls through to the default (no-op). New cases must be added for `SELECT`, `FROM`, `JOIN`, `WHERE`, `GROUP`, `ORDER`, `HAVING`, and `ON`. These keywords are already recognized by `LookupIdent()` in both Snowflake and DuckDB keyword maps.

### 5. Paren Depth Tracking

The existing `incParenCount()` and `decParenCount()` methods manage `p.ctes.ParenCount` for CTE tracking. The new `parenDepth` field is separate and serves SQL structure parsing.

In the `LPAREN` case of `parseTokens()`, add `p.parenDepth++` alongside the existing `p.incParenCount()`.

In the `RPAREN` case, add `p.parenDepth--` alongside the existing `p.decParenCount()`. If `p.parenDepth` goes negative (paren mismatch / error recovery), reset it to 0.

### 6. CreateQueryScope() Export Method

Add a method that returns the bottom of the scope stack (the top-level query scope):

```go
func (p *Parser) CreateQueryScope() *QueryScope {
    if len(p.scopeStack) == 0 {
        return NewQueryScope()
    }
    return p.scopeStack[0]
}
```

The bottom element (`scopeStack[0]`) is always the top-level query scope. CTE body scopes are pushed/popped during parsing (section-05) and are not present in the stack after parsing completes. They are accessible via `QueryScope.CTEs`.

### 7. Backward Compatibility

All existing behavior must be preserved:
- The existing `p.ctes` struct and its `incParenCount`/`decParenCount` continue to work unchanged.
- `CreateTokenIndex()` and `CreateTokenNameMap()` are unaffected.
- The `WITH` case still calls `parseWith()` as before. The clause state machine adds `setClause(ClauseWith)` logic but does not interfere with `parseWith()`'s token consumption.
- The `WITH` case should set `setClause(ClauseWith)` before calling `parseWith()` when `parenDepth == 0`.

### 8. Token Type Availability

The clause boundary keywords (`SELECT`, `FROM`, `JOIN`, `WHERE`, `GROUP`, `ORDER`, `HAVING`, `ON`) are all defined as constants in `/home/theotime.poulain/dbt-language-server/analysis/parser/token.go` and are present in both `snowflakeKeywords` and `duckdbKeywords` maps. The lexer already classifies them via `LookupIdent()`. No changes to `token.go` are needed for this section -- `ClauseKind` is defined in `scope.go` (section-01).

---

## Implementation Notes

**Status:** Completed

**Files modified:**
- `analysis/parser/parser.go` — added scopeStack/clauseStack/parenDepth fields, scope helpers, clause detection in parseTokens(), CreateQueryScope()
- `analysis/parser/scope_test.go` — added 7 clause state machine tests

**Deviations from plan:**
- Tests use snowflake dialect instead of duckdb. The duckdb keyword map is missing `"join"` and `"by"` entries, so JOIN/GROUP BY/ORDER BY tests would fail with duckdb. This is a pre-existing lexer limitation.

**Code review fixes applied:**
- Added ClauseOn assertion to JOIN test case
- Strengthened subquery test to check exact column position of outer FROM