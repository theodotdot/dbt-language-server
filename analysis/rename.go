package analysis

import (
	"fmt"
	"os"
	"path/filepath"
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

	target := s.resolveRenameTarget(uri, position)
	if target == nil {
		return resp
	}

	r := s.nameRange(uri, position, target)
	if r.Start == r.End {
		return resp
	}
	resp.Result = &lsp.PrepareRenameResult{Range: r, Placeholder: target.name}
	return resp
}

// nameRange returns the range of the entity name at the given position.
func (s *State) nameRange(uri string, position lsp.Position, target *renameTarget) lsp.Range {
	// REF token in SQL
	doc, ok := s.Documents[uri]
	if ok && doc.Tokens != nil {
		tll, err := doc.Tokens.FindTokenAtCursor(position.Line, position.Character)
		if err == nil && tll.Token.Type == parser.REF && tll.Token.Literal == target.name {
			return lsp.Range{
				Start: lsp.Position{Line: tll.Token.Line, Character: tll.Token.Column},
				End:   lsp.Position{Line: tll.Token.Line, Character: tll.Token.Column + len(target.name)},
			}
		}
	}

	// Schema.yml model or column
	rawPath := strings.TrimPrefix(uri, "file://")
	for modelName, details := range s.DbtContext.ModelDetailMap {
		if details.SchemaURI != rawPath {
			continue
		}
		if target.kind == "model" && modelName == target.name {
			return lsp.Range{
				Start: details.SchemaRange.Start,
				End:   lsp.Position{Line: details.SchemaRange.Start.Line, Character: details.SchemaRange.Start.Character + len(target.name)},
			}
		}
		if target.kind == "column" && modelName == target.modelName {
			for _, col := range details.Columns {
				if col.Name == target.name {
					return lsp.Range{
						Start: col.Position,
						End:   lsp.Position{Line: col.Position.Line, Character: col.Position.Character + len(target.name)},
					}
				}
			}
		}
	}
	return lsp.Range{}
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

type renameTarget struct {
	kind      string // "model" or "column"
	name      string
	modelName string // set for column renames
}

func (s *State) resolveRenameTarget(uri string, position lsp.Position) *renameTarget {
	// Case 1: REF token in SQL file
	doc, ok := s.Documents[uri]
	if ok && doc.Tokens != nil {
		tll, err := doc.Tokens.FindTokenAtCursor(position.Line, position.Character)
		if err == nil && tll.Token.Type == parser.REF && tll.Token.Literal != "ref" {
			name := tll.Token.Literal
			if _, exists := s.DbtContext.ModelDetailMap[name]; !exists {
				return nil
			}
			if found, _ := tll.TokenLookbackMatch(parser.PACKAGE, 2); found {
				return nil
			}
			return &renameTarget{kind: "model", name: name}
		}
	}

	// Case 2 & 3: model or column in schema.yml
	if strings.HasSuffix(uri, ".yml") {
		rawPath := strings.TrimPrefix(uri, "file://")
		for modelName, details := range s.DbtContext.ModelDetailMap {
			if details.SchemaURI != rawPath {
				continue
			}
			endChar := details.SchemaRange.Start.Character + len(modelName)
			if position.Line == details.SchemaRange.Start.Line &&
				position.Character >= details.SchemaRange.Start.Character &&
				position.Character < endChar {
				return &renameTarget{kind: "model", name: modelName}
			}
			for _, col := range details.Columns {
				colEnd := col.Position.Character + len(col.Name)
				if position.Line == col.Position.Line &&
					position.Character >= col.Position.Character &&
					position.Character < colEnd {
					return &renameTarget{kind: "column", name: col.Name, modelName: modelName}
				}
			}
		}
	}
	return nil
}

func (s *State) Rename(id int, uri string, position lsp.Position, newName string) lsp.RenameResponse {
	resp := lsp.RenameResponse{
		Response: lsp.Response{RPC: "2.0", ID: &id},
	}

	if !dbtIdentifierRegex.MatchString(newName) {
		resp.Error = &lsp.ResponseError{Code: lsp.ErrCodeInvalidParams, Message: fmt.Sprintf("invalid identifier: %q", newName)}
		return resp
	}

	target := s.resolveRenameTarget(uri, position)
	if target == nil {
		return resp
	}

	if newName == target.name {
		resp.Result = &lsp.WorkspaceEdit{}
		return resp
	}

	if target.kind == "model" {
		return s.renameModel(resp, target.name, newName)
	}
	return s.renameColumn(resp, target.modelName, target.name, newName)
}

func (s *State) renameModel(resp lsp.RenameResponse, oldName, newName string) lsp.RenameResponse {
	if _, exists := s.DbtContext.ModelDetailMap[newName]; exists {
		resp.Error = &lsp.ResponseError{Code: lsp.ErrCodeInvalidParams, Message: fmt.Sprintf("model %q already exists", newName)}
		return resp
	}

	model := s.DbtContext.ModelDetailMap[oldName]
	var changes []lsp.DocumentChange

	// RenameFile
	if model.URI != "" {
		newPath := filepath.Dir(model.URI) + "/" + newName + filepath.Ext(model.URI)
		changes = append(changes, lsp.DocumentChange{
			RenameFileValue: &lsp.RenameFile{
				Kind:   "rename",
				OldURI: "file://" + model.URI,
				NewURI: "file://" + newPath,
			},
		})
	}

	// Schema edit
	if model.SchemaURI != "" {
		changes = append(changes, lsp.DocumentChange{
			TextDocumentEditValue: &lsp.TextDocumentEdit{
				TextDocument: lsp.OptionalVersionedTextDocumentIdentifier{URI: "file://" + model.SchemaURI},
				Edits: []lsp.TextEdit{{
					Range: lsp.Range{
						Start: model.SchemaRange.Start,
						End:   lsp.Position{Line: model.SchemaRange.Start.Line, Character: model.SchemaRange.Start.Character + len(oldName)},
					},
					NewText: newName,
				}},
			},
		})
	}

	// Ref edits from project scan
	for _, docEdit := range s.findModelRefs(oldName, newName) {
		de := docEdit
		changes = append(changes, lsp.DocumentChange{TextDocumentEditValue: &de})
	}

	resp.Result = &lsp.WorkspaceEdit{DocumentChanges: changes}
	return resp
}

func (s *State) renameColumn(resp lsp.RenameResponse, modelName, oldName, newName string) lsp.RenameResponse {
	model := s.DbtContext.ModelDetailMap[modelName]
	if model.SchemaURI == "" {
		return resp
	}

	// Conflict check
	for _, col := range model.Columns {
		if col.Name == newName {
			resp.Error = &lsp.ResponseError{Code: lsp.ErrCodeInvalidParams, Message: fmt.Sprintf("column %q already exists in model %q", newName, modelName)}
			return resp
		}
	}

	// Find the target column's position
	for _, col := range model.Columns {
		if col.Name == oldName {
			change := lsp.DocumentChange{
				TextDocumentEditValue: &lsp.TextDocumentEdit{
					TextDocument: lsp.OptionalVersionedTextDocumentIdentifier{URI: "file://" + model.SchemaURI},
					Edits: []lsp.TextEdit{{
						Range: lsp.Range{
							Start: col.Position,
							End:   lsp.Position{Line: col.Position.Line, Character: col.Position.Character + len(oldName)},
						},
						NewText: newName,
					}},
				},
			}
			resp.Result = &lsp.WorkspaceEdit{DocumentChanges: []lsp.DocumentChange{change}}
			return resp
		}
	}
	return resp
}
