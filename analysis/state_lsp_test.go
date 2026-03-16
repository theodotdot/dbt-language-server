package analysis

import (
	"fmt"
	"testing"

	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/docs"
	"github.com/j-clemons/dbt-language-server/lsp"
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
