package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCodeActionResponseSerialization(t *testing.T) {
	t.Run("matches LSP spec structure", func(t *testing.T) {
		resp := CodeActionResponse{
			Result: []CodeAction{
				{
					Title: "Expand *",
					Kind:  "refactor",
					Edit: &WorkspaceEdit{
						Changes: map[string][]TextEdit{
							"file:///test.sql": {
								{
									Range:   Range{Start: Position{Line: 0, Character: 7}, End: Position{Line: 0, Character: 8}},
									NewText: "col_a, col_b",
								},
							},
						},
					},
				},
			},
		}
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatal(err)
		}
		s := string(data)
		if !strings.Contains(s, `"title":"Expand *"`) {
			t.Errorf("missing title, got %s", s)
		}
		if !strings.Contains(s, `"kind":"refactor"`) {
			t.Errorf("missing kind, got %s", s)
		}
		if !strings.Contains(s, `"file:///test.sql"`) {
			t.Errorf("missing URI key in changes, got %s", s)
		}
	})

	t.Run("workspace edit with multiple URIs", func(t *testing.T) {
		edit := WorkspaceEdit{
			Changes: map[string][]TextEdit{
				"file:///a.sql": {{Range: Range{}, NewText: "a"}},
				"file:///b.sql": {{Range: Range{}, NewText: "b"}},
			},
		}
		data, err := json.Marshal(edit)
		if err != nil {
			t.Fatal(err)
		}
		s := string(data)
		if !strings.Contains(s, `"file:///a.sql"`) || !strings.Contains(s, `"file:///b.sql"`) {
			t.Errorf("missing URI keys, got %s", s)
		}
	})

	t.Run("empty code action list produces array", func(t *testing.T) {
		resp := CodeActionResponse{Result: []CodeAction{}}
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"result":[]`) {
			t.Errorf("expected empty array, got %s", data)
		}
	})
}
