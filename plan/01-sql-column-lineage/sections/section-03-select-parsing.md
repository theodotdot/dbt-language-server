I have enough context now. Let me produce the section content.

# Section 03: SELECT List Parsing

## Overview

This section implements SELECT list parsing within the clause state machine built in section-02. When the parser encounters a `SELECT` keyword at `parenDepth == 0`, it enters `ClauseSelect` context and collects `SelectItem` entries until hitting a clause boundary keyword (FROM, WHERE, etc.) or EOF.

**Dependencies**: Sections 01 (data types: `SelectItem`, `ClauseKind`) and 02 (clause state machine: `parenDepth`, `clauseStack`, `scopeStack`).

**Blocks**: Sections 06 (resolution engine uses SelectItems) and 07 (state integration).

---

## Key Design Decisions

### Paren-Depth-Aware Alias Detection

Alias detection ONLY applies at `parenDepth == 0`. Inside parentheses (function calls, CASE expressions, subqueries), identifiers are never treated as aliases. This prevents `COALESCE(a, b)` from producing `a` and `b` as separate select items.

At `parenDepth == 0`:
1. If `AS` keyword is followed by `IDENT`, the IDENT is an explicit alias.
2. If no `AS`, and the last token before a COMMA or clause boundary is an `IDENT` not preceded by `DOT`, it is treated as an implicit alias. False positives are acceptable — this is for completion, where extra column names do not hurt.

### Subquery SELECTs

When `SELECT` is encountered at `parenDepth > 0`, this is a subquery (e.g., `WHERE x IN (SELECT ...)`). Do NOT push a new scope. Skip all tokens until the matching `)` brings paren depth back down.

### Star Handling

- Bare `*` at `parenDepth == 0` produces `SelectItem{IsStar: true, StarSource: ""}`.
- Qualified `table.*` produces `SelectItem{IsStar: true, StarSource: "table"}`.
- `*` inside a function call like `COUNT(*)` is at `parenDepth > 0` and is NOT treated as a SELECT star.

### Qualified Columns

`SELECT o.id FROM orders o` produces `SelectItem{Source: "o", Alias: "id"}`. The token before DOT is captured as the source qualifier.

---

## Tests

All tests go in `/home/theotime.poulain/dbt-language-server/analysis/parser/scope_test.go`.

Tests use table-driven format. Each test case parses SQL input via `Parse(input, docs.Dialect("duckdb"))`, calls `CreateQueryScope()`, and asserts properties on the resulting `QueryScope.SelectItems` slice.

### Test Cases for parseSelectItems

- **Simple SELECT list**: `SELECT id, name FROM t` produces 2 SelectItems with `Alias` values `"id"` and `"name"`.

- **Aliased column (explicit AS)**: `SELECT id AS order_id FROM t` produces 1 SelectItem with `Alias: "order_id"`.

- **Implicit alias**: `SELECT id order_id FROM t` produces 1 SelectItem with `Alias: "order_id"`.

- **SELECT star**: `SELECT * FROM t` produces 1 SelectItem with `IsStar: true`, `StarSource: ""`.

- **Qualified star**: `SELECT o.* FROM orders o` produces 1 SelectItem with `IsStar: true`, `StarSource: "o"`.

- **Qualified column**: `SELECT o.id FROM orders o` produces 1 SelectItem with `Source: "o"`, `Alias: "id"`.

- **Function call not treated as alias**: `SELECT COALESCE(a, b) AS c FROM t` produces exactly 1 SelectItem with `Alias: "c"`. Tokens `a` and `b` inside parens are NOT separate items.

- **COUNT star**: `SELECT COUNT(*) AS total FROM t` produces 1 SelectItem with `Alias: "total"`, and `IsStar` is false (the `*` is inside a function call at `parenDepth > 0`).

- **CASE expression**: `SELECT CASE WHEN x THEN y END AS z FROM t` produces 1 SelectItem with `Alias: "z"`.

- **Nested parens in function**: `SELECT TRIM(UPPER(name)) AS clean_name FROM t` produces 1 SelectItem with `Alias: "clean_name"`.

- **Subquery SELECT skipped**: `SELECT id FROM t WHERE x IN (SELECT id FROM other)` produces 1 SelectItem (`id`) on the outer scope. The inner `SELECT` does not create additional SelectItems on the parent scope.

- **Multiple expressions**: `SELECT a + b AS sum, c * d AS product FROM t` produces 2 SelectItems with aliases `"sum"` and `"product"`.

- **DISTINCT keyword**: `SELECT DISTINCT id, name FROM t` produces 2 SelectItems. The `DISTINCT` keyword is skipped, not treated as a column.

### Test Skeleton

```go
func TestParseSelectItems(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected []SelectItem // check Alias, IsStar, StarSource, Source fields
    }{
        // ... test cases listed above ...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p := Parse(tt.input, docs.Dialect("duckdb"))
            scope := p.CreateQueryScope()
            if scope == nil {
                t.Fatal("expected non-nil scope")
            }
            if len(scope.SelectItems) != len(tt.expected) {
                t.Fatalf("expected %d SelectItems, got %d", len(tt.expected), len(scope.SelectItems))
            }
            for i, exp := range tt.expected {
                got := scope.SelectItems[i]
                // Compare relevant fields: Alias, IsStar, StarSource, Source
            }
        })
    }
}
```

