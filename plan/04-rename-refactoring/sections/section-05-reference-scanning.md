I now have enough context. Let me produce the section content.

# Section 5: Project-Wide Reference Scanning

## Overview

This section implements the function that scans all SQL files in a dbt project to find every `ref('modelName')` call matching a given model name. The result is a set of `TextDocumentEdit` entries (grouped by file URI) used by the rename handler (section 06) when constructing the `WorkspaceEdit` for a model rename.

The function lives in `analysis/rename.go` alongside the PrepareRename logic from section 04.

## Dependencies

- **Section 01 (LSP Types):** Uses `lsp.TextDocumentEdit`, `lsp.TextEdit`, `lsp.Range`, `lsp.Position` from `lsp/textdocument_rename.go`.
- Relies on existing codebase constructs: `parser.Parse`, `parser.TokenIndex`, `parser.TokenLL`, `parser.REF`, `parser.PACKAGE`, `TokenLookbackMatch`, `util.WalkFilepath`, `util.ReadFileContents`.

## Tests

File: `/home/theotime.poulain/dbt-language-server/analysis/rename_test.go`

These tests exercise the reference scanning function. For disk-based tests, create fixture SQL files under `testdata/`.

```go
// Test: finds ref('target') in a single SQL file -> correct TextEdit range
func TestFindModelRefs_SingleFile(t *testing.T) {
    // Set up State with a SQL file containing {{ ref('orders') }}
    // Either as an open document (in State.Documents) or as a fixture file on disk
    // Call the scanning function with oldName="orders"
    // Assert: returns one TextDocumentEdit with one TextEdit at the correct range
}

// Test: finds ref('target') across multiple SQL files -> edits grouped by URI
func TestFindModelRefs_MultipleFiles(t *testing.T) {
    // Create 2+ fixture SQL files each containing ref('orders')
    // Assert: returns multiple TextDocumentEdit entries, one per file URI
}

// Test: skips files with no ref() calls
func TestFindModelRefs_SkipsNonMatching(t *testing.T) {
    // File with ref('other_model') only
    // Assert: no edits returned for that file
}

// Test: skips cross-project ref('pkg', 'target') -- PACKAGE token lookback filters it
func TestFindModelRefs_SkipsCrossProjectRef(t *testing.T) {
    // File containing {{ ref('some_pkg', 'orders') }}
    // Assert: no edit returned for that ref (PACKAGE token precedes REF token)
}

// Test: uses in-memory text for open documents (in State.Documents)
func TestFindModelRefs_UsesOpenDocuments(t *testing.T) {
    // Add a document to State.Documents with parsed tokens containing ref('orders')
    // The file on disk may not exist or have different content
    // Assert: edits reflect the in-memory version
}

// Test: reads from disk for closed files
func TestFindModelRefs_ReadsFromDisk(t *testing.T) {
    // Create a fixture SQL file on disk, do NOT add to State.Documents
    // Assert: scanning finds and parses the file, returns correct edits
}

// Test: skips files that fail to parse (broken SQL)
func TestFindModelRefs_SkipsBrokenFiles(t *testing.T) {
    // Create a fixture file with malformed content that the parser handles gracefully
    // Assert: no panic, returns empty or partial results
}

// Test: skips files that don't exist on disk
func TestFindModelRefs_SkipsMissingFiles(t *testing.T) {
    // Point model paths at a directory with a missing file reference
    // Assert: no panic, graceful skip
}
```

### Test Fixtures

Create SQL fixture files under `/home/theotime.poulain/dbt-language-server/testdata/models/`:

- `single_ref.sql` -- contains `select * from {{ ref('orders') }}`
- `multi_ref.sql` -- contains multiple ref calls, some matching target, some not
- `no_ref.sql` -- plain SQL with no jinja ref calls
- `cross_project_ref.sql` -- contains `{{ ref('some_pkg', 'orders') }}`

## Implementation Details

### File

`/home/theotime.poulain/dbt-language-server/analysis/rename.go` (same file as PrepareRename from section 04)

### Function Signature

```go
// findModelRefs scans all SQL files in the project's model paths and returns
// TextDocumentEdit entries for every same-project ref('oldName') found.
// Each entry is keyed by file URI with file:// prefix.
func (s *State) findModelRefs(oldName string) []lsp.TextDocumentEdit
```

This is a method on `*State` so it can access `DbtContext` (for project root, model paths, dialect) and `Documents` (for open file content).

### Algorithm

1. **Get model paths** from `s.DbtContext.ProjectYaml.ModelPaths.Value` (type `[]string`, typically `["models"]`).

2. **Walk each model path** using `util.WalkFilepath(s.DbtContext.ProjectRoot + "/" + modelPath, ".sql")` to collect all SQL file paths.

