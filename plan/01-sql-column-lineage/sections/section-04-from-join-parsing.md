# Section 04: FROM/JOIN Source Reference Parsing

## Overview

This section implements FROM and JOIN clause parsing to create `SourceRef` entries for each table source encountered. It intercepts `REF` and `SOURCE` token dispatch when the clause context is `ClauseFrom` or `ClauseJoin`, creates appropriately-typed `SourceRef` structs, detects table aliases, and handles edge cases like parenthesized subqueries and Jinja control flow producing multiple refs.

## Dependencies

- **section-01-data-types**: Provides `QueryScope`, `SourceRef`, `SourceKind*` constants, `ClauseKind` constants, and `FindSourceByAlias()`. These types must exist before this section can be implemented.
- **section-02-clause-state-machine**: Provides the `scopeStack`, `clauseStack`, `parenDepth` fields on `Parser`, and the main loop clause detection logic that sets `clauseContext` to `ClauseFrom`/`ClauseJoin`. This section adds behavior that triggers when those clause contexts are active.

## Files to Modify

- `/home/theotime.poulain/dbt-language-server/analysis/parser/parser.go` -- extend `parseTokens()` main loop and add `parseFromSources()` helper
- `/home/theotime.poulain/dbt-language-server/analysis/parser/scope_test.go` -- add FROM/JOIN parsing tests

## Tests

All tests go in `/home/theotime.poulain/dbt-language-server/analysis/parser/scope_test.go`. They are table-driven, parsing SQL input via `parser.Parse()` then calling `CreateQueryScope()` and asserting on the resulting `QueryScope.Sources` slice.

### Test Cases for parseFromSources

Each test case calls `Parse(input, "duckdb")`, gets the scope via `CreateQueryScope()`, and checks `scope.Sources`.

```go
func TestParseFromSources(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected []SourceRef // check Kind, Name, SourceName, Alias fields
    }{
        // cases listed below
    }
    // ...
}
```

**Simple table**: `SELECT id FROM orders` -- 1 SourceRef with Kind=SourceKindTable, Name="orders", Alias=""

**Aliased table (implicit)**: `SELECT id FROM orders o` -- SourceRef with Alias="o"

**Aliased table (explicit AS)**: `SELECT id FROM orders AS o` -- SourceRef with Alias="o"

**ref() in FROM**: `SELECT * FROM {{ ref('orders') }}` -- SourceRef with Kind=SourceKindRef, Name="orders"

**ref() with alias**: `SELECT * FROM {{ ref('orders') }} o` -- SourceRef with Kind=SourceKindRef, Name="orders", Alias="o"

**source() in FROM**: `SELECT * FROM {{ source('stripe', 'payments') }}` -- SourceRef with Kind=SourceKindSource, Name="payments", SourceName="stripe"

**JOIN produces two sources**: `SELECT * FROM {{ ref('orders') }} o JOIN {{ ref('customers') }} c ON o.customer_id = c.id` -- 2 SourceRefs, first has Name="orders" Alias="o", second has Name="customers" Alias="c"

**Multiple JOIN types**: LEFT JOIN, RIGHT JOIN, INNER JOIN, CROSS JOIN all create SourceRefs. Test with `LEFT JOIN {{ ref('x') }} a ... RIGHT JOIN {{ ref('y') }} b` and verify 3 total SourceRefs (1 FROM + 2 JOINs).

**CTE reference in FROM**: `WITH cte AS (SELECT 1) SELECT * FROM cte` -- SourceRef with Kind=SourceKindCTE, Name="cte" (CTE name matched against scope.CTEs map; CTE handling itself is section-05, but the FROM parser must check the CTEs map to set the Kind correctly)

**Self-join**: `SELECT * FROM {{ ref('events') }} e1 JOIN {{ ref('events') }} e2 ON e1.id = e2.id` -- 2 SourceRefs, both Name="events", aliases "e1" and "e2" respectively

**Parenthesized subquery in FROM**: `SELECT * FROM (SELECT id FROM t) sub` -- no SourceRef created for the subquery (skipped). The parser increments parenDepth and skips until matching `)`.

### Test Cases for Jinja + SQL Interaction

