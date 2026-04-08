package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCompletionItemFilterText(t *testing.T) {
	t.Run("includes filterText when set", func(t *testing.T) {
		item := CompletionItem{Label: "id", FilterText: "o.id"}
		data, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"filterText":"o.id"`) {
			t.Errorf("expected filterText in JSON, got %s", data)
		}
	})

	t.Run("omits filterText when empty", func(t *testing.T) {
		item := CompletionItem{Label: "id"}
		data, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "filterText") {
			t.Errorf("expected no filterText in JSON, got %s", data)
		}
	})
}

func TestCompletionItemTextEdit(t *testing.T) {
	t.Run("includes textEdit when set", func(t *testing.T) {
		item := CompletionItem{
			Label: "id",
			TextEdit: &TextEdit{
				Range:   Range{Start: Position{Line: 0, Character: 2}, End: Position{Line: 0, Character: 4}},
				NewText: "customer_id",
			},
		}
		data, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		s := string(data)
		if !strings.Contains(s, `"textEdit"`) {
			t.Errorf("expected textEdit in JSON, got %s", s)
		}
		if !strings.Contains(s, `"newText":"customer_id"`) {
			t.Errorf("expected newText in JSON, got %s", s)
		}
	})

	t.Run("omits textEdit when nil", func(t *testing.T) {
		item := CompletionItem{Label: "id"}
		data, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "textEdit") {
			t.Errorf("expected no textEdit in JSON, got %s", data)
		}
	})
}
