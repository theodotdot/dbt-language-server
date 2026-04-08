Now I have all the context needed. Let me generate the section content.

# Section 06: Rename Handler

## Overview

This section implements `State.Rename()` in `analysis/rename.go`. It handles the `textDocument/rename` request by performing conflict detection, constructing a `WorkspaceEdit` with `documentChanges` (containing `RenameFile`, schema `TextDocumentEdit`, and ref `TextDocumentEdit` entries for model rename, or a single schema `TextDocumentEdit` for column rename), and returning error responses when appropriate.

**Depends on:**
- Section 01 (LSP types: `RenameRequest`, `RenameResponse`, `WorkspaceEdit`, `DocumentChange`, `TextDocumentEdit`, `RenameFile`, `TextEdit`, `ResponseError` in `lsp/textdocument_rename.go`)
- Section 03 (dispatch wiring in `main.go` for `textDocument/rename`)
- Section 04 (`PrepareRename` detection logic reuse — same cursor-to-entity resolution)
- Section 05 (reference scanning: `scanProjectRefs()` or equivalent function that walks SQL files and returns grouped `TextDocumentEdit` entries)

**File to modify:** `/home/theotime.poulain/dbt-language-server/analysis/rename.go`

## Tests

Tests live in `/home/theotime.poulain/dbt-language-server/analysis/rename_test.go`. Write these before implementing.

### Validation Tests

- **Test: rename model to name that already exists** -- Set up `State` with `ModelDetailMap` containing both `"orders"` and `"customers"`. Call `Rename()` to rename `orders` to `customers`. Assert the response contains a `ResponseError` with code `-32602` and message containing `"already exists"`.

- **Test: rename column to name that already exists in same model** -- Set up a model with columns `["id", "name"]`. Call `Rename()` to rename column `id` to `name`. Assert `ResponseError`.

- **Test: rename with invalid identifier** -- Call `Rename()` with `newName` values like `"my model"` (spaces), `"1orders"` (leading digit), `"orders!"` (special chars), `""` (empty). Assert `ResponseError` for each.

- **Test: rename where newName equals oldName** -- Call `Rename()` with `newName == oldName`. Assert the response contains an empty `WorkspaceEdit` (no `documentChanges`), no error.

### Model Rename WorkspaceEdit Tests

- **Test: documentChanges includes RenameFile with correct URIs** -- Set up a model `"orders"` with `URI: "/project/models/orders.sql"`. Call `Rename()` with `newName: "purchases"`. Assert `documentChanges` contains a `RenameFile` with `OldURI: "file:///project/models/orders.sql"` and `NewURI: "file:///project/models/purchases.sql"`, `Kind: "rename"`.

- **Test: documentChanges includes TextDocumentEdit for schema.yml** -- Model has `SchemaURI: "/project/models/schema.yml"` and `SchemaRange.Start` at `{Line: 5, Character: 10}`, model name `"orders"` (6 chars). Assert a `TextDocumentEdit` targeting `"file:///project/models/schema.yml"` with a `TextEdit` at range `{5,10}-{5,16}` replacing with `"purchases"`.

- **Test: no schema TextDocumentEdit when SchemaURI is empty** -- Model has `SchemaURI: ""`. Assert no `TextDocumentEdit` for schema in `documentChanges`.

- **Test: new file path preserves directory and extension** -- Model at `/project/models/staging/orders.sql`. After rename to `purchases`, new path is `/project/models/staging/purchases.sql`.

- **Test: all URIs have file:// prefix** -- Iterate all `documentChanges` entries and assert every URI starts with `"file://"`.

### Column Rename WorkspaceEdit Tests

- **Test: single TextDocumentEdit for schema.yml column** -- Column `"order_id"` at position `{Line: 8, Character: 12}` in schema.yml. Rename to `"purchase_id"`. Assert one `TextDocumentEdit` with range `{8,12}-{8,20}` (8 chars for `order_id`) and `NewText: "purchase_id"`.

- **Test: no RenameFile or cross-file edits** -- Assert `documentChanges` has exactly one entry (the schema `TextDocumentEdit`), no `RenameFile`.

### JSON Serialization Tests

- **Test: ResponseError serializes correctly** -- Build a rename response with an error. Marshal to JSON. Assert output contains `"error": {"code": -32602, "message": "..."}` and no `"result"` field.

- **Test: successful rename serializes correctly** -- Build a rename response with a `WorkspaceEdit`. Marshal to JSON. Assert output contains `"result": {"documentChanges": [...]}`.

## Implementation Details

### Method Signature

```go
func (s *State) Rename(id int, uri string, position lsp.Position, newName string) lsp.RenameResponse
```

The return type `lsp.RenameResponse` must support both success and error cases. It should have the shape:

```go
type RenameResponse struct {
    Response
    Result *WorkspaceEdit  `json:"result,omitempty"`
    Error  *ResponseError  `json:"error,omitempty"`
}
```

Using `omitempty` with pointer fields ensures only one of `result` or `error` is serialized.

### Name Validation

