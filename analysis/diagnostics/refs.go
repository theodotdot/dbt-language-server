package diagnostics

import (
	"fmt"

	"github.com/j-clemons/dbt-language-server/analysis"
	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/lsp"
)

func tokenRange(tok parser.Token) lsp.Range {
	return lsp.Range{
		Start: lsp.Position{Line: tok.Line, Character: tok.Column},
		End:   lsp.Position{Line: tok.Line, Character: tok.Column + len(tok.Literal)},
	}
}

func CheckRefs(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
	if doc.Tokens == nil {
		return nil
	}
	var diags []lsp.Diagnostic
	for _, lineTokens := range doc.Tokens.LineTokens() {
		for _, tll := range lineTokens {
			if tll.Token.Type != parser.REF || tll.Token.Literal == "ref" {
				continue
			}
			if _, ok := ctx.ModelDetailMap[tll.Token.Literal]; !ok {
				diags = append(diags, lsp.Diagnostic{
					Range:    tokenRange(tll.Token),
					Message:  fmt.Sprintf("Model %q not found", tll.Token.Literal),
					Severity: 1,
					Source:   "dbt-ls",
				})
			}
		}
	}
	return diags
}

func CheckSources(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
	if doc.Tokens == nil {
		return nil
	}
	var diags []lsp.Diagnostic
	for _, lineTokens := range doc.Tokens.LineTokens() {
		for _, tll := range lineTokens {
			if tll.Token.Type != parser.SOURCE_TABLE {
				continue
			}
			// Walk back to find SOURCE token
			found, srcName := tll.TokenLookbackMatch(parser.SOURCE, 4)
			if !found {
				continue
			}
			source, ok := ctx.SourceDetailMap[srcName]
			if !ok {
				diags = append(diags, lsp.Diagnostic{
					Range:    tokenRange(tll.Token),
					Message:  fmt.Sprintf("Source %q not found", srcName),
					Severity: 1,
					Source:   "dbt-ls",
				})
				continue
			}
			if _, ok := source.Tables[tll.Token.Literal]; !ok {
				diags = append(diags, lsp.Diagnostic{
					Range:    tokenRange(tll.Token),
					Message:  fmt.Sprintf("Table %q not found in source %q", tll.Token.Literal, srcName),
					Severity: 1,
					Source:   "dbt-ls",
				})
			}
		}
	}
	return diags
}

func CheckVars(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
	if doc.Tokens == nil {
		return nil
	}
	var diags []lsp.Diagnostic
	for _, lineTokens := range doc.Tokens.LineTokens() {
		for _, tll := range lineTokens {
			if tll.Token.Type != parser.VAR || tll.Token.Literal == "var" {
				continue
			}
			if _, ok := ctx.VariableDetailMap[tll.Token.Literal]; ok {
				continue
			}
			if _, ok := doc.DefTokens[tll.Token.Literal]; ok {
				continue
			}
			diags = append(diags, lsp.Diagnostic{
				Range:    tokenRange(tll.Token),
				Message:  fmt.Sprintf("Variable %q not found", tll.Token.Literal),
				Severity: 2,
				Source:   "dbt-ls",
			})
		}
	}
	return diags
}

var macroBuiltins = map[string]bool{
	// Jinja builtins
	"if": true, "for": true, "set": true, "block": true, "macro": true,
	"call": true, "filter": true, "raw": true, "extends": true, "include": true,
	"import": true, "from": true, "do": true, "print": true, "with": true,
	"autoescape": true,
	// dbt builtins
	"ref": true, "source": true, "var": true, "config": true, "log": true,
	"return": true, "adapter": true, "run_query": true, "statement": true,
	"exceptions": true, "modules": true, "flags": true, "graph": true,
	"model": true, "this": true, "target": true, "env_var": true,
	"project_name": true,
}

func CheckMacros(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
	if doc.Tokens == nil {
		return nil
	}
	var diags []lsp.Diagnostic
	for _, lineTokens := range doc.Tokens.LineTokens() {
		for _, tll := range lineTokens {
			if tll.Token.Type != parser.MACRO {
				continue
			}
			if macroBuiltins[tll.Token.Literal] {
				continue
			}
			found := false
			for _, macros := range ctx.MacroDetailMap {
				if _, ok := macros[tll.Token.Literal]; ok {
					found = true
					break
				}
			}
			if !found {
				diags = append(diags, lsp.Diagnostic{
					Range:    tokenRange(tll.Token),
					Message:  fmt.Sprintf("Macro %q not found", tll.Token.Literal),
					Severity: 2,
					Source:   "dbt-ls",
				})
			}
		}
	}
	return diags
}
