package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/docs"
	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/lsp/completionKind"
	"github.com/j-clemons/dbt-language-server/testutils"
)

func newTestState() *State {
	s := NewState()
	s.DbtContext.Dialect = docs.Dialect("snowflake")
	s.DbtContext.ProjectYaml = DbtProjectYaml{
		ProjectName: AnnotatedField[string]{Value: "test_project"},
	}
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"customers": {
			URI:         "/test/models/customers.sql",
			ProjectName: "test_project",
			Description: "Customer model description",
			Columns: []Column{
				{Name: "customer_id", Description: "Unique customer identifier"},
				{Name: "first_name", Description: "Customer first name"},
			},
		},
	}
	s.DbtContext.SourceDetailMap = map[string]Source{
		"my_source": {
			Name:        "my_source",
			Description: "Source description",
			URI:         "/test/models/schema.yml",
			Range: lsp.Range{
				Start: lsp.Position{Line: 10, Character: 0},
				End:   lsp.Position{Line: 10, Character: 0},
			},
			Tables: map[string]SourceTable{
				"my_table": {
					Name:        "my_table",
					Description: "Table description",
					Table:       "my_source",
					URI:         "/test/models/schema.yml",
					Range: lsp.Range{
						Start: lsp.Position{Line: 12, Character: 0},
						End:   lsp.Position{Line: 12, Character: 0},
					},
					Columns: []Column{
						{Name: "id", Description: "Primary key"},
						{Name: "name", Description: "Name field"},
					},
				},
			},
		},
	}
	s.DbtContext.MacroDetailMap = map[Package]map[string]Macro{
		"test_project": {
			"my_macro": {
				Name:        "my_macro",
				ProjectName: "test_project",
				Description: "my_macro(arg1)",
				Arguments:   []MacroArg{{Name: "arg1"}},
				URI:         "/test/macros/my_macro.sql",
				Range: lsp.Range{
					Start: lsp.Position{Line: 0, Character: 9},
					End:   lsp.Position{Line: 0, Character: 25},
				},
			},
		},
		"other_pkg": {
			"ext_macro": {
				Name:        "ext_macro",
				ProjectName: "other_pkg",
				Description: "ext_macro(x, y)",
				Arguments:   []MacroArg{{Name: "x"}, {Name: "y"}},
				URI:         "/test/dbt_packages/other_pkg/macros/ext.sql",
				Range: lsp.Range{
					Start: lsp.Position{Line: 2, Character: 9},
					End:   lsp.Position{Line: 2, Character: 30},
				},
			},
		},
	}
	s.DbtContext.VariableDetailMap = map[string]Variable{
		"my_var": {
			Name:  "my_var",
			Value: 42,
			URI:   "/test/dbt_project.yml",
			Range: lsp.Range{
				Start: lsp.Position{Line: 5, Character: 10},
				End:   lsp.Position{Line: 5, Character: 10},
			},
		},
	}
	return &s
}

func TestHover(t *testing.T) {
	state := newTestState()
	uri := "test://hover.sql"

	tests := []struct {
		name     string
		sql      string
		pos      lsp.Position
		expected string
	}{
		{
			name:     "hover on ref",
			sql:      `select * from {{ ref('customers') }}`,
			pos:      lsp.Position{Line: 0, Character: 22},
			expected: "Customer model description",
		},
		{
			name:     "hover on var",
			sql:      `select {{ var('my_var') }}`,
			pos:      lsp.Position{Line: 0, Character: 15},
			expected: "my_var: 42",
		},
		{
			name:     "hover on source",
			sql:      `select * from {{ source('my_source', 'my_table') }}`,
			pos:      lsp.Position{Line: 0, Character: 25},
			expected: "Source description",
		},
		{
			name: "hover on source table",
			sql:  `select * from {{ source('my_source', 'my_table') }}`,
			pos:  lsp.Position{Line: 0, Character: 38},
			expected: fmt.Sprintf(
				"Source: %s\n%s\n\nTable: %s\n%s",
				"my_source", "Source description",
				"my_table", "Table description",
			),
		},
		{
			name:     "hover on macro",
			sql:      `select {{ my_macro(1) }}`,
			pos:      lsp.Position{Line: 0, Character: 10},
			expected: "my_macro(arg1)",
		},
		{
			name:     "hover on packaged macro",
			sql:      `select {{ other_pkg.ext_macro(1, 2) }}`,
			pos:      lsp.Position{Line: 0, Character: 20},
			expected: "ext_macro(x, y)",
		},
		{
			name:     "hover on unknown token",
			sql:      `select * from users`,
			pos:      lsp.Position{Line: 0, Character: 15},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state.parseDocument(uri, tc.sql)
			resp := state.Hover(1, uri, tc.pos)
			if resp.Result.Contents != tc.expected {
				t.Errorf("got %q, want %q", resp.Result.Contents, tc.expected)
			}
		})
	}
}

