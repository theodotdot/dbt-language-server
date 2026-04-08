package analysis

import (
	"os"
	"regexp"
	"strings"

	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/util"
)

var dbtIdentifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func (s *State) PrepareRename(id int, uri string, position lsp.Position) lsp.PrepareRenameResponse {
	resp := lsp.PrepareRenameResponse{
		Response: lsp.Response{RPC: "2.0", ID: &id},
	}

	// Case 1: REF token in SQL file
	doc, ok := s.Documents[uri]
	if ok && doc.Tokens != nil {
		tll, err := doc.Tokens.FindTokenAtCursor(position.Line, position.Character)
		if err == nil && tll.Token.Type == parser.REF && tll.Token.Literal != "ref" {
			name := tll.Token.Literal
			if _, exists := s.DbtContext.ModelDetailMap[name]; !exists {
				return resp
			}
			// Check for cross-project ref (preceded by PACKAGE token)
			if found, _ := tll.TokenLookbackMatch(parser.PACKAGE, 2); found {
				return resp
			}
			resp.Result = &lsp.PrepareRenameResult{
				Range: lsp.Range{
					Start: lsp.Position{Line: tll.Token.Line, Character: tll.Token.Column},
					End:   lsp.Position{Line: tll.Token.Line, Character: tll.Token.Column + len(name)},
				},
				Placeholder: name,
			}
			return resp
		}
	}

	// Case 2 & 3: model name or column name in schema.yml
	if strings.HasSuffix(uri, ".yml") {
		rawPath := strings.TrimPrefix(uri, "file://")
		for modelName, details := range s.DbtContext.ModelDetailMap {
			if details.SchemaURI != rawPath {
				continue
			}
			// Case 2: model name position
			endChar := details.SchemaRange.Start.Character + len(modelName)
			if position.Line == details.SchemaRange.Start.Line &&
				position.Character >= details.SchemaRange.Start.Character &&
				position.Character < endChar {
				resp.Result = &lsp.PrepareRenameResult{
					Range: lsp.Range{
						Start: details.SchemaRange.Start,
						End:   lsp.Position{Line: details.SchemaRange.Start.Line, Character: endChar},
					},
					Placeholder: modelName,
				}
				return resp
			}
			// Case 3: column name position
			for _, col := range details.Columns {
				colEnd := col.Position.Character + len(col.Name)
				if position.Line == col.Position.Line &&
					position.Character >= col.Position.Character &&
					position.Character < colEnd {
					resp.Result = &lsp.PrepareRenameResult{
						Range: lsp.Range{
							Start: col.Position,
							End:   lsp.Position{Line: col.Position.Line, Character: colEnd},
						},
						Placeholder: col.Name,
					}
					return resp
				}
			}
		}
	}

	return resp
}

// findModelRefs scans all SQL files in the project's model paths and returns
// TextDocumentEdit entries for every same-project ref('oldName') found.
func (s *State) findModelRefs(oldName, newName string) []lsp.TextDocumentEdit {
	editsMap := map[string][]lsp.TextEdit{}

	for _, modelPath := range s.DbtContext.ProjectYaml.ModelPaths.Value {
		fullPath := s.DbtContext.ProjectRoot + "/" + modelPath
		if _, err := os.Stat(fullPath); err != nil {
			continue
		}
		files, err := util.WalkFilepath(fullPath, ".sql")
		if err != nil {
			continue
		}
		for _, filePath := range files {
			uri := "file://" + filePath
			var tokens *parser.TokenIndex

			if doc, ok := s.Documents[uri]; ok && doc.Tokens != nil {
				tokens = doc.Tokens
			} else {
				text, err := util.ReadFileContents(filePath)
				if err != nil {
					continue
				}
				p := parser.Parse(text, s.DbtContext.Dialect)
				tokens = p.CreateTokenIndex()
			}

			for _, lineTokens := range tokens.LineTokens() {
				for _, tll := range lineTokens {
					if tll.Token.Type != parser.REF || tll.Token.Literal != oldName {
						continue
					}
					if found, _ := tll.TokenLookbackMatch(parser.PACKAGE, 2); found {
						continue
					}
					editsMap[uri] = append(editsMap[uri], lsp.TextEdit{
						Range: lsp.Range{
							Start: lsp.Position{Line: tll.Token.Line, Character: tll.Token.Column},
							End:   lsp.Position{Line: tll.Token.Line, Character: tll.Token.Column + len(oldName)},
						},
						NewText: newName,
					})
				}
			}
		}
	}

	result := make([]lsp.TextDocumentEdit, 0, len(editsMap))
	for uri, edits := range editsMap {
		result = append(result, lsp.TextDocumentEdit{
			TextDocument: lsp.OptionalVersionedTextDocumentIdentifier{URI: uri},
			Edits:        edits,
		})
	}
	return result
}

// Rename builds a WorkspaceEdit for renaming a symbol.
// Full implementation in section-06.
func (s *State) Rename(id int, uri string, position lsp.Position, newName string) lsp.RenameResponse {
	return lsp.RenameResponse{
		Response: lsp.Response{RPC: "2.0", ID: &id},
	}
}
