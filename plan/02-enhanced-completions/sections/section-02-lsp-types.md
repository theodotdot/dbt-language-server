Now I have everything I need to write the section.

# Section 02: LSP Types

## Overview

This section adds new LSP type definitions and modifies server capabilities required by dot-qualified completion (section-04) and code action (section-07). It has no dependencies on other sections and can be implemented in parallel with section-01 and section-06.

Three changes are needed:
1. Add `FilterText` and `TextEdit` fields to `CompletionItem`
2. Change `CompletionProvider` from `map[string]any` to typed `CompletionOptions` with trigger characters
3. Create `lsp/textdocument_codeaction.go` with CodeAction LSP types and add `CodeActionProvider` to `ServerCapabilities`

## Existing Types (Context)

The `lsp` package already has:
- `Range` (Start/End Position) in `/home/theotime.poulain/dbt-language-server/lsp/textdocument.go`
- `Position` (Line/Character) in the same file
- `CompletionItem` in `/home/theotime.poulain/dbt-language-server/lsp/textdocument_completion.go` with fields: Label, Detail, Documentation, Kind, InsertText, InsertTextFormat, SortText
- `CompletionOptions` struct with `TriggerCharacters []string` already defined in the same file but **not used** — `ServerCapabilities.CompletionProvider` is typed `map[string]any` and initialized as `map[string]any{}`

## Tests

Write tests before implementation. All tests go in the `lsp` package.

### File: `/home/theotime.poulain/dbt-language-server/lsp/textdocument_completion_test.go`

Tests for the new `CompletionItem` fields:

- **CompletionItem JSON serialization includes `filterText` when set**: marshal a `CompletionItem` with `FilterText: "o.id"`, verify the JSON output contains `"filterText":"o.id"`.
- **CompletionItem JSON omits `filterText` when empty**: marshal a `CompletionItem` with `FilterText: ""`, verify the JSON output does not contain the key `filterText`.
- **CompletionItem JSON includes `textEdit` when set**: marshal a `CompletionItem` with a non-nil `TextEdit` containing a Range and NewText, verify the JSON output contains the `textEdit` object with correct nested structure.
- **CompletionItem JSON omits `textEdit` when nil**: marshal a `CompletionItem` with `TextEdit: nil`, verify the JSON output does not contain the key `textEdit`.

### File: `/home/theotime.poulain/dbt-language-server/lsp/textdocument_codeaction_test.go`

Tests for the new code action types:

- **CodeActionResponse JSON serialization matches LSP spec**: construct a `CodeActionResponse` with one `CodeAction` containing a `WorkspaceEdit` with one `TextEdit`. Marshal to JSON and verify it produces the expected structure with `title`, `kind`, `edit.changes` (map of URI to TextEdit array).
- **WorkspaceEdit with TextEdits serializes correctly**: construct a `WorkspaceEdit` with entries for two URIs, marshal to JSON, verify both URI keys appear with their respective TextEdit arrays.
- **Empty CodeAction list produces valid JSON array**: construct a `CodeActionResponse` with `Result: []CodeAction{}`, marshal, verify it produces `[]` (not `null`). This may require initializing the slice explicitly.

## Implementation

### 1. Add `FilterText` and `TextEdit` to `CompletionItem`

**File:** `/home/theotime.poulain/dbt-language-server/lsp/textdocument_completion.go`

Add a `TextEdit` struct and two new fields to `CompletionItem`:

```go
type TextEdit struct {
    Range   Range  `json:"range"`
    NewText string `json:"newText"`
}

type CompletionItem struct {
    Label            string    `json:"label"`
    Detail           string    `json:"detail"`
    Documentation    string    `json:"documentation"`
    Kind             int       `json:"kind"`
    InsertText       string    `json:"insertText"`
    InsertTextFormat int       `json:"insertTextFormat,omitempty"`
    SortText         string    `json:"sortText"`
    FilterText       string    `json:"filterText,omitempty"`
    TextEdit         *TextEdit `json:"textEdit,omitempty"`
}
```

Key points:
- `FilterText` uses `omitempty` so it is excluded from JSON when not set (most completion items do not use it).
- `TextEdit` is a pointer with `omitempty` so it is excluded when nil.
- `TextEdit` struct uses the existing `Range` type from `textdocument.go`.
- The `TextEdit` struct defined here will also be reused by the code action types.

### 2. Change `CompletionProvider` to Typed Struct

**File:** `/home/theotime.poulain/dbt-language-server/lsp/initialize.go`

Two changes:

**a)** Change the `CompletionProvider` field type in `ServerCapabilities` from `map[string]any` to `CompletionOptions`:

```go
type ServerCapabilities struct {
    TextDocumentSync       int                    `json:"textDocumentSync"`
    HoverProvider          bool                   `json:"hoverProvider"`
    DefinitionProvider     bool                   `json:"definitionProvider"`
    CompletionProvider     CompletionOptions      `json:"completionProvider"`
    ExecuteCommandProvider ExecuteCommandOptions  `json:"executeCommandProvider"`
    SemanticTokensProvider *SemanticTokensOptions `json:"semanticTokensProvider,omitempty"`
    CodeActionProvider     bool                   `json:"codeActionProvider"`
}
```

**b)** Update `NewInitializeResponse` to use the typed struct with the `.` trigger character, and set `CodeActionProvider: true`:

```go
CompletionProvider: CompletionOptions{
    TriggerCharacters: []string{"."},
},
CodeActionProvider: true,
```

The `CompletionOptions` struct already exists in `textdocument_completion.go` with the correct shape. It was just not being used.

### 3. Create Code Action LSP Types

**New file:** `/home/theotime.poulain/dbt-language-server/lsp/textdocument_codeaction.go`

Define these types:

- `CodeActionRequest` — wraps `Request` + `CodeActionParams`
- `CodeActionParams` — `TextDocument TextDocumentIdentifier`, `Range Range`, `Context CodeActionContext`
- `CodeActionContext` — `Diagnostics []Diagnostic` (can be empty struct or minimal; the server does not use diagnostics for code action triggers in this implementation)
- `CodeAction` — `Title string`, `Kind string`, `Edit *WorkspaceEdit`
- `WorkspaceEdit` — `Changes map[string][]TextEdit` (key is document URI)
- `CodeActionResponse` — wraps `Response` + `Result []CodeAction`

The `TextEdit` type is already defined in `textdocument_completion.go` (from step 1 above) and is reused here. Do not redefine it.

The `Kind` field on `CodeAction` uses string values per LSP spec. The relevant kind for SELECT * expansion is `"refactor"`.

Struct signatures (stubs are sufficient):

```go
type CodeActionParams struct {
    TextDocument TextDocumentIdentifier `json:"textDocument"`
    Range        Range                  `json:"range"`
    Context      CodeActionContext      `json:"context"`
}

type CodeActionContext struct {
    Diagnostics []Diagnostic `json:"diagnostics"`
}

type CodeAction struct {
    Title string         `json:"title"`
    Kind  string         `json:"kind"`
    Edit  *WorkspaceEdit `json:"edit,omitempty"`
}

type WorkspaceEdit struct {
    Changes map[string][]TextEdit `json:"changes"`
}

type CodeActionRequest struct {
    Request
    Params CodeActionParams `json:"params"`
}

type CodeActionResponse struct {
    Response
    Result []CodeAction `json:"result"`
}
```

Note: `Diagnostic` type may already exist in `textdocument_diagnostics.go`. If so, reuse it. If not, define a minimal stub (it is not used by the code action logic, only present for protocol conformance).

## Files Modified/Created

| File | Action |
|------|--------|
| `/home/theotime.poulain/dbt-language-server/lsp/textdocument_completion.go` | Add `TextEdit` struct, add `FilterText` and `TextEdit` fields to `CompletionItem` |
| `/home/theotime.poulain/dbt-language-server/lsp/initialize.go` | Change `CompletionProvider` field type to `CompletionOptions`, add `CodeActionProvider bool`, update `NewInitializeResponse` |
| `/home/theotime.poulain/dbt-language-server/lsp/textdocument_codeaction.go` | New file with CodeAction LSP types |
| `/home/theotime.poulain/dbt-language-server/lsp/textdocument_completion_test.go` | New file with CompletionItem serialization tests |
| `/home/theotime.poulain/dbt-language-server/lsp/textdocument_codeaction_test.go` | New file with CodeAction serialization tests |

## Downstream Dependents

- **section-04-dot-qualified** uses `FilterText` and `TextEdit` on `CompletionItem`, and relies on `.` being registered as a trigger character.
- **section-07-code-action** uses `CodeAction`, `WorkspaceEdit`, `TextEdit`, `CodeActionProvider`, and the request/response types.

## Implementation Notes (Post-Implementation)

All files matched plan exactly. Existing `Diagnostic` type reused from `textdocument_diagnostics.go`. No deviations.

### Test count: 7 test cases across 2 new test files
- `TestCompletionItemFilterText` (2 subtests)
- `TestCompletionItemTextEdit` (2 subtests)
- `TestCodeActionResponseSerialization` (3 subtests)