func TestDefinition(t *testing.T) {
	state := newTestState()
	uri := "test://def.sql"

	tests := []struct {
		name        string
		sql         string
		pos         lsp.Position
		expectedURI string
		expectedPos lsp.Position
	}{
		{
			name:        "go to def on ref",
			sql:         `select * from {{ ref('customers') }}`,
			pos:         lsp.Position{Line: 0, Character: 22},
			expectedURI: "file:///test/models/customers.sql",
			expectedPos: lsp.Position{Line: 0, Character: 0},
		},
		{
			name:        "go to def on source",
			sql:         `select * from {{ source('my_source', 'my_table') }}`,
			pos:         lsp.Position{Line: 0, Character: 25},
			expectedURI: "file:///test/models/schema.yml",
			expectedPos: lsp.Position{Line: 10, Character: 0},
		},
		{
			name:        "go to def on source table",
			sql:         `select * from {{ source('my_source', 'my_table') }}`,
			pos:         lsp.Position{Line: 0, Character: 38},
			expectedURI: "file:///test/models/schema.yml",
			expectedPos: lsp.Position{Line: 12, Character: 0},
		},
		{
			name:        "go to def on var",
			sql:         `select {{ var('my_var') }}`,
			pos:         lsp.Position{Line: 0, Character: 15},
			expectedURI: "file:///test/dbt_project.yml",
			expectedPos: lsp.Position{Line: 5, Character: 10},
		},
		{
			name:        "go to def on macro",
			sql:         `select {{ my_macro(1) }}`,
			pos:         lsp.Position{Line: 0, Character: 10},
			expectedURI: "file:///test/macros/my_macro.sql",
			expectedPos: lsp.Position{Line: 0, Character: 9},
		},
		{
			name:        "go to def on packaged macro",
			sql:         `select {{ other_pkg.ext_macro(1, 2) }}`,
			pos:         lsp.Position{Line: 0, Character: 20},
			expectedURI: "file:///test/dbt_packages/other_pkg/macros/ext.sql",
			expectedPos: lsp.Position{Line: 2, Character: 9},
		},
		{
			name: "go to def on jinja set variable",
			sql: `{% set my_val = 42 %}
select {{ my_val }}`,
			pos:         lsp.Position{Line: 1, Character: 10},
			expectedURI: uri,
			expectedPos: lsp.Position{Line: 0, Character: 7},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state.parseDocument(uri, tc.sql)
			resp := state.Definition(1, uri, tc.pos)
			if resp.Result.URI != tc.expectedURI {
				t.Errorf("URI: got %q, want %q", resp.Result.URI, tc.expectedURI)
			}
			if resp.Result.Range.Start != tc.expectedPos {
				t.Errorf("Position: got %v, want %v", resp.Result.Range.Start, tc.expectedPos)
			}
		})
	}
}

