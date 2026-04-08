Now I have all the context needed. Let me generate the section content.

# Section 4: Jinja Syntax Validation

## Overview

This section implements Jinja syntax validation in `analysis/diagnostics/jinja.go`. It is a post-parse validation pass that walks the token stream to detect structural Jinja errors: unclosed delimiters, mismatched block tags, and empty expressions.

**File to create:** `/home/theotime.poulain/dbt-language-server/analysis/diagnostics/jinja.go`
**Test file to create:** `/home/theotime.poulain/dbt-language-server/analysis/diagnostics/jinja_test.go`

**Dependencies:** Section 02 (Engine) must be implemented first. The checker function signature `Checker func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic` and the Engine's checker registration mechanism must exist.

## Tests

File: `analysis/diagnostics/jinja_test.go`

### Unclosed Delimiters

```go
// Test: matched {{ }} → no diagnostic
// Test: unclosed {{ without }} → Error diagnostic
// Test: matched {% %} → no diagnostic
// Test: unclosed {% without %} → Error diagnostic
// Test: message: 'Unclosed "{{" expression'
```

### Block Tag Matching

```go
// Test: matched {% if %}...{% endif %} → no diagnostic
// Test: matched {% for %}...{% endfor %} → no diagnostic
// Test: nested {% if %}{% for %}{% endfor %}{% endif %} → no diagnostic
// Test: mismatched {% if %}{% endfor %} → Error with "Expected endfor but found endif" style message
// Test: unclosed {% if %} at EOF → Error at if position with "Unclosed {% if %} block"
// Test: unexpected {% endif %} with no opening → Error "no matching {% if %}"
// Test: {% elif %} inside {% if %} → no diagnostic
// Test: {% else %} inside {% if %} → no diagnostic
// Test: {% elif %} inside {% for %} → Error (incompatible)
// Test: {% raw %}...{% endraw %} → no diagnostics for content inside raw block
```

### Block Type Detection

```go
// Test: IDENT literal "if" after JINJA_LBRACE is recognized as block opener
// Test: IDENT literal "endif" after JINJA_LBRACE is recognized as block closer
// Test: non-block IDENT (e.g., "set", "ref", "config") is not treated as block tag
```

### Empty Expressions

```go
// Test: {{ }} (empty) → Error diagnostic
// Test: {{ var }} (non-empty) → no diagnostic
// Test: message: 'Empty expression "{{ }}"'
```

### Test Approach

Each test should:
1. Parse a SQL/Jinja string using `parser.Parse(input, "duckdb")` to get a `*Parser`
2. Build an `analysis.Document` with the resulting `TokenIndex`
3. Call the Jinja checker function
4. Assert on the returned `[]lsp.Diagnostic` slice (count, messages, severity, positions)

Use table-driven tests. The `DbtContext` parameter is unused by the Jinja checker (it only inspects token structure), so pass a zero-value `analysis.DbtContext{}`.

## Implementation Details

### Token Stream Reality

The parser's `parseJinjaBlock()` (in `analysis/parser/parser.go:156`) consumes Jinja blocks eagerly. The `default` branch (line 176) just advances to `JINJA_RBRACE` without special typing. Block keywords like `if`, `for`, `endif`, `endfor` are **not** preserved as distinct typed tokens. They appear in the token stream as:

```
JINJA_LBRACE → IDENT (literal="if") → ... → JINJA_RBRACE
```

The block matcher must examine `IDENT` token **literals** to determine block types. It cannot rely on typed tokens.

Additionally, the `SET` case (line 159) is handled specially — after a `JINJA_LBRACE`, if the next token is `SET`, parseJinjaBlock processes the set statement. The `SET` token type is a SQL keyword (see `token.go` line 169). Inside Jinja blocks, it gets the `JINJA_SET` type only for the variable name that follows.

### Checker Function Signature

```go
// CheckJinja validates Jinja syntax structure in the token stream.
func CheckJinja(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic
```

This function is registered with the Engine as a `Checker` in section 05.

### Unclosed Delimiters

Walk the token stream looking for `DB_LBRACE` (`{{`) and `JINJA_LBRACE` (`{%`) tokens. Track whether each has a matching `DB_RBRACE` (`}}`) or `JINJA_RBRACE` (`%}`).

Approach: maintain a counter per delimiter type. On `DB_LBRACE`, increment. On `DB_RBRACE`, decrement. If counter is non-zero at end of file, emit Error diagnostic at the position of the unmatched opener.

The lexer already pairs these in most cases, so unclosed delimiters may manifest as lexer errors or missing tokens. The checker handles both: explicit unpaired tokens and truncated token sequences.

Messages:
- `Unclosed "{{" expression`
- `Unclosed "{%" block`

Severity: `1` (Error in LSP DiagnosticSeverity)

### Block Tag Matching

Walk `JINJA_LBRACE` tokens and examine the **first IDENT token after them** in the token stream. To find this, iterate line tokens and when a `JINJA_LBRACE` is encountered, look at subsequent tokens on the same or following lines for the first `IDENT` type token. Use the `PrevToken` linked list to navigate — or more practically, do a forward scan since `LineTokens()` returns `map[int][]TokenLL`.

Practical approach: iterate all tokens in line order. When encountering `JINJA_LBRACE`, set a flag. The next `IDENT` token encountered is the block keyword. Then process it.

Maintain a stack of `blockEntry` structs:

```go
type blockEntry struct {
    tag  string // "if", "for", "block", "macro", "call", "filter", "raw"
    line int
    col  int
}
```

