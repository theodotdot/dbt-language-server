# Section 06 Code Review

## Correctness
- `ResolveColumnsAtCursor` correctly handles nil scope, empty sources, alias prefix filtering
- CTE star expansion recurses with visited set to prevent cycles
- Self-join correctly produces separate ScopeColumn entries per alias
- Source/Table kinds return empty (known limitation per plan)

## Design Decisions
- Dropped `cursorLine`/`cursorCol` params from signature — clause identification not needed since columns from FROM/JOIN are available in all clauses. Can be added later if needed.
- `ModelColumns` type avoids import cycle with analysis package. Caller converts before calling.

## Test Coverage
- 10 tests covering: ref, CTE, star expansion, alias filtering, multiple sources, unknown table, source kind, nil scope, qualified forms, self-join
- All table-driven, no parsing involved (pure unit tests on QueryScope)

## Verdict
Clean implementation matching plan. Ship it.