func TestTokenToSemanticType(t *testing.T) {
	tests := []struct {
		tokenType    parser.TokenType
		expectedType int
		expectedOk   bool
	}{
		{parser.REF, semFunction, true},
		{parser.SOURCE, semFunction, true},
		{parser.VAR, semFunction, true},
		{parser.CONFIG, semFunction, true},
		{parser.MACRO, semMacro, true},
		{parser.JINJA_SET, semVariable, true},
		{parser.PACKAGE, semNamespace, true},
		{parser.DB_LBRACE, semOperator, true},
		{parser.DB_RBRACE, semOperator, true},
		{parser.JINJA_LBRACE, semOperator, true},
		{parser.JINJA_RBRACE, semOperator, true},
		{parser.SET, semKeyword, true},
		{parser.IDENT, 0, false},
		{parser.SELECT, 0, false},
	}

	for _, tc := range tests {
		t.Run(string(tc.tokenType), func(t *testing.T) {
			semType, ok := tokenToSemanticType(tc.tokenType)
			if ok != tc.expectedOk {
				t.Errorf("ok: got %v, want %v", ok, tc.expectedOk)
			}
			if ok && semType != tc.expectedType {
				t.Errorf("type: got %d, want %d", semType, tc.expectedType)
			}
		})
	}
}

func TestSemanticTokensFull(t *testing.T) {
	state := newTestState()
	uri := "test://semantic.sql"

	state.parseDocument(uri, `select * from {{ ref('customers') }}`)

	resp := state.SemanticTokensFull(1, uri)
	data := resp.Result.Data

	if len(data) == 0 {
		t.Fatal("expected semantic tokens data, got empty")
	}

	// data should be multiples of 5
	if len(data)%5 != 0 {
		t.Fatalf("data length %d is not a multiple of 5", len(data))
	}

	// Verify DB_LBRACE token is present (should map to semOperator=5)
	foundOperator := false
	foundFunction := false
	for i := 0; i < len(data); i += 5 {
		tokenType := data[i+3]
		if tokenType == semOperator {
			foundOperator = true
		}
		if tokenType == semFunction {
			foundFunction = true
		}
	}

	if !foundOperator {
		t.Error("expected operator semantic token for {{ }}")
	}
	if !foundFunction {
		t.Error("expected function semantic token for ref")
	}
}

func TestSemanticTokensFullNoDocument(t *testing.T) {
	state := newTestState()
	resp := state.SemanticTokensFull(1, "nonexistent://file.sql")
	if len(resp.Result.Data) != 0 {
		t.Errorf("expected empty data for nonexistent document, got %v", resp.Result.Data)
	}
}

func TestSemanticTokensFullJinjaSet(t *testing.T) {
	state := newTestState()
	uri := "test://set.sql"

	state.parseDocument(uri, `{% set my_var = 1 %}
select {{ my_var }}`)

	resp := state.SemanticTokensFull(1, uri)
	data := resp.Result.Data

	foundVariable := false
	foundKeyword := false
	for i := 0; i < len(data); i += 5 {
		tokenType := data[i+3]
		if tokenType == semVariable {
			foundVariable = true
		}
		if tokenType == semKeyword {
			foundKeyword = true
		}
	}

	if !foundVariable {
		t.Error("expected variable semantic token for set variable")
	}
	if !foundKeyword {
		t.Error("expected keyword semantic token for 'set'")
	}
}

func TestGetReferencedModels(t *testing.T) {
	state := newTestState()
	uri := "test://ref.sql"

	tests := []struct {
		name     string
		sql      string
		expected []string
	}{
		{
			name:     "single ref",
			sql:      "select * from {{ ref('customers') }}",
			expected: []string{"customers"},
		},
		{
			name:     "multiple refs",
			sql:      "select * from {{ ref('customers') }} join {{ ref('orders') }}",
			expected: []string{"customers", "orders"},
		},
		{
			name:     "duplicate ref",
			sql:      "{{ ref('customers') }} union {{ ref('customers') }}",
			expected: []string{"customers"},
		},
		{
			name:     "no refs",
			sql:      "select * from my_table",
			expected: nil,
		},
		{
			name:     "ref on different lines",
			sql:      "select * from {{ ref('customers') }}\njoin {{ ref('orders') }} on true",
			expected: []string{"customers", "orders"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state.parseDocument(uri, tc.sql)
			models := getReferencedModels(state.Documents[uri].Tokens)
			if len(models) != len(tc.expected) {
				t.Fatalf("expected %d models, got %d: %v", len(tc.expected), len(models), models)
			}
			got := make(map[string]bool)
			for _, m := range models {
				got[m] = true
			}
			for _, e := range tc.expected {
				if !got[e] {
					t.Errorf("missing expected model %q", e)
				}
			}
		})
	}

	t.Run("nil tokens", func(t *testing.T) {
		models := getReferencedModels(nil)
		if models != nil {
			t.Errorf("expected nil, got %v", models)
		}
	})
}

