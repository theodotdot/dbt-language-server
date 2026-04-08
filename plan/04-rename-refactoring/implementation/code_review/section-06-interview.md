# Section 06 Code Review Interview

## Auto-fixed
- **`nameRange` column disambiguation** — added `modelName == target.modelName` check to avoid returning wrong column position when multiple models share a schema.yml.
- **`renameColumn` empty SchemaURI guard** — added early return if `model.SchemaURI == ""`.
- **Zero-range validation in PrepareRename** — check `r.Start == r.End` before setting result, prevents degenerate highlight range on fallthrough.
- **Added `TestRenameSerializesSuccess`** — verifies WorkspaceEdit round-trips through JSON with `documentChanges`.

## Let go
- **Case-sensitive conflict detection** — ModelDetailMap keys are lowercase by convention, matches dbt CLI behavior.
- **`filepath.Dir` + `/` separator** — Linux-only codebase, URIs use forward slashes.
- **No `.yaml` suffix check** — pre-existing behavior, not a regression.
- **Weak no-schema test** — test is still valid, relies on empty model paths.
- **ModelDetailMap not updated after rename** — handled by document change pipeline.
