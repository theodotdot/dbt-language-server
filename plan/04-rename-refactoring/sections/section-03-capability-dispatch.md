Now I have all the context needed to write the section.

# Section 03: Capability Declaration and Message Dispatch

## Overview

This section adds rename capability advertisement to the LSP initialize response and wires up the two rename-related message handlers in the main dispatch loop. After this section, the server will declare rename support to editors and route incoming `textDocument/prepareRename` and `textDocument/rename` requests to handler methods on `State` (which are implemented in later sections).

**Depends on:** section-01-lsp-types (the `PrepareRenameRequest`, `PrepareRenameResponse`, `RenameRequest`, `RenameResponse` types must exist)

**Blocks:** section-06-rename-handler (dispatch is required before end-to-end flow works)

## Files Modified

- `/home/theotime.poulain/dbt-language-server/lsp/initialize.go` -- add `RenameProvider` field and `RenameOptions` struct
- `/home/theotime.poulain/dbt-language-server/main.go` -- add two cases in `handleMessage()`

## Tests

### Capability Test

Verify the initialize response includes rename capability:

```go
// In an appropriate test file (e.g., lsp/initialize_test.go or main_test.go)
// Test: NewInitializeResponse includes RenameProvider with PrepareProvider=true
func TestInitializeResponseIncludesRenameProvider(t *testing.T) {
    resp := lsp.NewInitializeResponse(1)
    // Assert resp.Result.Capabilities.RenameProvider is non-nil
    // Assert resp.Result.Capabilities.RenameProvider.PrepareProvider == true
}
```

### Dispatch Tests

Verify `handleMessage` routes rename methods correctly. The existing codebase has no dispatch tests, so follow whatever test pattern is viable. The simplest approach: call `handleMessage` with a crafted JSON payload for each method and confirm that the `State` method is invoked (or that a response is written). Since `State.PrepareRename` and `State.Rename` may not yet be implemented, stub them to return a minimal valid response so dispatch can be tested.

```go
// Test: handleMessage dispatches "textDocument/prepareRename"
// - Create a State, construct a valid PrepareRenameRequest JSON
// - Call handleMessage with method="textDocument/prepareRename"
// - Verify a response is written to the writer (bytes.Buffer)

// Test: handleMessage dispatches "textDocument/rename"
// - Same pattern with RenameRequest JSON
```

If the `State.PrepareRename` and `State.Rename` methods do not exist yet when implementing this section, create minimal stubs on `State` that return empty/null responses so compilation and dispatch succeed.

## Implementation Details

### 1. Add RenameOptions struct and RenameProvider field

In `/home/theotime.poulain/dbt-language-server/lsp/initialize.go`:

Add a new struct:

```go
type RenameOptions struct {
    PrepareProvider bool `json:"prepareProvider"`
}
```

Add a field to `ServerCapabilities`:

```go
type ServerCapabilities struct {
    TextDocumentSync       int                    `json:"textDocumentSync"`
    HoverProvider          bool                   `json:"hoverProvider"`
    DefinitionProvider     bool                   `json:"definitionProvider"`
    CompletionProvider     map[string]any         `json:"completionProvider"`
    ExecuteCommandProvider ExecuteCommandOptions  `json:"executeCommandProvider"`
    SemanticTokensProvider *SemanticTokensOptions `json:"semanticTokensProvider,omitempty"`
    RenameProvider         *RenameOptions         `json:"renameProvider,omitempty"`
}
```

Use a pointer so it serializes as `null`/omitted when not set, and as the object form `{"prepareProvider": true}` when set. The object form is required because we implement `prepareRename`.

In `NewInitializeResponse`, populate the new field:

```go
RenameProvider: &RenameOptions{PrepareProvider: true},
```

This goes inside the `ServerCapabilities` literal in `NewInitializeResponse`, alongside the existing fields.

### 2. Add dispatch cases in main.go

In `/home/theotime.poulain/dbt-language-server/main.go`, inside the `handleMessage` function's `switch method` block, add two new cases. Follow the exact pattern used by existing handlers like `textDocument/definition`:

**Case: textDocument/prepareRename**

```go
case "textDocument/prepareRename":
    var request lsp.PrepareRenameRequest
    if err := json.Unmarshal(contents, &request); err != nil {
        logger.Printf("textDocument/prepareRename: %s", err)
        return
    }

    response := state.PrepareRename(request.ID, request.Params.TextDocument.URI, request.Params.Position)

    util.WriteResponse(writer, response)
```

**Case: textDocument/rename**

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

The `PrepareRenameRequest` and `RenameRequest` types come from section-01 (`lsp/textdocument_rename.go`). The request params follow `TextDocumentPositionParams` (which has `TextDocument` and `Position` fields), and `RenameRequest` additionally has a `NewName` field.

### 3. State method stubs (compilation bridge)

If sections 04/06 have not yet been implemented, the code will not compile because `state.PrepareRename(...)` and `state.Rename(...)` do not exist. Create minimal stubs in `/home/theotime.poulain/dbt-language-server/analysis/rename.go` to satisfy the compiler:

```go
package analysis

import "github.com/j-clemons/dbt-language-server/lsp"

// PrepareRename determines if the cursor is on a renameable symbol.
// Full implementation in section-04.
func (s *State) PrepareRename(id int, uri string, position lsp.Position) lsp.PrepareRenameResponse {
    return lsp.PrepareRenameResponse{
        Response: lsp.Response{RPC: "2.0", ID: &id},
    }
}

// Rename builds a WorkspaceEdit for renaming a symbol.
// Full implementation in section-06.
func (s *State) Rename(id int, uri string, position lsp.Position, newName string) lsp.RenameResponse {
    return lsp.RenameResponse{
        Response: lsp.Response{RPC: "2.0", ID: &id},
    }
}
```

These stubs return empty responses (which serialize as valid JSON-RPC null results) and will be replaced by later sections. The method signatures must match exactly what the dispatch code in `main.go` calls.

## Verification Checklist

1. `go build ./...` succeeds (requires section-01 types and the stubs above)
2. `NewInitializeResponse(1)` serialized to JSON contains `"renameProvider":{"prepareProvider":true}`
3. Sending a `textDocument/prepareRename` message through `handleMessage` writes a JSON-RPC response to the writer
4. Sending a `textDocument/rename` message through `handleMessage` writes a JSON-RPC response to the writer

## Implementation Notes (Post-Implementation)

### Files created
- `analysis/rename.go` — PrepareRename/Rename stub methods on State
- `lsp/initialize_test.go` — capability test

### Files modified
- `lsp/initialize.go` — RenameOptions struct, RenameProvider field + init
- `main.go` — two dispatch cases (prepareRename, rename)

### Test count: 1 (initialize response includes rename provider with JSON verification)