func TestTextDocumentCompletion(t *testing.T) {
	state := newTestState()
	uri := "test://completion.sql"

	tests := []struct {
		name      string
		sql       string
		pos       lsp.Position
		checkItem func(items []lsp.CompletionItem) bool
	}{
		{
			name: "ref completion",
			sql:  `select * from {{ ref('`,
			pos:  lsp.Position{Line: 0, Character: 22},
			checkItem: func(items []lsp.CompletionItem) bool {
				for _, item := range items {
					if item.Label == "customers" {
						return true
					}
				}
				return false
			},
		},
		{
			name: "var completion",
			sql:  `select {{ var('`,
			pos:  lsp.Position{Line: 0, Character: 15},
			checkItem: func(items []lsp.CompletionItem) bool {
				for _, item := range items {
					if item.Label == "my_var" {
						return true
					}
				}
				return false
			},
		},
		{
			name: "source completion",
			sql:  `select * from {{ source('`,
			pos:  lsp.Position{Line: 0, Character: 25},
			checkItem: func(items []lsp.CompletionItem) bool {
				for _, item := range items {
					if item.Label == "my_source - my_table" {
						return true
					}
				}
				return false
			},
		},
		{
			name: "macro completion",
			sql:  `select {{ `,
			pos:  lsp.Position{Line: 0, Character: 10},
			checkItem: func(items []lsp.CompletionItem) bool {
				for _, item := range items {
					if item.Label == "my_macro" {
						return true
					}
				}
				return false
			},
		},
		{
			name: "column completion from ref'd model",
			sql:  "select  from {{ ref('customers') }}",
			pos:  lsp.Position{Line: 0, Character: 7},
			checkItem: func(items []lsp.CompletionItem) bool {
				for _, item := range items {
					if item.Label == "customer_id" {
						return true
					}
				}
				return false
			},
		},
		{
			name: "no column completion without ref",
			sql:  "select  from my_table",
			pos:  lsp.Position{Line: 0, Character: 7},
			checkItem: func(items []lsp.CompletionItem) bool {
				for _, item := range items {
					if item.Kind == completionKind.Field {
						return false
					}
				}
				return true
			},
		},
		{
			name: "column completion includes sql functions",
			sql:  "select  from {{ ref('customers') }}",
			pos:  lsp.Position{Line: 0, Character: 7},
			checkItem: func(items []lsp.CompletionItem) bool {
				hasColumn := false
				hasFunction := false
				for _, item := range items {
					if item.Kind == completionKind.Field {
						hasColumn = true
					}
					if item.Kind == completionKind.Function {
						hasFunction = true
					}
				}
				return hasColumn && hasFunction
			},
		},
		{
			name: "column completion multiline ref",
			sql:  "select \nfrom {{ ref('customers') }}",
			pos:  lsp.Position{Line: 0, Character: 7},
			checkItem: func(items []lsp.CompletionItem) bool {
				for _, item := range items {
					if item.Label == "first_name" {
						return true
					}
				}
				return false
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state.parseDocument(uri, tc.sql)
			resp := state.TextDocumentCompletion(1, uri, tc.pos)
			if !tc.checkItem(resp.Result) {
				t.Errorf("expected completion item not found in %v", resp.Result)
			}
		})
	}
}

