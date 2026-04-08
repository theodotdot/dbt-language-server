package analysis

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/lsp/completionKind"
)

func getRefCompletionItems(modelMap map[string]ModelDetails, suffix string) []lsp.CompletionItem {
	items := make([]lsp.CompletionItem, 0, len(modelMap))

	for k := range modelMap {
		items = append(
			items,
			lsp.CompletionItem{
				Label:         k,
				Detail:        fmt.Sprintf("Project: %s", modelMap[k].ProjectName),
				Documentation: modelMap[k].Description,
				Kind:          completionKind.Reference,
				InsertText:    fmt.Sprintf("%s%s", k, suffix),
				SortText:      k,
			},
		)
	}

	return items
}

func reverseRefPrefix(str string) string {
	var result string
	for _, v := range str {
		switch v {
		case '(':
			result = ")" + result
		case '{':
			result = "}" + result
		default:
			result = string(v) + result
		}
	}

	return result
}

func getSuffix(leadingStr string, trailingStr string, suffixType string) string {
	if trailingStr != "" {
		return ""
	}
	leadingSymbols := regexp.MustCompile(`{{\s*` + suffixType + `\(('|")`)
	prefix := leadingSymbols.FindString(leadingStr)
	suffix := reverseRefPrefix(strings.Replace(prefix, suffixType, "", 1))

	return suffix
}

// Get the last used quote type in the string
func getQuoteType(str string) string {
	quotes := regexp.MustCompile(`['"]`)
	allMatches := quotes.FindString(str)
	if len(allMatches) == 0 {
		return ""
	}
	return string(allMatches[len(allMatches)-1])
}

func buildMacroSnippet(prefix string, args []MacroArg) string {
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteByte('(')
	for i, arg := range args {
		if i > 0 {
			b.WriteString(", ")
		}
		if arg.Default != "" {
			b.WriteString(fmt.Sprintf("${%d:%s}", i+1, arg.Default))
		} else {
			b.WriteString(fmt.Sprintf("${%d:%s}", i+1, arg.Name))
		}
	}
	b.WriteByte(')')
	return b.String()
}

func getMacroCompletionItems(packageMacroMap map[Package]map[string]Macro, ProjectYaml DbtProjectYaml) []lsp.CompletionItem {
	items := make([]lsp.CompletionItem, 0, len(packageMacroMap))

	for _, macroMap := range packageMacroMap {
		for k := range macroMap {
			macro := macroMap[k]
			var prefix string
			if ProjectYaml.ProjectName.Value == string(macro.ProjectName) {
				prefix = k
			} else {
				prefix = fmt.Sprintf("%s.%s", macro.ProjectName, k)
			}

			insertText := prefix
			insertTextFormat := 0
			if len(macro.Arguments) > 0 {
				insertText = buildMacroSnippet(prefix, macro.Arguments)
				insertTextFormat = 2 // Snippet
			}

			items = append(
				items,
				lsp.CompletionItem{
					Label:            k,
					Detail:           fmt.Sprintf("Project: %s", macro.ProjectName),
					Documentation:    macro.Description,
					Kind:             completionKind.Snippet,
					InsertText:       insertText,
					InsertTextFormat: insertTextFormat,
					SortText:         k,
				},
			)
		}
	}

	return items
}

func getVariableCompletionItems(variables map[string]Variable, suffix string) []lsp.CompletionItem {
	items := make([]lsp.CompletionItem, 0, len(variables))

	for k, v := range variables {
		items = append(
			items,
			lsp.CompletionItem{
				Label:         k,
				Detail:        k,
				Documentation: fmt.Sprintf("%v", v.Value),
				Kind:          completionKind.Variable,
				InsertText:    fmt.Sprintf("%s%s", k, suffix),
				SortText:      k,
			},
		)
	}

	return items
}

func getColumnCompletionItems(modelNames []string, modelMap map[string]ModelDetails) []lsp.CompletionItem {
	seen := make(map[string]bool)
	items := make([]lsp.CompletionItem, 0)

	for _, name := range modelNames {
		model, ok := modelMap[name]
		if !ok {
			continue
		}
		for _, col := range model.Columns {
			if seen[col.Name] {
				continue
			}
			seen[col.Name] = true
			items = append(items, lsp.CompletionItem{
				Label:         col.Name,
				Detail:        fmt.Sprintf("Column from %s", name),
				Documentation: col.Description,
				Kind:          completionKind.Field,
				InsertText:    col.Name,
				SortText:      col.Name,
			})
		}
	}

	return items
}

func getMacroArgCompletionItems(macro Macro, providedArgs map[string]bool) []lsp.CompletionItem {
	items := make([]lsp.CompletionItem, 0, len(macro.Arguments))
	for _, arg := range macro.Arguments {
		if providedArgs[arg.Name] {
			continue
		}
		detail := "required"
		if arg.Default != "" {
			detail = fmt.Sprintf("default: %s", arg.Default)
		}
		items = append(items, lsp.CompletionItem{
			Label:      arg.Name,
			Detail:     detail,
			Kind:       completionKind.Variable,
			InsertText: arg.Name + "=",
			SortText:   "00" + arg.Name,
		})
	}
	return items
}

func getScopeColumnCompletionItems(columns []parser.ScopeColumn) []lsp.CompletionItem {
	items := make([]lsp.CompletionItem, 0, len(columns))
	for _, col := range columns {
		items = append(items, lsp.CompletionItem{
			Label:         col.Name,
			Detail:        fmt.Sprintf("Column from %s", col.Source),
			Documentation: col.Description,
			Kind:          completionKind.Field,
			InsertText:    col.Name,
			SortText:      "0" + col.Name,
		})
	}
	return items
}

func getSourceCompletionItems(sources map[string]Source, suffix string, quoteType string) []lsp.CompletionItem {
	items := make([]lsp.CompletionItem, 0, len(sources))

	for k, s := range sources {
		for _, t := range s.Tables {
			items = append(
				items,
				lsp.CompletionItem{
					Label:         fmt.Sprintf("%s - %s", s.Name, t.Name),
					Detail:        fmt.Sprintf("Source: %s", s.Name),
					Documentation: fmt.Sprintf("%s\n\nTable: %s\n%s", s.Description, t.Name, t.Description),
					Kind:          completionKind.Reference,
					InsertText:    fmt.Sprintf("%s%s, %s%s%s", s.Name, quoteType, quoteType, t.Name, suffix),
					SortText:      k,
				},
			)
		}
	}

	return items
}
