Now I have enough context. Let me produce the section content.

# Section 07: Code Action (SELECT * Expansion)

## Overview

This section implements `textDocument/codeAction` support for expanding `SELECT *` into an explicit column list. It requires:

1. New LSP types for code actions in `lsp/textdocument_codeaction.go`
2. A new handler in `analysis/code_action.go`
3. Routing in `main.go`
4. `CodeActionProvider` in `ServerCapabilities`

## Dependencies

- **section-02-lsp-types**: Provides `CodeActionProvider` field on `ServerCapabilities`, the `TextEdit` struct, and the code action LSP types file. If section-02 has already created `lsp/textdocument_codeaction.go` with the types, this section only needs to implement the handler logic. If not, this section must create those types.
- **Split 01 (column lineage engine)**: `ResolveColumnsAtPosition`, `QueryScope`, `ScopeColumn`, token types like `MUL`, `DOT`, `IDENT` from the parser. The code action uses scope resolution to determine which columns to expand `*` into.

## Tests

All tests use Go standard `testing` package with table-driven patterns.

### LSP Type Tests (if not covered by section-02)

File: `/home/theotime.poulain/dbt-language-server/lsp/textdocument_codeaction_test.go`

- `CodeActionResponse` JSON serialization matches LSP spec structure
- `WorkspaceEdit` with `TextEdit` entries serializes correctly (map of URI to `[]TextEdit`)
- Empty `CodeAction` list serializes to valid JSON array `[]`

### Code Action Handler Tests

File: `/home/theotime.poulain/dbt-language-server/analysis/code_action_test.go` (new file)

Table-driven tests with SQL content, cursor position, and expected code action results:

- Cursor on `*` in `SELECT * FROM {{ ref('orders') }}` with resolvable columns: offers "Expand SELECT *" action with `TextEdit` replacing `*` with comma-separated column list
- Cursor on `*` with no resolvable columns (e.g., ref'd model has no columns in `ModelDetailMap`): returns empty code action list
- Cursor NOT on `*` (e.g., on a column name): returns empty code action list
- Qualified star `o.*` where `o` is a known alias: expands to only columns from alias `o`, `TextEdit` range covers `o.*` (the full `ident.` + `*` span)
- Qualified star detection: token before `MUL` is `DOT` preceded by `IDENT` — correctly identifies as qualified star

### Routing Test

File: `/home/theotime.poulain/dbt-language-server/analysis/state_lsp_test.go`

- `textDocument/codeAction` request routes to handler and returns valid response (integration-level, tested via `handleMessage` or by calling `state.TextDocumentCodeAction` directly)

## Implementation

### 1. LSP Types

File: `/home/theotime.poulain/dbt-language-server/lsp/textdocument_codeaction.go` (new file)

Section-02 may have already created this file. If so, verify it contains the types below; otherwise create them.

Required types:

- **`CodeActionRequest`**: wraps `Request` with `CodeActionParams`
- **`CodeActionParams`**: `TextDocument TextDocumentIdentifier`, `Range Range`, `Context CodeActionContext`
- **`CodeActionContext`**: `Diagnostics []any` (unused for now, but required by LSP spec)
- **`CodeAction`**: `Title string`, `Kind string`, `Edit *WorkspaceEdit`
- **`WorkspaceEdit`**: `Changes map[string][]TextEdit` (key is document URI)
- **`TextEdit`**: `Range Range`, `NewText string` — check if this already exists from section-02's `CompletionItem.TextEdit` work. Reuse if so.
- **`CodeActionResponse`**: wraps `Response` with `Result []CodeAction`

All fields use `json` struct tags matching the LSP spec camelCase naming.

### 2. ServerCapabilities Update

File: `/home/theotime.poulain/dbt-language-server/lsp/initialize.go`

Add `CodeActionProvider bool` field to `ServerCapabilities` with JSON tag `"codeActionProvider"`. Set to `true` in `NewInitializeResponse`.

Current `ServerCapabilities` struct (line 29-37):
```go
type ServerCapabilities struct {
    TextDocumentSync int `json:"textDocumentSync"`
    HoverProvider          bool                   `json:"hoverProvider"`
    DefinitionProvider     bool                   `json:"definitionProvider"`
    CompletionProvider     map[string]any         `json:"completionProvider"`
    ExecuteCommandProvider ExecuteCommandOptions  `json:"executeCommandProvider"`
    SemanticTokensProvider *SemanticTokensOptions `json:"semanticTokensProvider,omitempty"`
}
```

Add after `SemanticTokensProvider`:
```go
CodeActionProvider bool `json:"codeActionProvider"`
```

Note: section-02 may have already changed `CompletionProvider` from `map[string]any` to `CompletionOptions`. Either way, `CodeActionProvider` is additive.

### 3. Code Action Handler

File: `/home/theotime.poulain/dbt-language-server/analysis/code_action.go` (new file)

Method signature on `State`:

```go
func (s *State) TextDocumentCodeAction(id int, uri string, actionRange lsp.Range) lsp.CodeActionResponse
```

Logic:

1. Get the document from `s.Documents[uri]`
2. Get the document's tokens (from `Document.Tokens`, a `[][]parser.TokenWithLocation` — lines of tokens)
3. Iterate tokens on the lines within `actionRange` looking for `MUL` tokens
4. For each `MUL` token found in a SELECT context:
   a. Check if it's a qualified star: look at the previous token — if `DOT`, and the token before that is `IDENT`, extract the alias from that `IDENT` token's `Literal`
   b. Call `ResolveColumnsAtPosition` (from split 01) with the `*` token's position to get available `ScopeColumn` entries
   c. If qualified star, filter columns to only those matching the alias
   d. If no columns resolved, skip (don't offer action)
   e. Build replacement text: join column names with `,\n` plus indentation (spaces matching the `*` token's column position)
   f. Compute the `TextEdit` range: for plain `*`, use the `MUL` token's range; for qualified `ident.*`, extend the range start back to cover the `IDENT` and `DOT` tokens
   g. Create a `CodeAction` with `Title: "Expand SELECT *"`, `Kind: "refactor"`, and `Edit` containing a `WorkspaceEdit` with the computed `TextEdit`
5. Return all collected code actions (could be multiple if multiple `*` tokens exist)

SELECT context verification: use `ClauseRanges` from `QueryScope` (split 01) to confirm the `MUL` token is within a SELECT clause, not in a `WHERE x * y` multiplication context. If `ClauseRanges` is unavailable, a simpler heuristic is to walk backward from the `MUL` token and check that the nearest clause keyword is `SELECT`.

### 4. Method Routing

File: `/home/theotime.poulain/dbt-language-server/main.go`

Add a new case in the `handleMessage` switch (around line 85), after the `textDocument/completion` case:

```go
case "textDocument/codeAction":
    var request lsp.CodeActionRequest
    if err := json.Unmarshal(contents, &request); err != nil {
        logger.Printf("textDocument/codeAction: %s", err)
        return
    }
    response := state.TextDocumentCodeAction(request.ID, request.Params.TextDocument.URI, request.Params.Range)
    util.WriteResponse(writer, response)
```

### 5. Edge Cases

- **No columns available**: return empty `[]CodeAction` (valid LSP response)
- **Multiple `*` in query**: each gets its own code action
- **Indentation**: derive from the `*` token's character offset — use that many spaces for continuation lines after the first column
- **Document not found**: return empty response (same pattern as other handlers)

## File Summary

| File | Action |
|------|--------|
| `lsp/textdocument_codeaction.go` | New — LSP types (may exist from section-02) |
| `lsp/textdocument_codeaction_test.go` | New — serialization tests |
| `lsp/initialize.go` | Modify — add `CodeActionProvider` to capabilities |
| `analysis/code_action.go` | New — handler implementation |
| `analysis/code_action_test.go` | New — handler tests |
| `main.go` | Modify — add `textDocument/codeAction` route |

## Implementation Notes (Post-Implementation)

### Deviations from plan
- Used `QueryScope.SelectItems` (IsStar, StarSource, Token) instead of manual token scanning for ASTERISK. Cleaner; the parser already tracks star context.
- Token type is `ASTERISK` not `MUL` as plan stated.
- LSP types, CodeActionProvider, and routing types already existed from section-02. Only handler + routing + tests were needed.
- Added range filtering from code review: handler respects `actionRange` parameter to only return actions for stars within the requested LSP range.
- Added defensive bounds check for negative startChar on qualified stars.

### Files modified/created
- `analysis/code_action.go` — NEW (handler using SelectItems iteration, range filtering, column dedup)
- `analysis/code_action_test.go` — NEW (9 subtests: bare star, no columns, cursor not on star, qualified star, doc not found, nil scope, multiline indent, range filter, full range multiple stars)
- `main.go` — MODIFIED (added textDocument/codeAction routing)

### Test count: 9 subtests in TestTextDocumentCodeAction