func TestScopeColumnCompletion(t *testing.T) {
	state := newTestState()
	uri := "test://scope-completion.sql"

	t.Run("source column completion", func(t *testing.T) {
		state.parseDocument(uri, "select \nfrom {{ source('my_source', 'my_table') }}")
		resp := state.TextDocumentCompletion(1, uri, lsp.Position{Line: 0, Character: 7})
		foundId := false
		foundName := false
		for _, item := range resp.Result {
			if item.Label == "id" && item.Kind == completionKind.Field {
				foundId = true
			}
			if item.Label == "name" && item.Kind == completionKind.Field {
				foundName = true
			}
		}
		if !foundId {
			t.Error("expected 'id' column from source")
		}
		if !foundName {
			t.Error("expected 'name' column from source")
		}
	})

	t.Run("source with no columns", func(t *testing.T) {
		state.DbtContext.SourceDetailMap["empty_source"] = Source{
			Name: "empty_source",
			Tables: map[string]SourceTable{
				"empty_table": {
					Name:  "empty_table",
					Table: "empty_source",
				},
			},
		}
		state.parseDocument(uri, "select \nfrom {{ source('empty_source', 'empty_table') }}")
		resp := state.TextDocumentCompletion(1, uri, lsp.Position{Line: 0, Character: 7})
		for _, item := range resp.Result {
			if item.Kind == completionKind.Field {
				t.Errorf("expected no column items, got %q", item.Label)
			}
		}
	})

	t.Run("backward compat nil scope", func(t *testing.T) {
		state.parseDocument(uri, "select \nfrom {{ ref('customers') }}")
		doc := state.Documents[uri]
		doc.Scope = nil
		state.Documents[uri] = doc
		resp := state.TextDocumentCompletion(1, uri, lsp.Position{Line: 0, Character: 7})
		foundColumn := false
		for _, item := range resp.Result {
			if item.Label == "customer_id" && item.Kind == completionKind.Field {
				foundColumn = true
			}
		}
		if !foundColumn {
			t.Error("expected 'customer_id' from fallback path")
		}
	})
}