**Jinja if/else in FROM**: Input with `{% if %}` producing two `{{ ref() }}` calls in FROM clause -- both should appear as SourceRefs. Example:
```sql
SELECT *
FROM
{% if true %}
  {{ ref('orders_prod') }}
{% else %}
  {{ ref('orders_dev') }}
{% endif %}
```
Expected: 2 SourceRefs, Kind=SourceKindRef, names "orders_prod" and "orders_dev".

**{% set %} between SELECT and FROM**: A `{% set %}` block between clauses should not disrupt clause tracking. The FROM clause after the block should still produce SourceRefs.

**Jinja expression in SELECT list**: `SELECT {{ var('col_name') }} FROM t` -- scope should still have 1 SourceRef for "t" from the FROM clause.

## Implementation Details

### How Token Dispatch Works (Critical Background)

The existing `parseTokens()` main loop dispatches on `p.curTok.Type`. The lexer's `LookupIdent()` classifies the literal `ref` as token type `REF` and `source` as `SOURCE` before the main loop sees them. So when the parser encounters `{{ ref('orders') }}`:

1. `DB_LBRACE` (`{{`) is hit first -- currently the `case DB_LBRACE:` only handles CONFIG and IDENT peekTok types
2. But `ref` is classified as `REF` by `LookupIdent()`, so peekTok.Type is `REF`, not `IDENT`
3. The main loop advances and hits `case REF:` which calls `parseRef()`
4. `parseRef()` reclassifies the model name token (e.g., "orders") as type `REF`

This means FROM/JOIN source detection must happen **inside or immediately after** the existing `case REF:` and `case SOURCE:` handlers in `parseTokens()`, not in a separate FROM-parsing function that tries to consume tokens independently.

### Strategy: Intercept in Main Loop

After section-02 adds clause context tracking, the `case REF:` and `case SOURCE:` blocks in `parseTokens()` gain additional logic:

**For `case REF:`** (after `p.parseRef()` runs):
- Check if `clauseContext` (top of clauseStack) is `ClauseFrom` or `ClauseJoin`
- If yes, find the REF-typed token in the recently parsed tokens (it is `p.curTok` after `parseRef()` sets `p.curTok.Type = REF`)
- Create `SourceRef{Kind: SourceKindRef, Name: p.curTok.Literal, Token: p.curTok}`
- Check following tokens for alias (see alias detection below)
- Append to current scope's Sources

**For `case SOURCE:`** (after `p.parseSource()` runs):
- Check if clauseContext is `ClauseFrom` or `ClauseJoin`
- If yes, walk the recently parsed tokens to find the `SOURCE` and `SOURCE_TABLE` typed tokens
- Create `SourceRef{Kind: SourceKindSource, Name: <SOURCE_TABLE literal>, SourceName: <SOURCE literal>}`
- Check following tokens for alias
- Append to current scope's Sources

**For plain `IDENT` in FROM/JOIN context**:
- When clauseContext is `ClauseFrom` or `ClauseJoin` and an `IDENT` token is encountered that is not a keyword
- Check if the name matches a CTE in the current scope's CTEs map: if yes, Kind=SourceKindCTE; otherwise Kind=SourceKindTable
- Check following token for alias
- Append to current scope's Sources

### Alias Detection for Table References

After consuming a table source (ref, source, or plain IDENT), look at the next non-Jinja token to detect aliases. This must happen at `parenDepth == 0`:

1. If next token is `AS` and the one after is `IDENT`: explicit alias. Set `SourceRef.Alias` to that IDENT's literal.
2. If next token is `IDENT` and it is NOT a SQL keyword (not `ON`, `JOIN`, `LEFT`, `RIGHT`, `INNER`, `FULL`, `CROSS`, `WHERE`, `GROUP`, `ORDER`, `HAVING`, `LIMIT`, `UNION`, `SEMICOLON`, etc.): implicit alias. Set `SourceRef.Alias` to that IDENT's literal.
3. Otherwise: no alias.

The alias-checking logic should use `peekTok` to look ahead without consuming tokens unnecessarily, or consume and handle appropriately within the main loop flow.

### Parenthesized Subquery in FROM

