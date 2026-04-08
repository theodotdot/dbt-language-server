package analysis

import (
	"testing"

	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/lsp"
)

func newRenameState(uri, sql string) *State {
	s := NewState()
	p := parser.Parse(sql, "snowflake")
	s.Documents[uri] = Document{
		Text:      sql,
		Tokens:    p.CreateTokenIndex(),
		DefTokens: p.CreateTokenNameMap(),
	}
	return &s
}

func TestPrepareRenameRefToken(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('orders') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {URI: "models/orders.sql"},
	}

	resp := s.PrepareRename(1, uri, lsp.Position{Line: 0, Character: 22})
	if resp.Result == nil {
		t.Fatal("expected non-nil result for REF token")
	}
	if resp.Result.Placeholder != "orders" {
		t.Errorf("placeholder: got %q, want %q", resp.Result.Placeholder, "orders")
	}
	width := resp.Result.Range.End.Character - resp.Result.Range.Start.Character
	if width != len("orders") {
		t.Errorf("range width: got %d, want %d", width, len("orders"))
	}
}

func TestPrepareRenameModelInSchema(t *testing.T) {
	uri := "file:///models/schema.yml"
	s := NewState()
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"customers": {
			SchemaURI: "/models/schema.yml",
			SchemaRange: lsp.Range{
				Start: lsp.Position{Line: 5, Character: 10},
				End:   lsp.Position{Line: 5, Character: 10},
			},
		},
	}

	resp := s.PrepareRename(1, uri, lsp.Position{Line: 5, Character: 12})
	if resp.Result == nil {
		t.Fatal("expected non-nil result for model in schema")
	}
	if resp.Result.Placeholder != "customers" {
		t.Errorf("placeholder: got %q, want %q", resp.Result.Placeholder, "customers")
	}
	if resp.Result.Range.End.Character != 10+len("customers") {
		t.Errorf("end char: got %d, want %d", resp.Result.Range.End.Character, 10+len("customers"))
	}
}

func TestPrepareRenameColumnInSchema(t *testing.T) {
	uri := "file:///models/schema.yml"
	s := NewState()
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {
			SchemaURI: "/models/schema.yml",
			SchemaRange: lsp.Range{
				Start: lsp.Position{Line: 3, Character: 8},
				End:   lsp.Position{Line: 3, Character: 8},
			},
			Columns: []Column{
				{Name: "order_id", Position: lsp.Position{Line: 7, Character: 12}},
			},
		},
	}

	resp := s.PrepareRename(1, uri, lsp.Position{Line: 7, Character: 14})
	if resp.Result == nil {
		t.Fatal("expected non-nil result for column in schema")
	}
	if resp.Result.Placeholder != "order_id" {
		t.Errorf("placeholder: got %q, want %q", resp.Result.Placeholder, "order_id")
	}
}

func TestPrepareRenameCrossProjectRefReturnsNull(t *testing.T) {
	uri := "file:///test.sql"
	// Cross-project ref: ref('pkg', 'orders')
	sql := "select * from {{ ref('pkg', 'orders') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {},
	}

	// Find the position of "orders" in the parsed tokens
	resp := s.PrepareRename(1, uri, lsp.Position{Line: 0, Character: 30})
	if resp.Result != nil {
		t.Error("expected nil result for cross-project ref")
	}
}

func TestPrepareRenameNonRenameableReturnsNull(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ source('src', 'tbl') }}"
	s := newRenameState(uri, sql)

	// Cursor on "select" keyword
	resp := s.PrepareRename(1, uri, lsp.Position{Line: 0, Character: 0})
	if resp.Result != nil {
		t.Error("expected nil result for non-renameable position")
	}
}

func TestPrepareRenameNonExistentModelReturnsNull(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('nonexistent') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{}

	resp := s.PrepareRename(1, uri, lsp.Position{Line: 0, Character: 22})
	if resp.Result != nil {
		t.Error("expected nil result for non-existent model")
	}
}

func TestPrepareRenameRangeWidth(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('my_model') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"my_model": {},
	}

	resp := s.PrepareRename(1, uri, lsp.Position{Line: 0, Character: 22})
	if resp.Result == nil {
		t.Fatal("expected non-nil result")
	}
	width := resp.Result.Range.End.Character - resp.Result.Range.Start.Character
	if width != len("my_model") {
		t.Errorf("width: got %d, want %d", width, len("my_model"))
	}
}