func TestDotQualifiedCompletion(t *testing.T) {
	state := newTestState()
	// Add orders model for alias tests
	state.DbtContext.ModelDetailMap["orders"] = ModelDetails{
		URI:         "/test/models/orders.sql",
		ProjectName: "test_project",
		Columns: []Column{
			{Name: "order_id", Description: "Order ID"},
			{Name: "customer_id", Description: "FK"},
		},
	}
	uri := "test://dot.sql"

	t.Run("alias dot returns aliased columns", func(t *testing.T) {
		state.parseDocument(uri, "select o. from {{ ref('orders') }} o")
		resp := state.TextDocumentCompletion(1, uri, lsp.Position{Line: 0, Character: 9})
		if len(resp.Result) == 0 {
			t.Fatal("expected completion items for alias 'o'")
		}
		for _, item := range resp.Result {
			if item.Kind != completionKind.Field {
				t.Errorf("expected Field kind, got %d for %q", item.Kind, item.Label)
			}
		}
		labels := map[string]bool{}
		for _, item := range resp.Result {
			labels[item.Label] = true
		}
		if !labels["order_id"] || !labels["customer_id"] {
			t.Errorf("expected order_id and customer_id, got %v", labels)
		}
	})

	t.Run("jinja dot is NOT column completion", func(t *testing.T) {
		state.parseDocument(uri, "select {{ dbt_utils.")
		resp := state.TextDocumentCompletion(1, uri, lsp.Position{Line: 0, Character: 20})
		for _, item := range resp.Result {
			if item.Kind == completionKind.Field {
				t.Errorf("should not return Field items inside jinja, got %q", item.Label)
			}
		}
	})

	t.Run("jinja config dot is NOT column completion", func(t *testing.T) {
		state.parseDocument(uri, "select {{ config.")
		resp := state.TextDocumentCompletion(1, uri, lsp.Position{Line: 0, Character: 17})
		for _, item := range resp.Result {
			if item.Kind == completionKind.Field {
				t.Errorf("should not return Field items inside jinja, got %q", item.Label)
			}
		}
	})

	t.Run("unknown alias returns no columns", func(t *testing.T) {
		state.parseDocument(uri, "select x. from {{ ref('orders') }}")
		resp := state.TextDocumentCompletion(1, uri, lsp.Position{Line: 0, Character: 9})
		for _, item := range resp.Result {
			if item.Kind == completionKind.Field {
				t.Errorf("should not return Field items for unknown alias, got %q", item.Label)
			}
		}
	})

	t.Run("filter text set correctly", func(t *testing.T) {
		state.parseDocument(uri, "select o. from {{ ref('orders') }} o")
		resp := state.TextDocumentCompletion(1, uri, lsp.Position{Line: 0, Character: 9})
		for _, item := range resp.Result {
			expected := "o." + item.Label
			if item.FilterText != expected {
				t.Errorf("FilterText: got %q, want %q", item.FilterText, expected)
			}
		}
	})

	t.Run("text edit present with correct range", func(t *testing.T) {
		state.parseDocument(uri, "select o. from {{ ref('orders') }} o")
		resp := state.TextDocumentCompletion(1, uri, lsp.Position{Line: 0, Character: 9})
		for _, item := range resp.Result {
			if item.TextEdit == nil {
				t.Errorf("expected TextEdit for %q", item.Label)
				continue
			}
			if item.TextEdit.Range.Start.Character != 9 {
				t.Errorf("TextEdit start: got %d, want 9", item.TextEdit.Range.Start.Character)
			}
			if item.TextEdit.NewText != item.Label {
				t.Errorf("TextEdit NewText: got %q, want %q", item.TextEdit.NewText, item.Label)
			}
		}
	})

	t.Run("join with two aliases", func(t *testing.T) {
		sql := "select c. from {{ ref('customers') }} c join {{ ref('orders') }} o on c.customer_id = o.customer_id"
		state.parseDocument(uri, sql)
		resp := state.TextDocumentCompletion(1, uri, lsp.Position{Line: 0, Character: 9})
		labels := map[string]bool{}
		for _, item := range resp.Result {
			labels[item.Label] = true
		}
		if !labels["customer_id"] || !labels["first_name"] {
			t.Errorf("expected customer columns for alias 'c', got %v", labels)
		}
		if labels["order_id"] {
			t.Error("should not include order columns for alias 'c'")
		}
	})
}

func TestDocumentScopePopulated(t *testing.T) {
	state := newTestState()
	uri := "test://scope.sql"

	tests := []struct {
		name       string
		sql        string
		minSources int
	}{
		{"ref produces source", "select id from {{ ref('customers') }}", 1},
		{"plain SQL", "select 1", 0},
		{"empty string", "", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state.parseDocument(uri, tc.sql)
			doc := state.Documents[uri]
			if doc.Scope == nil {
				t.Fatal("expected non-nil Scope")
			}
			if len(doc.Scope.Sources) < tc.minSources {
				t.Errorf("expected at least %d sources, got %d", tc.minSources, len(doc.Scope.Sources))
			}
		})
	}
}

func TestResolveColumnsAtPosition(t *testing.T) {
	state := newTestState()
	uri := "test://resolve.sql"

	state.parseDocument(uri, "select  from {{ ref('customers') }}")
	cols := state.ResolveColumnsAtPosition(uri, "")
	if len(cols) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(cols))
	}

	names := map[string]bool{}
	for _, c := range cols {
		names[c.Name] = true
	}
	if !names["customer_id"] || !names["first_name"] {
		t.Errorf("expected customer_id and first_name, got %v", cols)
	}
}

func TestResolveColumnsAtPositionCTE(t *testing.T) {
	state := newTestState()
	uri := "test://cte.sql"

	state.parseDocument(uri, "with cte as (select customer_id from {{ ref('customers') }}) select  from cte")
	cols := state.ResolveColumnsAtPosition(uri, "")
	if len(cols) != 1 {
		t.Fatalf("expected 1 CTE column, got %d: %+v", len(cols), cols)
	}
	if cols[0].Name != "customer_id" {
		t.Errorf("expected 'customer_id', got %q", cols[0].Name)
	}
}

