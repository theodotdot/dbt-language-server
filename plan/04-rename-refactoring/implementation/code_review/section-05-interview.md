# Section 05 Code Review Interview

## Auto-fixed
- **`findModelRefs` takes `newName` param** — sets `NewText` on each `TextEdit` directly. Avoids dangerous empty-string default that section-06 caller could forget to patch.
- **Tightened test assertions** — exact counts (3 total edits, 2 files) instead of `< 1` / `< 2`.
- **`testdataRoot(t)` propagates errors** — uses `t.Helper()` + `t.Fatal(err)` instead of swallowing `os.Getwd()` error.

## Let go
- **TokenLookbackMatch position-2 only** — matches established codebase convention, not introduced here.
- **Missing broken-files test** — `parser.Parse` handles broken input gracefully; low value.
- **`os.Stat` guard "redundant"** — actually needed because `WalkFilepath` panics on nil `info` for non-existent paths.
- **Map iteration non-determinism** — tests don't depend on ordering.
