# Section 07 Code Review Interview

## Auto-fixed
- **Null guard in CrossProjectExcluded** — added `resp.Error` and `resp.Result` checks before iterating.
- **Consolidated path helpers** — replaced `testdataRoot()` and `renameFixturesRoot()` with single `testdataPath(t, rel)` using `testutils.GetTestdataPath`, matching codebase convention.
- **Error code assertion in table-driven test** — added `lsp.ErrCodeInvalidParams` check for invalid identifier cases.

## Let go
- **Duplicate fixture directories** — `testdata/models/` (section-05) and `testdata/rename_fixtures/` (section-07) serve different test groups; consolidation not worth the churn.
- **ColumnPositionPropagation not "true" integration** — YAML parsing already verified in state_test.go; this test verifies renameColumn uses positions correctly.
- **Missing in-memory override via full Rename API** — already covered by TestFindModelRefs_UsesOpenDocuments.
- **Missing noop via integration path** — already covered by TestRenameSameNameNoop.