Use the regex `^[a-zA-Z_][a-zA-Z0-9_]*$` to validate `newName`. If invalid or empty, return a `ResponseError` with code `-32602` and a descriptive message. Extract this into a helper (e.g., `isValidIdentifier(name string) bool`) since it is shared with `PrepareRename`.

### Entity Detection (reuse from PrepareRename)

The `Rename` handler must identify what entity the cursor is on, using the same logic as `PrepareRename` (Section 04). Recommended approach: extract the detection into a shared helper that returns a struct describing the entity:

```go
type renameTarget struct {
    kind     string // "model" or "column"
    name     string // current name
    // for column rename, also need:
    modelName string
}
```

Both `PrepareRename` and `Rename` call this helper. This avoids duplicating cursor-to-entity resolution.

### Conflict Detection

**Model rename:** Check `s.DbtContext.ModelDetailMap[newName]`. If it exists and `newName != oldName`, return `ResponseError{Code: -32602, Message: "model 'X' already exists"}`.

**Column rename:** Iterate `model.Columns` for the target model. If any column's `Name == newName` and `newName != oldName`, return `ResponseError`.

### No-op Check

If `newName == oldName`, return a successful response with an empty `WorkspaceEdit` (nil or empty `DocumentChanges` slice).

### Building the WorkspaceEdit for Model Rename

Construct a `[]DocumentChange` with up to three kinds of entries.

**1. RenameFile entry:**
```go
oldPath := model.URI
newPath := filepath.Dir(oldPath) + "/" + newName + filepath.Ext(oldPath)
```
Create a `DocumentChange` with a `RenameFile{Kind: "rename", OldURI: "file://" + oldPath, NewURI: "file://" + newPath}`.

**2. Schema.yml TextDocumentEdit (if SchemaURI is non-empty):**
Compute the edit range from `model.SchemaRange.Start`. The end character is `Start.Character + len(oldName)`. Create a `TextEdit{Range: computedRange, NewText: newName}` wrapped in a `TextDocumentEdit` targeting `"file://" + model.SchemaURI`.

**3. Ref TextDocumentEdits (from Section 05's scanning):**
Call the reference scanning function (Section 05) which returns `[]TextDocumentEdit` entries grouped by file URI. Append each as a `DocumentChange`.

### Building the WorkspaceEdit for Column Rename

Single `TextDocumentEdit` for the schema.yml file. The edit range starts at `column.Position` and ends at `{Line: column.Position.Line, Character: column.Position.Character + len(oldColumnName)}`. No `RenameFile`, no cross-file scanning.

### URI Prefixing

`ModelDetails.URI` and `ModelDetails.SchemaURI` store raw filesystem paths without `file://`. All URIs in `documentChanges` entries must be prefixed with `"file://"`. This matches the existing convention seen in `state.go` (e.g., `Definition` handler: `response.Result.URI = "file://" + model.URI`).

### Edge Cases

- **File not found on disk during ref scanning:** skip silently (handled by Section 05)
- **Parse error in SQL file:** skip that file (handled by Section 05)
- **Model has no SQL file (URI empty):** should not happen, but guard against it -- skip `RenameFile` if `model.URI == ""`
- **newName matches oldName:** return empty `WorkspaceEdit` (no-op), no error

### Dispatch Wiring (handled by Section 03, referenced here for context)

In `main.go`, the `textDocument/rename` case unmarshals `lsp.RenameRequest`, calls `state.Rename()`, and writes the response:

```go
case "textDocument/rename":
    var request lsp.RenameRequest
    if err := json.Unmarshal(contents, &request); err != nil {
        logger.Printf("textDocument/rename: %s", err)
        return
    }
    response := state.Rename(request.ID, request.Params.TextDocument.URI, request.Params.Position, request.Params.NewName)
    util.WriteResponse(writer, response)
```

## Implementation Notes (Post-Implementation)

### Files modified
- `analysis/rename.go` — replaced Rename stub with full implementation; refactored PrepareRename to use shared `resolveRenameTarget`

### Key structures added
- `renameTarget` — cursor-to-entity resolution result (kind, name, modelName)
- `resolveRenameTarget()` — shared helper for PrepareRename and Rename
- `nameRange()` — computes range for PrepareRename result
- `renameModel()` — builds WorkspaceEdit with RenameFile + schema edit + ref edits
- `renameColumn()` — builds WorkspaceEdit with single schema edit

### Deviations from plan
- **Shared `resolveRenameTarget` helper** — plan suggested this as "recommended approach"; implemented as described. PrepareRename refactored to use it (all existing tests still pass).
- **`nameRange` zero-range guard** — added `r.Start == r.End` check in PrepareRename to avoid degenerate highlight ranges on fallthrough.
- **`renameColumn` SchemaURI guard** — added early return for empty SchemaURI (defensive).
- **`nameRange` column disambiguation** — added `modelName == target.modelName` check to handle multiple models sharing a schema.yml.

### Test count: 12 (invalid identifier, model conflict, column conflict, noop, rename file, schema edit, no-schema, dir preservation, column schema edit, URI prefixes, success serialization, error serialization)