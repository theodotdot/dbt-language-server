Now I have all the context needed. Let me generate the section content.

# Section 4: PrepareRename Handler

## Overview

Implement `State.PrepareRename()` in a new file `analysis/rename.go`. This method determines whether the cursor is on a renameable entity (REF token in SQL, model name in schema.yml, or column name in schema.yml) and returns the entity's range plus its current name as a placeholder. If the position is not renameable, it returns a null result.

## Dependencies

- **section-01-lsp-types**: `PrepareRenameRequest`, `PrepareRenameResponse`, `PrepareRenameResult` types in `lsp/textdocument_rename.go`
- **section-02-column-position**: `Column.Position lsp.Position` field and its propagation from `ColumnProperties.Name.Position` during aggregation

Both must be completed before this section.

## Key Data Structures (from dependencies, for reference only)

The `PrepareRenameResponse` (defined in section-01) wraps a result containing a `Range` and a `Placeholder` string. A null/empty result means "not renameable."

The `Column` struct (modified in section-02) has a `Position lsp.Position` field giving the 0-indexed line/character of the column name in the schema.yml file.

The existing `Token` struct in `analysis/parser/token.go`:

```go
type Token struct {
    Type    TokenType
    Literal string
    Line    int
    Column  int
}
```

The existing `ModelDetails` struct in `analysis/agg_model_details.go`:

```go
type ModelDetails struct {
    URI         string
    ProjectName string
    Description string
    SchemaURI   string
    SchemaRange lsp.Range   // Start and End are both set to the same point (start of model name)
    Columns     []Column
}
```

## Tests

Write tests in `analysis/rename_test.go`. These tests validate the PrepareRename logic in isolation.

### Test: cursor on REF token in SQL returns range + placeholder

Set up a `State` with a document containing parsed tokens (via `parser.Parse`) that includes a `ref('orders')` call. Set up `ModelDetailMap` with an `"orders"` entry. Call `PrepareRename` with cursor positioned on the REF token. Assert the response contains a range covering the model name and `"orders"` as placeholder.

### Test: cursor on model name in schema.yml returns range + placeholder

Set up `ModelDetailMap` with a model whose `SchemaURI` matches the test URI and `SchemaRange.Start` is set to the model name position. Call `PrepareRename` with cursor on that position. Assert range spans `[Start.Character, Start.Character + len(modelName))` on `Start.Line`, and placeholder is the model name.

### Test: cursor on column name in schema.yml returns range + placeholder

Set up `ModelDetailMap` with a model whose `SchemaURI` matches and has a `Column` with a known `Position`. Call `PrepareRename` with cursor on that column position. Assert range and placeholder match the column name.

### Test: cursor on cross-project ref returns null

Set up tokens where a REF token is preceded by a PACKAGE token (i.e., `ref('pkg', 'orders')`). Call `PrepareRename` on the REF token. Assert the response result is empty/null.

### Test: cursor on non-renameable position returns null

Call `PrepareRename` with cursor on a SQL keyword, whitespace, or `source()` token. Assert null result.

### Test: cursor on REF for non-existent model returns null

Set up tokens with a REF token whose `Literal` is not in `ModelDetailMap`. Assert null result.

### Test: position matching computes end from start + len(name)

Verify that for a model name "my_model" at character 10, the returned range end character is 18 (10 + 8).

## Implementation

### File: `/home/theotime.poulain/dbt-language-server/analysis/rename.go`

Create this new file with the `PrepareRename` method on `State`.

### Method Signature

```go
func (s *State) PrepareRename(id int, uri string, position lsp.Position) lsp.PrepareRenameResponse
```

### Detection Logic

The method must check three cases in order. Build a default response with an empty/null result first, then fill it in if a renameable entity is found.

**Case 1 -- Cursor on a REF token in a SQL file:**

1. Look up the document in `s.Documents[uri]`
2. Call `doc.Tokens.FindTokenAtCursor(position.Line, position.Character)`
3. If the token type is `parser.REF`, the model name is `token.Literal` (not `token.Value` -- the Token struct uses `Literal`)
4. Verify the model exists: check `s.DbtContext.ModelDetailMap[token.Literal]` is present
5. Check this is NOT a cross-project ref: call `tokenLL.TokenLookbackMatch(parser.PACKAGE, 2)`. If a PACKAGE token is found, this is `ref('pkg', 'model')` -- return the default null response
6. Compute the range from the token's `Line`/`Column` fields: start is `{Line: token.Line, Character: token.Column}`, end is `{Line: token.Line, Character: token.Column + len(token.Literal)}`
7. Return the range and model name as placeholder

**Case 2 -- Cursor on a model name in schema.yml:**

1. Check if the URI ends with `.yml` (the project only scans `.yml` files, not `.yaml`)
2. Iterate all entries in `s.DbtContext.ModelDetailMap`. For each model whose `SchemaURI` matches the URI:
   - `SchemaRange.Start` gives the start position of the model name
   - Compute end character as `SchemaRange.Start.Character + len(modelName)`
   - Check cursor line equals `SchemaRange.Start.Line` AND cursor character is within `[SchemaRange.Start.Character, SchemaRange.Start.Character + len(modelName))`
3. If found, return the computed range and model name as placeholder

**Case 3 -- Cursor on a column name in schema.yml:**

1. Same `.yml` file check (already done if Case 2 didn't match)
2. For each model whose `SchemaURI` matches, iterate its `Columns` slice
3. For each column, compute end character as `col.Position.Character + len(col.Name)`
4. Check cursor line equals `col.Position.Line` AND cursor character is within `[col.Position.Character, col.Position.Character + len(col.Name))`
5. If found, return the computed range and column name as placeholder

**Otherwise:** return the default response (null result = not renameable).

### Name Validation

Add a package-level compiled regex for dbt identifier validation:

```go
var dbtIdentifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
```

This regex is used in `PrepareRename` only to validate that the found name is a valid identifier (it always should be, but serves as a safety check). The same regex is also used by the `Rename` handler (section-06) to validate `newName`.

### URI Matching Note

`ModelDetails.SchemaURI` stores raw filesystem paths without `file://` prefix. The `uri` parameter passed to `PrepareRename` comes from the dispatch in `main.go`. Check how existing handlers receive URIs -- in `state.go`, `OpenDocument` strips `file://` but other methods like `Hover` and `Definition` receive the URI as-is from the request params. The `uri` used for document lookup in `s.Documents` includes the `file://` prefix. For schema matching, strip `file://` from the incoming URI before comparing to `SchemaURI`, or compare with prefix awareness. Follow the pattern used by existing methods like `GoToSchema`.

Looking at existing code: `s.Documents[uri]` uses URIs with `file://` prefix (set in `OpenDocument`). But `ModelDetails.SchemaURI` is a raw path without prefix. So when comparing, strip `file://` from the incoming `uri` before matching against `SchemaURI`.

### Files to Create/Modify

| File | Action |
|------|--------|
| `/home/theotime.poulain/dbt-language-server/analysis/rename.go` | Create -- `PrepareRename` method, `dbtIdentifierRegex` |
| `/home/theotime.poulain/dbt-language-server/analysis/rename_test.go` | Create -- tests listed above |

## Implementation Notes (Post-Implementation)

### Files modified
- `analysis/rename.go` — replaced stub with full PrepareRename (3 detection cases) + dbtIdentifierRegex

### Files created
- `analysis/rename_test.go` — 7 test cases

### Test count: 7 (REF token, model in schema, column in schema, cross-project null, non-renameable null, non-existent model null, range width)