When `LPAREN` is encountered in FROM/JOIN context at the current scope's parenDepth == 0 (meaning the opening paren is not inside an expression), this is a subquery or grouped expression. The parser should:
- Increment parenDepth
- Skip all tokens until the matching `)` brings parenDepth back to 0
- Do NOT create a SourceRef for the subquery content
- After the closing `)`, optionally check for an alias (subquery alias like `FROM (SELECT ...) sub`)

### Jinja Control Flow Producing Multiple Refs

When `{% if %}` / `{% else %}` / `{% endif %}` produce multiple `{{ ref() }}` calls within a single FROM clause, each `case REF:` dispatch creates a separate SourceRef. No special handling is needed -- the existing main loop processes each REF token sequentially, and the clause context remains `ClauseFrom` throughout the Jinja block boundaries (since `parseJinjaBlock()` does not change clause context).

### JOIN Keyword Variants

The clause state machine (section-02) must recognize these patterns as setting `ClauseJoin`:
- `JOIN` alone
- `LEFT JOIN`, `RIGHT JOIN`, `INNER JOIN`, `CROSS JOIN`, `FULL JOIN`
- `LEFT OUTER JOIN`, `RIGHT OUTER JOIN`, `FULL OUTER JOIN`
- `NATURAL JOIN`

The key token is `JOIN` -- the preceding modifier tokens (`LEFT`, `RIGHT`, etc.) are already recognized as keywords. The clause state machine sets context to `ClauseJoin` when it sees the `JOIN` token at parenDepth == 0. The FROM source parser then handles the next table reference the same way as in FROM context.

### Current Scope Access

The "current scope" is the top of the `scopeStack` on the Parser. A helper like `p.currentScope()` (returning `p.scopeStack[len(p.scopeStack)-1]`) provides access. New SourceRefs are appended to `p.currentScope().Sources`.

### Edge Case: ON Clause

After a JOIN source and its alias, the `ON` keyword typically follows. When `ON` is encountered, the clause context should switch to `ClauseOn` (section-02 handles this). The FROM source parser stops looking for sources when it hits `ON`. After the ON condition (which runs until the next JOIN, WHERE, or other clause boundary), the clause context may switch back to `ClauseJoin` if another JOIN follows.

### Integration with Existing CTE Paren Counting

The existing `incParenCount()`/`decParenCount()` for CTE tracking in `parser.go` continues to work. The new `parenDepth` field (from section-02) is separate from `p.ctes.ParenCount`. Both track parentheses but for different purposes -- `p.ctes.ParenCount` tracks CTE body boundaries for the old CTE struct, while `parenDepth` tracks nesting for clause detection. They must both be updated on `LPAREN`/`RPAREN`.

## Summary Checklist

1. Write test cases in `scope_test.go` covering all scenarios listed above
2. Add FROM/JOIN source detection logic in `parseTokens()` main loop, intercepting `case REF:`, `case SOURCE:`, and adding IDENT handling for FROM/JOIN context
3. Implement alias detection for table references (explicit AS and implicit)
4. Handle parenthesized subqueries in FROM by skipping to matching `)`
5. Verify Jinja if/else produces multiple SourceRefs
6. Verify JOIN variants (LEFT, RIGHT, INNER, CROSS, FULL) all produce SourceRefs
7. Verify CTE names in FROM are tagged as SourceKindCTE
8. Ensure all existing parser tests still pass

---

## Implementation Notes

**Status:** Completed

**Files modified:**
- `analysis/parser/parser.go` — added lastSourceRef tracking, inFromJoinContext(), isSourceAliasKeyword(), addSourceRef(). Extended REF/SOURCE cases and default case in parseTokens() for source detection and alias assignment.
- `analysis/parser/scope_test.go` — added 11 TestParseFromSources test cases

**Deviations from plan:**
- Alias detection for ref/source uses deferred `lastSourceRef` pattern instead of immediate peekTok, because Jinja closing tokens (`'`, `)`, `}}`) separate the ref call from the alias.
- CTE-in-FROM test deferred to section-05 (CTEs map not yet populated)
- Subquery-in-FROM skipping works by design (parenDepth > 0 prevents clause transitions) but not explicitly tested

**Code review fixes:**
- Fixed critical comma-separated FROM bug
- Added LEFT/RIGHT JOIN multi-source test
- Made alias assertions unconditional