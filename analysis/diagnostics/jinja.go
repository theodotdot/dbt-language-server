package diagnostics

import (
	"fmt"
	"sort"

	"github.com/j-clemons/dbt-language-server/analysis"
	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/lsp"
)

type blockEntry struct {
	tag  string
	line int
	col  int
}

var blockOpeners = map[string]bool{
	"if": true, "for": true, "block": true, "macro": true,
	"call": true, "filter": true, "raw": true,
}

var blockClosers = map[string]string{
	"endif": "if", "endfor": "for", "endblock": "block",
	"endmacro": "macro", "endcall": "call", "endfilter": "filter",
	"endraw": "raw",
}

// CheckJinja validates Jinja syntax structure in the token stream.
func CheckJinja(doc analysis.Document, _ analysis.DbtContext) []lsp.Diagnostic {
	if doc.Tokens == nil {
		return nil
	}

	lt := doc.Tokens.LineTokens()
	lines := sortedLines(lt)
	tokens := flattenTokens(lt, lines)

	var diags []lsp.Diagnostic
	var stack []blockEntry
	inRaw := false

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i].Token

		// Empty expression check: {{ immediately followed by }}
		if tok.Type == parser.DB_LBRACE {
			if i+1 < len(tokens) && tokens[i+1].Token.Type == parser.DB_RBRACE {
				diags = append(diags, lsp.Diagnostic{
					Range:    tokenRange(tok),
					Severity: 1,
					Source:   "dbt-ls",
					Message:  `Empty expression "{{ }}"`,
				})
			}
			continue
		}

		// Block tag detection: look for {%
		if tok.Type != parser.JINJA_LBRACE {
			continue
		}

		// Find the first meaningful token after {%
		keyword, keyTok := findBlockKeyword(tokens, i)
		if keyword == "" {
			continue
		}

		// Raw block handling
		if inRaw {
			if keyword == "endraw" {
				if len(stack) > 0 && stack[len(stack)-1].tag == "raw" {
					stack = stack[:len(stack)-1]
				}
				inRaw = false
			}
			continue
		}

		// Opening tag
		if blockOpeners[keyword] {
			stack = append(stack, blockEntry{tag: keyword, line: keyTok.Line, col: keyTok.Column})
			if keyword == "raw" {
				inRaw = true
			}
			continue
		}

		// Intermediate tags
		if keyword == "elif" || keyword == "else" {
			topTag := ""
			if len(stack) > 0 {
				topTag = stack[len(stack)-1].tag
			}
			if keyword == "elif" && topTag != "if" {
				diags = append(diags, lsp.Diagnostic{
					Range:    tokenRange(keyTok),
					Severity: 1,
					Source:   "dbt-ls",
					Message:  `Unexpected "{% elif %}" — no matching "{% if %}"`,
				})
			} else if keyword == "else" && topTag != "if" && topTag != "for" {
				diags = append(diags, lsp.Diagnostic{
					Range:    tokenRange(keyTok),
					Severity: 1,
					Source:   "dbt-ls",
					Message:  `Unexpected "{% else %}" — no matching "{% if %}" or "{% for %}"`,
				})
			}
			continue
		}

		// Closing tag
		if expected, ok := blockClosers[keyword]; ok {
			if len(stack) == 0 {
				diags = append(diags, lsp.Diagnostic{
					Range:    tokenRange(keyTok),
					Severity: 1,
					Source:   "dbt-ls",
					Message:  fmt.Sprintf(`Unexpected "{%% %s %%}" — no matching "{%% %s %%}"`, keyword, expected),
				})
				continue
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if top.tag != expected {
				diags = append(diags, lsp.Diagnostic{
					Range:    tokenRange(keyTok),
					Severity: 1,
					Source:   "dbt-ls",
					Message:  fmt.Sprintf(`Expected "{%% end%s %%}" but found "{%% %s %%}". Innermost open block is "{%% %s %%}"`, top.tag, keyword, top.tag),
				})
			}
		}
	}

	// Unclosed blocks at EOF
	for _, entry := range stack {
		diags = append(diags, lsp.Diagnostic{
			Range: lsp.Range{
				Start: lsp.Position{Line: entry.line, Character: entry.col},
				End:   lsp.Position{Line: entry.line, Character: entry.col + len(entry.tag)},
			},
			Severity: 1,
			Source:   "dbt-ls",
			Message:  fmt.Sprintf(`Unclosed "{%% %s %%}" block (opened at line %d)`, entry.tag, entry.line+1),
		})
	}

	return diags
}

// findBlockKeyword finds the first IDENT-like token after a JINJA_LBRACE at position i.
// Returns the keyword string and the token. Handles FOR and ELSE being SQL keywords.
func findBlockKeyword(tokens []parser.TokenLL, lbraceIdx int) (string, parser.Token) {
	for j := lbraceIdx + 1; j < len(tokens); j++ {
		tok := tokens[j].Token
		if tok.Type == parser.JINJA_RBRACE {
			break
		}
		if tok.Type == parser.IDENT {
			return tok.Literal, tok
		}
		// FOR and ELSE are SQL keywords, not IDENT
		if tok.Type == parser.FOR {
			return "for", tok
		}
		if tok.Type == parser.ELSE {
			return "else", tok
		}
		// SET handled by parser specially — not a block opener
		if tok.Type == parser.SET {
			return "", tok
		}
	}
	return "", parser.Token{}
}

func sortedLines(lt map[int][]parser.TokenLL) []int {
	lines := make([]int, 0, len(lt))
	for l := range lt {
		lines = append(lines, l)
	}
	sort.Ints(lines)
	return lines
}

func flattenTokens(lt map[int][]parser.TokenLL, lines []int) []parser.TokenLL {
	var result []parser.TokenLL
	for _, l := range lines {
		result = append(result, lt[l]...)
	}
	return result
}
