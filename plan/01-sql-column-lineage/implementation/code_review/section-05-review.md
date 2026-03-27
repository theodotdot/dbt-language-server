# Section 05 Code Review

## Correctness

### Scope push/pop
- `pushCTEScope` correctly parents to `scopeStack[0]` (top-level), not current scope. Good.
- `popCTEScope` guard `len(p.scopeStack) <= 1` prevents underflow. Good.
- Clause stack parallels scope stack correctly.

### Column extraction
- `popCTEScope` iterates top-level CTEs map to find the matching scope by pointer equality (`cteDef.Scope == cteScope`). Works but O(n) scan over all CTEs. Acceptable for typical CTE counts.
- Star handling: appends "*" string. Resolution deferred to section-06. Correct per plan.

### Chained CTEs
- RPAREN handler duplicates the chained CTE registration logic from `parseWith()` (save name, registerCTE, skip AS, push scope). Duplication is minor and acceptable given the control flow.
- `continue` after `p.ctes.Ind = false` correctly re-processes the current token in the main switch. Critical fix.

### CTE lookup from FROM/JOIN
- Changed from `p.currentScope().CTEs` to `p.scopeStack[0].CTEs`. Correct — CTEs are always registered on the top-level scope.

## Potential Issues

### Minor: popCTEScope finds CTE by scope pointer scan
- Iterates all CTEs to find matching scope. Could store CTE name on the scope or pass it to popCTEScope. Not worth changing — CTE counts are small.

### Minor: Missing test for Jinja set between WITH and SELECT
- Plan mentions `{% set %}` between CTE and main SELECT. Not tested. Low risk since `parseJinjaBlock` doesn't affect clause state.

## Test Coverage
- 6 CTE tests + 2 error recovery tests. Covers: single, chained, star, aliases, backward compat, scope isolation, missing paren, jinja-only input.
- Good coverage of the planned test matrix.

## Verdict
Implementation matches plan. No critical issues. Ship it.