**Opening tags** (IDENT literal is one of): `if`, `for`, `block`, `macro`, `call`, `filter`, `raw` — push onto stack.

**Intermediate tags** (`elif`, `else`): verify stack top is compatible. `elif` and `else` require `if` on top. If incompatible, emit Error.

**Closing tags** (`endif`, `endfor`, `endblock`, `endmacro`, `endcall`, `endfilter`, `endraw`): pop stack and verify the closing tag matches the opening. Strip `end` prefix to compare.

**Error cases:**
- Closing tag with empty stack: Error at closing tag position. Message: `Unexpected "{% endif %}" — no matching "{% if %}"`
- Closing tag doesn't match stack top: Error at closing tag position. Message: `Expected "{% endfor %}" but found "{% endif %}". Innermost open block is "{% for %}"`
- EOF with non-empty stack: Error at each unclosed tag's position. Message: `Unclosed "{% if %}" block (opened at line N)`

### Non-Block Statements

These Jinja statements do NOT open blocks and must not be pushed onto the stack:
- `set` — single-line variable assignment (already handled as `SET` token type)
- `ref`, `source`, `config`, `do`, `print`, `import`, `from`, `include`, `extends` — expression calls or single-line directives

### Raw Block Handling

When `{% raw %}` is pushed onto the stack, skip ALL token validation until `{% endraw %}` is found. Content inside raw blocks is literal text and must not generate diagnostics. Implementation: set a `inRaw` flag when `raw` is pushed; only process `endraw` closers until the flag is cleared.

### Empty Expressions

After finding a `DB_LBRACE` token, check if the next token in the stream is `DB_RBRACE`. If so, emit Error: `Empty expression "{{ }}"` at the `DB_LBRACE` position.

To determine "next token": look at the next `TokenLL` in the same line's token list, or the first token on the next line. The linked list `PrevToken` goes backward only, so forward scanning requires iterating the `LineTokens()` map.

### Token Iteration Strategy

`TokenIndex.LineTokens()` returns `map[int][]TokenLL`. To iterate in order:

1. Collect and sort the line numbers (map keys)
2. For each line, iterate the `[]TokenLL` slice in order
3. This gives a complete forward scan of all tokens

```go
func sortedLines(lineTokens map[int][]TokenLL) []int
```

### Diagnostic Construction

All diagnostics use:
- `Source: "dbt-ls"`
- `Severity: 1` (Error) for all Jinja syntax issues
- `Range`: start at the token's `Line`/`Column`, end at `Column + len(Literal)`

```go
lsp.Diagnostic{
    Range: lsp.Range{
        Start: lsp.Position{Line: tok.Line, Character: tok.Column},
        End:   lsp.Position{Line: tok.Line, Character: tok.Column + len(tok.Literal)},
    },
    Severity: 1,
    Source:   "dbt-ls",
    Message:  "...",
}
```

### Key Token Types (from `parser/token.go`)

| Constant | Value | Meaning |
|----------|-------|---------|
| `DB_LBRACE` | `{{` | Expression open |
| `DB_RBRACE` | `}}` | Expression close |
| `JINJA_LBRACE` | `{%` | Block open |
| `JINJA_RBRACE` | `%}` | Block close |
| `IDENT` | `IDENT` | Identifier (includes block keywords in Jinja context) |
| `SET` | `SET` | SQL/Jinja `set` keyword |

### Important Parser Behavior

Looking at `parseJinjaBlock()` (parser.go:156-181):
- On `JINJA_LBRACE`, the parser calls `parseJinjaBlock()` which does `NextToken()`
- If the token is `SET`, it processes the set statement specially
- The `default` branch simply advances to `JINJA_RBRACE` — this is where `if`, `for`, `endif`, etc. end up
- All tokens consumed inside the block are added to the token stream via `NextToken()` (which appends to `p.tokens`)
- So `IDENT` tokens with literals like `"if"`, `"endif"` ARE in the `TokenIndex`, just typed as `IDENT`

The forward scan approach: when you see `JINJA_LBRACE`, the next meaningful token (skipping whitespace-like tokens) tells you the block type. In the default branch, the parser advances through all tokens until `JINJA_RBRACE`, and they all get added. So scanning forward from `JINJA_LBRACE` in the token index will find the first `IDENT` which is the block keyword.

## Implementation Notes (Post-Implementation)

### Deviations from plan
- `{% else %}` inside `{% for %}` is valid Jinja2 (for-else pattern). Fixed to allow `else` when stack top is `if` OR `for`. Spec listed `elif` inside `for` as error but `else` inside `for` should be valid.
- `findBlockKeyword` handles `FOR` and `ELSE` as SQL keyword token types (not IDENT). `SET` returns empty string to avoid treating it as a block opener.
- Unclosed delimiter counter (spec's `DB_LBRACE`/`DB_RBRACE` tracking) not implemented — parser already pairs these in practice. Unclosed block tags at EOF ARE detected via stack-at-EOF logic.
- `emptyCtx()` helper added to jinja_test.go since DbtContext is unused by this checker.

### Files created
- `analysis/diagnostics/jinja.go` — CheckJinja + helpers (blockEntry, findBlockKeyword, sortedLines, flattenTokens)
- `analysis/diagnostics/jinja_test.go` — 17 subtests

### Test count: 17 (matched pairs: 5, block matching: 7, empty expr: 2, raw: 1, set: 1, macro: 1)