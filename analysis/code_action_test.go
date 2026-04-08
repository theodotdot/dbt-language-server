package analysis

import (
	"strings"
	"testing"

	"github.com/j-clemons/dbt-language-server/lsp"
)

func TestTextDocumentCodeAction(t *testing.T) {
	t.Run("expand bare star", func(t *testing.T) {
		state := newTestState()
		uri := "test://action.sql"
		state.parseDocument(uri, "select * from {{ ref('customers') }}")

		resp := state.TextDocumentCodeAction(1, uri, lsp.Range{
			Start: lsp.Position{Line: 0, Character: 7},
			End:   lsp.Position{Line: 0, Character: 8},
		})

		if len(resp.Result) != 1 {
			t.Fatalf("expected 1 code action, got %d", len(resp.Result))
		}
		action := resp.Result[0]
		if action.Title != "Expand SELECT *" {
			t.Errorf("title: got %q, want %q", action.Title, "Expand SELECT *")
		}
		if action.Kind != "refactor" {
			t.Errorf("kind: got %q, want %q", action.Kind, "refactor")
		}
		if action.Edit == nil {
			t.Fatal("expected edit, got nil")
		}
		edits, ok := action.Edit.Changes[uri]
		if !ok || len(edits) != 1 {
			t.Fatalf("expected 1 text edit for uri, got %v", action.Edit.Changes)
		}
		// Should contain both columns from customers model
		if !strings.Contains(edits[0].NewText, "customer_id") {
			t.Errorf("NewText missing customer_id: %q", edits[0].NewText)
		}
		if !strings.Contains(edits[0].NewText, "first_name") {
			t.Errorf("NewText missing first_name: %q", edits[0].NewText)
		}
		// Range should cover the * token
		if edits[0].Range.Start.Character != 7 || edits[0].Range.End.Character != 8 {
			t.Errorf("range: got %v, want col 7-8", edits[0].Range)
		}
	})

	t.Run("no columns available", func(t *testing.T) {
		state := newTestState()
		uri := "test://action.sql"
		// ref to a model with no columns defined
		state.parseDocument(uri, "select * from {{ ref('unknown_model') }}")

		resp := state.TextDocumentCodeAction(1, uri, lsp.Range{
			Start: lsp.Position{Line: 0, Character: 7},
			End:   lsp.Position{Line: 0, Character: 8},
		})

		if len(resp.Result) != 0 {
			t.Fatalf("expected 0 code actions, got %d", len(resp.Result))
		}
	})

	t.Run("cursor not on star", func(t *testing.T) {
		state := newTestState()
		uri := "test://action.sql"
		state.parseDocument(uri, "select customer_id from {{ ref('customers') }}")

		resp := state.TextDocumentCodeAction(1, uri, lsp.Range{
			Start: lsp.Position{Line: 0, Character: 7},
			End:   lsp.Position{Line: 0, Character: 18},
		})

		if len(resp.Result) != 0 {
			t.Fatalf("expected 0 code actions, got %d", len(resp.Result))
		}
	})

	t.Run("qualified star with alias", func(t *testing.T) {
		state := newTestState()
		uri := "test://action.sql"
		state.parseDocument(uri, "select c.* from {{ ref('customers') }} c")

		resp := state.TextDocumentCodeAction(1, uri, lsp.Range{
			Start: lsp.Position{Line: 0, Character: 7},
			End:   lsp.Position{Line: 0, Character: 10},
		})

		if len(resp.Result) != 1 {
			t.Fatalf("expected 1 code action, got %d", len(resp.Result))
		}
		edits := resp.Result[0].Edit.Changes[uri]
		if len(edits) != 1 {
			t.Fatalf("expected 1 edit, got %d", len(edits))
		}
		// Range should cover "c.*" (3 chars: c, dot, star)
		if edits[0].Range.Start.Character != 7 {
			t.Errorf("start char: got %d, want 7", edits[0].Range.Start.Character)
		}
		if edits[0].Range.End.Character != 10 {
			t.Errorf("end char: got %d, want 10", edits[0].Range.End.Character)
		}
		// Should have customer columns
		if !strings.Contains(edits[0].NewText, "customer_id") {
			t.Errorf("NewText missing customer_id: %q", edits[0].NewText)
		}
	})

	t.Run("document not found", func(t *testing.T) {
		state := newTestState()
		resp := state.TextDocumentCodeAction(1, "test://nonexistent.sql", lsp.Range{})
		if len(resp.Result) != 0 {
			t.Fatalf("expected 0 code actions, got %d", len(resp.Result))
		}
	})

	t.Run("nil scope fallback", func(t *testing.T) {
		state := newTestState()
		uri := "test://action.sql"
		state.Documents[uri] = Document{Text: "select * from foo"}
		resp := state.TextDocumentCodeAction(1, uri, lsp.Range{
			Start: lsp.Position{Line: 0, Character: 7},
			End:   lsp.Position{Line: 0, Character: 8},
		})
		if len(resp.Result) != 0 {
			t.Fatalf("expected 0 code actions with nil scope, got %d", len(resp.Result))
		}
	})

	t.Run("multiline expansion indentation", func(t *testing.T) {
		state := newTestState()
		uri := "test://action.sql"
		state.parseDocument(uri, "select\n    * from {{ ref('customers') }}")

		resp := state.TextDocumentCodeAction(1, uri, lsp.Range{
			Start: lsp.Position{Line: 1, Character: 4},
			End:   lsp.Position{Line: 1, Character: 5},
		})

		if len(resp.Result) != 1 {
			t.Fatalf("expected 1 code action, got %d", len(resp.Result))
		}
		newText := resp.Result[0].Edit.Changes[uri][0].NewText
		// Should have indented continuation lines
		lines := strings.Split(newText, ",\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 lines, got %d: %q", len(lines), newText)
		}
		// Second line should be indented to match the * position (col 4)
		if !strings.HasPrefix(lines[1], "    ") {
			t.Errorf("expected 4-space indent on continuation, got %q", lines[1])
		}
	})

	t.Run("range filters unrelated stars", func(t *testing.T) {
		state := newTestState()
		uri := "test://action.sql"
		// Two SELECT * on different lines
		state.parseDocument(uri, "select * from {{ ref('customers') }}\nunion all\nselect * from {{ ref('customers') }}")

		// Request only covers line 0
		resp := state.TextDocumentCodeAction(1, uri, lsp.Range{
			Start: lsp.Position{Line: 0, Character: 0},
			End:   lsp.Position{Line: 0, Character: 36},
		})

		if len(resp.Result) != 1 {
			t.Fatalf("expected 1 code action (range-filtered), got %d", len(resp.Result))
		}
	})

	t.Run("full range returns all stars", func(t *testing.T) {
		state := newTestState()
		uri := "test://action.sql"
		state.parseDocument(uri, "select * from {{ ref('customers') }}\nunion all\nselect * from {{ ref('customers') }}")

		// Request covers entire file
		resp := state.TextDocumentCodeAction(1, uri, lsp.Range{
			Start: lsp.Position{Line: 0, Character: 0},
			End:   lsp.Position{Line: 2, Character: 36},
		})

		if len(resp.Result) != 2 {
			t.Fatalf("expected 2 code actions for 2 stars, got %d", len(resp.Result))
		}
	})
}