func TestResolveColumnsAtPositionAliasFiltered(t *testing.T) {
	state := newTestState()
	uri := "test://alias.sql"

	state.parseDocument(uri, "select c. from {{ ref('customers') }} c")
	cols := state.ResolveColumnsAtPosition(uri, "c")
	if len(cols) != 2 {
		t.Fatalf("expected 2 columns for alias 'c', got %d", len(cols))
	}
	for _, c := range cols {
		if c.Source != "c" {
			t.Errorf("expected source 'c', got %q", c.Source)
		}
	}
}

func integrationTestState(t *testing.T) (*State, string) {
	t.Helper()
	testdataRoot, err := testutils.GetTestdataPath("jaffle_shop_duckdb")
	if err != nil {
		t.Fatal(err)
	}
	state := NewState()
	state.refreshDbtContext(testdataRoot)
	return &state, testdataRoot
}

func TestResolveColumnsIntegration(t *testing.T) {
	state, _ := integrationTestState(t)
	uri := "test://integration.sql"

	t.Run("simple ref columns", func(t *testing.T) {
		state.parseDocument(uri, "select  from {{ ref('customers') }}")
		cols := state.ResolveColumnsAtPosition(uri, "")
		if len(cols) < 3 {
			t.Fatalf("expected >= 3 columns from customers, got %d", len(cols))
		}
		names := map[string]bool{}
		for _, c := range cols {
			names[c.Name] = true
		}
		for _, expected := range []string{"customer_id", "first_name", "last_name"} {
			if !names[expected] {
				t.Errorf("missing expected column %q", expected)
			}
		}
	})

	t.Run("multi-CTE query", func(t *testing.T) {
		sql := `with order_totals as (
    select customer_id, count(order_id) as order_count
    from {{ ref('stg_orders') }}
    group by customer_id
), customer_info as (
    select customer_id, first_name
    from {{ ref('stg_customers') }}
)
select ci.customer_id, ci.first_name, ot.order_count
from customer_info ci
join order_totals ot on ci.customer_id = ot.customer_id`
		state.parseDocument(uri, sql)
		cols := state.ResolveColumnsAtPosition(uri, "ci")
		if len(cols) != 2 {
			t.Fatalf("expected 2 columns for alias 'ci', got %d: %+v", len(cols), cols)
		}
	})

	t.Run("alias filtering in JOIN", func(t *testing.T) {
		sql := `select o.order_id, c.first_name
from {{ ref('orders') }} o
join {{ ref('customers') }} c on o.customer_id = c.customer_id`
		state.parseDocument(uri, sql)
		oCols := state.ResolveColumnsAtPosition(uri, "o")
		if len(oCols) == 0 {
			t.Fatal("expected columns for alias 'o'")
		}
		for _, c := range oCols {
			if c.Source != "o" {
				t.Errorf("expected source 'o', got %q", c.Source)
			}
		}
	})

	t.Run("jinja if/else produces multiple sources", func(t *testing.T) {
		sql := "select order_id from\n{% if true %}\n  {{ ref('orders') }}\n{% else %}\n  {{ ref('stg_orders') }}\n{% endif %}"
		state.parseDocument(uri, sql)
		scope := state.Documents[uri].Scope
		if scope == nil {
			t.Fatal("expected non-nil scope")
		}
		if len(scope.Sources) < 2 {
			t.Errorf("expected >= 2 sources from jinja if/else, got %d", len(scope.Sources))
		}
	})

	t.Run("testdata file parse", func(t *testing.T) {
		testdataRoot, _ := testutils.GetTestdataPath("jaffle_shop_duckdb")
		content, err := os.ReadFile(filepath.Join(testdataRoot, "models/order_details.sql"))
		if err != nil {
			t.Fatal(err)
		}
		state.parseDocument(uri, string(content))
		scope := state.Documents[uri].Scope
		if scope == nil {
			t.Fatal("expected non-nil scope")
		}
		if len(scope.Sources) < 3 {
			t.Errorf("expected >= 3 sources in order_details, got %d", len(scope.Sources))
		}
	})
}