3. **For each SQL file path**, get a `*parser.TokenIndex`:
   - Check if the file is open in the editor by looking up the path in `s.Documents`. Note: `State.Documents` keys use the URI as received from the editor (with `file://` prefix stripped by `OpenDocument`). The walker returns raw filesystem paths. Match accordingly.
   - If open: use the existing `Document.Tokens` (already parsed).
   - If not open: read from disk via `util.ReadFileContents(filePath)`, parse with `parser.Parse(text, s.DbtContext.Dialect)`, and call `CreateTokenIndex()`. If reading or parsing fails, skip the file silently.

4. **Iterate all tokens** via `tokenIndex.LineTokens()`. For each `TokenLL`:
   - Check `tll.Token.Type == parser.REF` and `tll.Token.Literal == oldName`.
   - **Cross-project ref filtering:** Call `tll.TokenLookbackMatch(parser.PACKAGE, 2)`. If it returns `(true, _)`, a PACKAGE token precedes this REF token, meaning it is a `ref('pkg', 'model')` call. Skip it.
   - For valid matches, build a `lsp.TextEdit` with:
     - `Range`: start at `lsp.Position{Line: tll.Token.Line, Character: tll.Token.Column}`, end at `lsp.Position{Line: tll.Token.Line, Character: tll.Token.Column + len(oldName)}`
     - `NewText`: left empty by this function (the caller in section 06 sets it, or alternatively pass `newName` as a parameter and set it here -- either approach works, but passing `newName` is simpler)

5. **Group edits by file URI** into `lsp.TextDocumentEdit` entries. Each entry's `TextDocument.URI` must have the `file://` prefix. Collect all `TextEdit` items for the same file into one `TextDocumentEdit`.

6. Return the slice of `TextDocumentEdit` entries.

### Token Range Details

The `Token` struct has `Line int` and `Column int` fields (0-indexed). The `Literal` field contains the model name string (e.g., `"orders"` for `ref('orders')`). The token range spans from `(Line, Column)` to `(Line, Column + len(Literal))`. This covers just the model name inside the quotes, not the full `ref()` expression -- which is exactly what needs to be replaced.

### Open vs Closed File Detection

The key for `State.Documents` is the URI as passed to `OpenDocument`, which strips `file://`. However, `OpenDocument` receives the full URI from the editor. Check the existing code path: in `main.go`, `didOpen` passes `request.Params.TextDocument.URI` to `state.OpenDocument(uri, text)`, and `OpenDocument` does `strings.TrimPrefix(uri, "file://")` only for `refreshDbtContext` but stores the document under the original `uri` key. So the key in `State.Documents` retains the `file://` prefix.

To match walker paths (raw filesystem paths like `/home/user/project/models/foo.sql`) against `State.Documents` keys (like `file:///home/user/project/models/foo.sql`), prepend `file://` to the walker path before the lookup.

### Cross-Project Ref Filtering

`TokenLookbackMatch(parser.PACKAGE, 2)` walks back up to 2 tokens in the linked list. In a `ref('pkg', 'model')` call, the token sequence is approximately `PACKAGE -> REF` (the lexer emits a PACKAGE token for the first argument and REF for the second). If a PACKAGE token is found within 2 lookback steps, this is a cross-project ref and must be excluded from rename edits.

This matches the pattern used in existing handlers (e.g., `Hover` uses `TokenLookbackMatch(parser.PACKAGE, 2)` for macro resolution).

### Performance

The function walks all SQL files and parses closed files on every rename invocation. For typical dbt projects (hundreds of files), this completes in milliseconds. No caching or indexing is needed. The server is single-threaded (sequential message processing in `main.go`), so no locking is required.

### Error Handling

- If `util.WalkFilepath` fails for a model path (e.g., directory doesn't exist), skip that path and continue with others.
- If `util.ReadFileContents` fails for a file, skip it silently.
- If parsing produces no tokens, the loop simply finds no matches -- no special handling needed.

## Implementation Notes (Post-Implementation)

### Files modified
- `analysis/rename.go` — added `findModelRefs(oldName, newName string) []lsp.TextDocumentEdit`

### Files created
- `analysis/rename_test.go` — 8 new test cases (added to existing file)
- `testdata/models/single_ref.sql`, `multi_ref.sql`, `no_ref.sql`, `cross_project_ref.sql` — fixture files

### Deviations from plan
- **`findModelRefs` takes `newName` parameter** and sets `TextEdit.NewText` directly, rather than leaving it empty for the caller. Safer: prevents accidental empty-string replacements.
- **`os.Stat` guard before `WalkFilepath`** — required because `util.WalkFilepath` panics on non-existent paths (nil `os.FileInfo` dereference in its callback).
- **Omitted `TestFindModelRefs_SkipsBrokenFiles`** — `parser.Parse` handles broken input gracefully by design; low-value test.

### Test count: 8 (single file, multiple files, non-matching, cross-project skip, open docs, disk read, missing paths, range width)