---

## Implementation Details

### File: `/home/theotime.poulain/dbt-language-server/analysis/parser/parser.go`

Extend the `parseTokens()` main loop. When clause state machine (from section-02) sets `clauseContext` to `ClauseSelect`, the parser begins collecting select items.

### SELECT Item Collection Strategy

The collection is **not** a separate function call that consumes tokens in a sub-loop. Instead, it works within the existing `parseTokens()` main loop by tracking state across iterations:

1. **Item accumulator state** on the Parser struct:
   - `selectItemStart Token` -- position of current item's first token
   - `selectItemTokens []Token` -- tokens accumulated for current item
   - `selectExprDepth int` -- alias for the existing `parenDepth` within select context

2. **On each token while `clauseContext == ClauseSelect`**:
   - `ASTERISK` at `parenDepth == 0`: If the previous token was `DOT` and the one before that was `IDENT`, create `SelectItem{IsStar: true, StarSource: prevIdent}`. Otherwise create `SelectItem{IsStar: true}`.
   - `COMMA` at `parenDepth == 0`: Finalize the current select item. Apply alias detection heuristic to accumulated tokens. Reset accumulator.
   - `AS` at `parenDepth == 0`: Mark that the next IDENT is an explicit alias.
   - `LPAREN`: Increment `parenDepth`. Continue accumulating.
   - `RPAREN`: Decrement `parenDepth`. Continue accumulating.
   - Clause boundary keyword at `parenDepth == 0` (FROM, WHERE, etc.): Finalize current item (same as COMMA logic), then transition clause context.
   - `DISTINCT` right after `SELECT`: Skip, do not treat as a column name.
   - Any other token at `parenDepth == 0`: Accumulate for alias detection.

3. **Alias detection on item finalization**:
   - If explicit `AS` was seen, the token after `AS` is the alias.
   - If no `AS`, check the last accumulated token at `parenDepth == 0`. If it is an `IDENT` not preceded by `DOT`, treat it as an implicit alias.
   - If the item is a single `IDENT`, both `Expression` and `Alias` are set to that identifier.
   - If the item is `qualifier.ident` (IDENT DOT IDENT), set `Source` to the qualifier and `Alias` to the ident.

### Handling SELECT Inside Parentheses

When `SELECT` is encountered and `parenDepth > 0` (subquery), skip tokens until paren depth returns to the level it was at when `SELECT` was encountered. This is tracked by recording `parenDepth` at the point `SELECT` is seen and consuming tokens until depth drops back.

### Integration with Clause State Machine

The clause state machine from section-02 handles transitioning out of `ClauseSelect` when it encounters `FROM`, `WHERE`, etc. at `parenDepth == 0`. The select item collection must finalize any in-progress item when this transition occurs.

### Token Types Relevant to SELECT Parsing

From `/home/theotime.poulain/dbt-language-server/analysis/parser/token.go`:
- `IDENT` -- column names, table qualifiers, aliases
- `ASTERISK` (`*`) -- star expressions
- `AS` -- explicit alias keyword
- `COMMA` (`,`) -- item separator
- `DOT` (`.`) -- qualifier separator
- `LPAREN` / `RPAREN` -- parentheses for functions, CASE, subqueries
- `SELECT` -- clause start (already a keyword in both dialect maps)
- `DISTINCT` -- modifier to skip
- `CASE`, `WHEN`, `THEN`, `ELSE`, `END` -- CASE expression tokens (opaque, just track paren depth)

### Edge Cases

- **Empty SELECT** (e.g., `SELECT FROM t`): No items collected. Scope has empty `SelectItems`.
- **Trailing comma** (e.g., `SELECT a, FROM t`): Last item is empty, discard it.
- **Jinja expression in SELECT list** (e.g., `SELECT {{ var('col') }} FROM t`): The Jinja tokens (`DB_LBRACE`, `VAR`, `DB_RBRACE`) are consumed by existing Jinja parsing. The SQL select accumulator sees gaps in its token stream. If a `VAR` token appears in select context, it can be accumulated as an opaque expression. The resulting SelectItem may lack a meaningful alias -- this is acceptable.
- **Window functions** (e.g., `SELECT ROW_NUMBER() OVER (PARTITION BY x ORDER BY y) AS rn`): The `OVER (...)` is inside parentheses, so `ORDER BY` inside does NOT trigger a clause transition. The `parenDepth` tracking handles this correctly.

---

## Implementation Notes

**Status:** Completed

**Files modified:**
- `analysis/parser/parser.go` — added selectTokens/selectHasAS/selectStarted fields, collectSelectToken(), finalizeSelectItem(), resetSelectAccumulator() methods. Integrated with parseTokens() via default case and setClause() hook.
- `analysis/parser/scope_test.go` — added 14 TestParseSelectItems test cases

**Deviations from plan:**
- Tests use snowflake dialect (same as section-02, duckdb keyword map limitation)
- Implementation uses in-loop token collection via default switch case + setClause() finalization hook, rather than separate function. Cleaner integration with existing parseTokens structure.
- ASTERISK handling distinguishes multiplication from star by checking accumulated token context (bare * or DOT-preceded = star, otherwise = operator)

**Code review fixes:**
- Added nested parens and window function test cases
- Removed dead code in implicit alias branch