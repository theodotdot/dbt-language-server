I now have enough context. Here is the section content:

# Section 01: Data Types

## Overview

This section defines all new types needed by the SQL column lineage engine in a new file `analysis/parser/scope.go`, plus the corresponding unit tests in `analysis/parser/scope_test.go`. These types are foundational -- every subsequent section depends on them.

**No dependencies.** This section can be implemented first.

**Files to create:**
- `/home/theotime.poulain/dbt-language-server/analysis/parser/scope.go`
- `/home/theotime.poulain/dbt-language-server/analysis/parser/scope_test.go`

**No existing files are modified** in this section.

---

## Tests First

File: `/home/theotime.poulain/dbt-language-server/analysis/parser/scope_test.go`

Package: `parser`

All tests are table-driven using Go standard `testing` package. The tests cover type construction and the `FindSourceByAlias` lookup method on `QueryScope`.

### Test cases for FindSourceByAlias

1. **Returns correct SourceRef when alias matches** -- scope has a source with `Alias: "o"`, calling `FindSourceByAlias("o")` returns that source.

2. **Falls back to name match when no alias match** -- scope has a source with `Name: "orders"` and no alias. `FindSourceByAlias("orders")` returns it.

3. **Returns nil when no match** -- scope has sources, but none match the given alias/name. Returns nil.

4. **Handles self-join (same name, different aliases)** -- scope has two sources both with `Name: "events"` but aliases `"e1"` and `"e2"`. `FindSourceByAlias("e1")` returns the first, `FindSourceByAlias("e2")` returns the second.

5. **Empty Sources/CTEs returns zero values gracefully** -- a `QueryScope` with nil/empty `Sources` and `CTEs` does not panic on `FindSourceByAlias`, returns nil.

### Test structure

```go
func TestFindSourceByAlias(t *testing.T) {
    tests := []struct {
        name     string
        sources  []*SourceRef
        alias    string
        wantName string // expected SourceRef.Name, or "" if nil expected
    }{
        // ... table entries for the 5 cases above
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            scope := &QueryScope{Sources: tt.sources}
            got := scope.FindSourceByAlias(tt.alias)
            // assert got matches expectations
        })
    }
}
```

---

## Types to Define

All types go in `/home/theotime.poulain/dbt-language-server/analysis/parser/scope.go`, package `parser`.

### ClauseKind

```go
type ClauseKind int

const (
    ClauseNone ClauseKind = iota
    ClauseWith
    ClauseSelect
    ClauseFrom
    ClauseJoin
    ClauseWhere
    ClauseGroupBy
    ClauseOrderBy
    ClauseHaving
    ClauseOn
)
```

`ClauseKind` is parser state, NOT a token type. It is not added to `token.go`. It lives in `scope.go` because it is part of the scope/clause tracking system.

### SourceKind

```go
type SourceKind int

const (
    SourceKindTable SourceKind = iota
    SourceKindRef
    SourceKindSource
    SourceKindCTE
)
```

### SourceRef

```go
type SourceRef struct {
    Kind       SourceKind
    Name       string     // model name, source table name, CTE name, or raw table
    SourceName string     // only for SourceKindSource: first arg of source('src', 'table')
    Alias      string     // table alias if any
    Token      Token      // position of the source reference
}
```

### CTEDef

```go
type CTEDef struct {
    Name    string
    Scope   *QueryScope  // the CTE's own query scope
    Columns []string     // resolved output column names; may contain "*" for unresolved stars
    Token   Token        // position of CTE name
}
```

During parsing, `Columns` is populated from the CTE's SELECT list. Simple names/aliases stored directly. `SELECT *` produces a `"*"` entry. At resolution time (section 06), `"*"` entries are lazily expanded using the CTE's scope sources and `ModelDetailMap`.

### SelectItem

```go
type SelectItem struct {
    Expression string  // raw expression text
    Alias      string  // AS alias, or column name if simple reference
    Source     string  // source table/alias if resolvable (e.g., "o" from "o.id")
    IsStar     bool    // true for SELECT * or table.*
    StarSource string  // table alias/name for table.* (empty for bare *)
    Token      Token   // position
}
```

### ScopeColumn (resolution output)

```go
type ScopeColumn struct {
    Name        string  // column name
    Source      string  // table/CTE/ref name it comes from
    Qualified   string  // "alias.column" form
    Description string  // from schema.yml if available
}
```

This type is consumed by LSP handlers (completion, diagnostics, rename) via the resolution engine (section 06).

### QueryScope

```go
type QueryScope struct {
    Sources      []*SourceRef
    CTEs         map[string]*CTEDef
    SelectItems  []SelectItem
    ClauseRanges map[ClauseKind]Token  // start position of each SQL clause
    Parent       *QueryScope           // nil for top-level query
}
```

Key design decisions:
- **Sources is a slice, not a map**, to support self-joins where the same model appears multiple times with different aliases.
- **ClauseRanges** maps each clause kind to the `Token` marking its start position. The resolution engine (section 06) uses this to determine which clause the cursor is in.
- **Parent** links CTE scopes back to the top-level scope. Nil for the outermost query.

### Constructor

```go
func NewQueryScope(parent *QueryScope) *QueryScope
```

Initializes `CTEs` map and `ClauseRanges` map to avoid nil map panics. Sets `Parent` to the provided value. Returns a ready-to-use scope.

### FindSourceByAlias method

```go
func (qs *QueryScope) FindSourceByAlias(alias string) *SourceRef
```

Search algorithm:
1. Iterate `qs.Sources`. If any source has `Alias == alias`, return it (first match).
2. If no alias match, iterate again checking `Name == alias`. Return first match.
3. Return nil if nothing matches.

This two-pass approach ensures explicit aliases take priority over implicit name matches. The method must handle nil/empty `Sources` without panicking.

---

## Relationship to Existing Code

- The `Token` type used in these structs is the existing `parser.Token` from `/home/theotime.poulain/dbt-language-server/analysis/parser/token.go` (has `Type`, `Literal`, `Line`, `Column` fields).
- These types do NOT modify any existing file. They are purely additive.
- The `ScopeColumn` type references `ModelDetails` and `Source` from the `analysis` package only at resolution time (section 06), not in this section's type definitions.
- The existing `CTE` struct in `parser.go` (with `Ind`, `ParenCount`, `Tokens`, `TokenNameMap`) is untouched. The new `CTEDef` type is separate and will be populated in parallel (section 05).

---

## Implementation Notes

**Status:** Completed

**Files created:**
- `analysis/parser/scope.go` — all type definitions, constructor, and FindSourceByAlias method
- `analysis/parser/scope_test.go` — 7 tests (2 for NewQueryScope, 5 for FindSourceByAlias)

**Deviations from plan:** None. Implementation is 1:1 with spec.

**Code review fix applied:** Added `wantAlias` assertion to self-join test case to verify correct SourceRef identity (not just name match).