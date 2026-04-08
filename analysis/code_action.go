package analysis

import (
	"fmt"
	"strings"

	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/lsp"
)

func (s *State) TextDocumentCodeAction(id int, uri string, actionRange lsp.Range) lsp.CodeActionResponse {
	response := lsp.CodeActionResponse{
		Response: lsp.Response{RPC: "2.0", ID: &id},
		Result:   []lsp.CodeAction{},
	}

	doc, exists := s.Documents[uri]
	if !exists || doc.Scope == nil {
		return response
	}

	for _, item := range doc.Scope.SelectItems {
		if !item.IsStar {
			continue
		}

		// Only return actions for stars within the requested range
		if item.Token.Line < actionRange.Start.Line || item.Token.Line > actionRange.End.Line {
			continue
		}
		if item.Token.Line == actionRange.Start.Line && item.Token.Column+1 <= actionRange.Start.Character {
			continue
		}
		if item.Token.Line == actionRange.End.Line && item.Token.Column >= actionRange.End.Character {
			continue
		}

		// Resolve columns for this star
		aliasPrefix := item.StarSource
		columns := s.ResolveColumnsAtPosition(uri, aliasPrefix)
		columns = append(columns, s.resolveSourceColumns(doc.Scope)...)

		if aliasPrefix != "" {
			var filtered []parser.ScopeColumn
			for _, c := range columns {
				if c.Source == aliasPrefix {
					filtered = append(filtered, c)
				}
			}
			columns = filtered
		}

		if len(columns) == 0 {
			continue
		}

		// Build column list with indentation
		indent := strings.Repeat(" ", item.Token.Column)
		var parts []string
		seen := make(map[string]bool)
		for _, col := range columns {
			if seen[col.Name] {
				continue
			}
			seen[col.Name] = true
			parts = append(parts, col.Name)
		}
		newText := strings.Join(parts, fmt.Sprintf(",\n%s", indent))

		// Compute edit range
		startChar := item.Token.Column
		if aliasPrefix != "" {
			// Cover "alias.*" — alias + dot + star
			startChar = item.Token.Column - len(aliasPrefix) - 1
			if startChar < 0 {
				startChar = 0
			}
		}
		editRange := lsp.Range{
			Start: lsp.Position{Line: item.Token.Line, Character: startChar},
			End:   lsp.Position{Line: item.Token.Line, Character: item.Token.Column + 1},
		}

		response.Result = append(response.Result, lsp.CodeAction{
			Title: "Expand SELECT *",
			Kind:  "refactor",
			Edit: &lsp.WorkspaceEdit{
				Changes: map[string][]lsp.TextEdit{
					uri: {{Range: editRange, NewText: newText}},
				},
			},
		})
	}

	return response
}
