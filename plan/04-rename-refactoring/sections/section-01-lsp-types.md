Now I have all the context needed.

# Section 1: LSP Protocol Types

## Overview

This section defines all LSP protocol types needed for rename/refactoring support. These are pure data structures in a new file `lsp/textdocument_rename.go`, plus a minor addition to `lsp/message.go`. No tests are needed for this section -- types are verified implicitly by marshaling tests in later sections (section-07).

## Dependencies

- None. This is a leaf section with no prerequisites.
- **Blocked by this section:** section-03 (capability/dispatch), section-04 (prepareRename handler), section-06 (rename handler).

## Existing Types to Reuse

The following types already exist in `/home/theotime.poulain/dbt-language-server/lsp/textdocument.go` and must NOT be redefined:

- `Position` -- `{Line int, Character int}`
- `Range` -- `{Start Position, End Position}`
- `TextDocumentIdentifier` -- `{URI string}`
- `TextDocumentPositionParams` -- `{TextDocument TextDocumentIdentifier, Position Position}`
- `Location` -- `{URI string, Range Range}`

The base `Request` and `Response` structs are in `/home/theotime.poulain/dbt-language-server/lsp/message.go`:

- `Request` -- `{RPC string, ID int, Method string}`
- `Response` -- `{RPC string, ID *int}`

## File: `lsp/textdocument_rename.go` (NEW)

Create this file in `/home/theotime.poulain/dbt-language-server/lsp/textdocument_rename.go` with the following types. All types are in package `lsp`.

### PrepareRename Types

**`PrepareRenameRequest`** -- embeds `Request`, has `Params TextDocumentPositionParams`. Follows the exact pattern of `DefinitionRequest` in `textdocument_definition.go`.

**`PrepareRenameResponse`** -- embeds `Response`, has `Result *PrepareRenameResult` with `json:"result"` tag. The result pointer is nullable: nil means "not renameable" (serializes as `"result": null`).

**`PrepareRenameResult`** -- has `Range Range` and `Placeholder string`. These tell the editor the extent of the symbol and its current name.

### Rename Types

**`RenameRequest`** -- embeds `Request`, has `Params RenameParams`.

**`RenameParams`** -- embeds `TextDocumentPositionParams`, adds `NewName string` with json tag `"newName"`.

**`RenameResponse`** -- embeds `Response`, has `Result *WorkspaceEdit` with `json:"result,omitempty"` and `Error *ResponseError` with `json:"error,omitempty"`. This allows serializing either a successful result or an error. Using `omitempty` with pointer fields ensures only one of `result` or `error` appears in the JSON output.

### WorkspaceEdit and DocumentChanges

**`WorkspaceEdit`** -- has `DocumentChanges []DocumentChange` with json tag `"documentChanges"`. Uses `documentChanges` (not the simpler `changes` map) because model rename requires `RenameFile` operations.

**`DocumentChange`** -- a union type. Go has no native unions, so model as a struct with two pointer fields:

- `TextDocumentEditValue *TextDocumentEdit`
- `RenameFileValue *RenameFile`

Implement `json.Marshaler` interface: marshal whichever pointer is non-nil. Only one should be set at a time. The custom `MarshalJSON` method checks which field is non-nil and marshals that field's value directly (not wrapped in another object).

**`TextDocumentEdit`** -- has `TextDocument OptionalVersionedTextDocumentIdentifier` with json tag `"textDocument"`, and `Edits []TextEdit` with json tag `"edits"`.

**`OptionalVersionedTextDocumentIdentifier`** -- has `URI string` with json tag `"uri"` and `Version *int` with json tag `"version"` (pointer for nullable). This differs from the existing `VersionedTextDocumentIdentifier` which has a non-nullable int version.

**`TextEdit`** -- has `Range Range` with json tag `"range"` and `NewText string` with json tag `"newText"`.

**`RenameFile`** -- has `Kind string` with json tag `"kind"` (always set to `"rename"`), `OldURI string` with json tag `"oldUri"`, and `NewURI string` with json tag `"newUri"`.

### ResponseError

**`ResponseError`** -- has `Code int` with json tag `"code"` and `Message string` with json tag `"message"`. Use code `-32602` (InvalidParams) for conflicts and invalid names. Define a constant for this:

```go
const ErrCodeInvalidParams = -32602
```

## Custom JSON Marshaling Detail

The `DocumentChange.MarshalJSON()` method is critical for correct LSP wire format. The `documentChanges` array must contain heterogeneous entries -- some are `TextDocumentEdit` objects (with `textDocument` and `edits` fields), others are `RenameFile` objects (with `kind`, `oldUri`, `newUri` fields). They appear as bare objects in the array, not wrapped in a discriminator.

The implementation should:
1. Check if `RenameFileValue` is non-nil, marshal it with `json.Marshal(*d.RenameFileValue)`
2. Otherwise check if `TextDocumentEditValue` is non-nil, marshal it with `json.Marshal(*d.TextDocumentEditValue)`
3. If both are nil, return `json.Marshal(nil)`

## No Changes to `lsp/message.go`

The existing `Response` struct does not need modification. The `RenameResponse` struct defined above handles the error case independently by having its own `Error *ResponseError` field alongside the embedded `Response`. This avoids touching shared infrastructure.

## Summary of Types

| Type | Purpose |
|------|---------|
| `PrepareRenameRequest` | Incoming request for prepareRename |
| `PrepareRenameResponse` | Response with range + placeholder or null |
| `PrepareRenameResult` | Range and placeholder string |
| `RenameRequest` | Incoming request for rename |
| `RenameParams` | Position + newName |
| `RenameResponse` | Result (WorkspaceEdit) or Error |
| `WorkspaceEdit` | Container for documentChanges array |
| `DocumentChange` | Union: TextDocumentEdit or RenameFile |
| `TextDocumentEdit` | Text edits within a single file |
| `OptionalVersionedTextDocumentIdentifier` | URI + nullable version |
| `TextEdit` | Range + new text |
| `RenameFile` | File rename operation (old/new URI) |
| `ResponseError` | Error code + message |

## Implementation Notes (Post-Implementation)

Deviations from plan:
- `TextEdit` not redefined — reused from `textdocument_completion.go`
- `WorkspaceEdit` extended in `textdocument_codeaction.go` rather than redefined — added `DocumentChanges` field with `omitempty`, also added `omitempty` to existing `Changes` field
- No new file `lsp/message.go` changes needed (plan already noted this)

### Files modified/created
- `lsp/textdocument_rename.go` — NEW (12 types + 1 constant + DocumentChange.MarshalJSON)
- `lsp/textdocument_codeaction.go` — MODIFIED (WorkspaceEdit extended)

### Test count: 0 (deferred to section-07 per plan)