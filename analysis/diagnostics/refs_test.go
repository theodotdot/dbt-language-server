package diagnostics

import (
	"testing"

	"github.com/j-clemons/dbt-language-server/analysis"
	"github.com/j-clemons/dbt-language-server/analysis/parser"
)

func makeDoc(sql string) analysis.Document {
	p := parser.Parse(sql, "snowflake")
	return analysis.Document{
		Text:      sql,
		Tokens:    p.CreateTokenIndex(),
		DefTokens: p.CreateTokenNameMap(),
		Scope:     p.CreateQueryScope(),
	}
}

func TestCheckRefs(t *testing.T) {
	t.Run("valid ref no diagnostic", func(t *testing.T) {
		doc := makeDoc("select * from {{ ref('orders') }}")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{"orders": {}},
		}
		diags := CheckRefs(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics, got %d: %v", len(diags), diags)
		}
	})

	t.Run("invalid ref", func(t *testing.T) {
		doc := makeDoc("select * from {{ ref('nonexistent') }}")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{},
		}
		diags := CheckRefs(doc, ctx)
		if len(diags) != 1 {
			t.Fatalf("expected 1 diagnostic, got %d", len(diags))
		}
		if diags[0].Severity != 1 {
			t.Errorf("severity: got %d, want 1 (Error)", diags[0].Severity)
		}
		if diags[0].Message != `Model "nonexistent" not found` {
			t.Errorf("message: got %q", diags[0].Message)
		}
	})

	t.Run("multiple invalid refs", func(t *testing.T) {
		doc := makeDoc("select * from {{ ref('a') }} join {{ ref('b') }}")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{},
		}
		diags := CheckRefs(doc, ctx)
		if len(diags) != 2 {
			t.Fatalf("expected 2 diagnostics, got %d", len(diags))
		}
	})

	t.Run("diagnostic range spans literal", func(t *testing.T) {
		doc := makeDoc("select * from {{ ref('orders') }}")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{},
		}
		diags := CheckRefs(doc, ctx)
		if len(diags) != 1 {
			t.Fatalf("expected 1 diagnostic, got %d", len(diags))
		}
		r := diags[0].Range
		if r.End.Character-r.Start.Character != len("orders") {
			t.Errorf("range width: got %d, want %d", r.End.Character-r.Start.Character, len("orders"))
		}
	})
}

func TestCheckSources(t *testing.T) {
	t.Run("valid source no diagnostic", func(t *testing.T) {
		doc := makeDoc("select * from {{ source('my_src', 'my_tbl') }}")
		ctx := analysis.DbtContext{
			SourceDetailMap: map[string]analysis.Source{
				"my_src": {Tables: map[string]analysis.SourceTable{"my_tbl": {}}},
			},
		}
		diags := CheckSources(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics, got %d: %v", len(diags), diags)
		}
	})

	t.Run("invalid source name", func(t *testing.T) {
		doc := makeDoc("select * from {{ source('bad_src', 'tbl') }}")
		ctx := analysis.DbtContext{
			SourceDetailMap: map[string]analysis.Source{},
		}
		diags := CheckSources(doc, ctx)
		if len(diags) != 1 {
			t.Fatalf("expected 1 diagnostic, got %d", len(diags))
		}
		if diags[0].Severity != 1 {
			t.Errorf("severity: got %d, want 1", diags[0].Severity)
		}
		if diags[0].Message != `Source "bad_src" not found` {
			t.Errorf("message: got %q", diags[0].Message)
		}
	})

	t.Run("invalid table name", func(t *testing.T) {
		doc := makeDoc("select * from {{ source('my_src', 'bad_tbl') }}")
		ctx := analysis.DbtContext{
			SourceDetailMap: map[string]analysis.Source{
				"my_src": {Tables: map[string]analysis.SourceTable{"good_tbl": {}}},
			},
		}
		diags := CheckSources(doc, ctx)
		if len(diags) != 1 {
			t.Fatalf("expected 1 diagnostic, got %d", len(diags))
		}
		if diags[0].Message != `Table "bad_tbl" not found in source "my_src"` {
			t.Errorf("message: got %q", diags[0].Message)
		}
	})
}

func TestCheckVars(t *testing.T) {
	t.Run("valid var no diagnostic", func(t *testing.T) {
		doc := makeDoc("select {{ var('my_var') }}")
		ctx := analysis.DbtContext{
			VariableDetailMap: map[string]analysis.Variable{"my_var": {}},
		}
		diags := CheckVars(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics, got %d: %v", len(diags), diags)
		}
	})

	t.Run("var from set no diagnostic", func(t *testing.T) {
		doc := makeDoc("{% set my_var = 1 %}\nselect {{ var('my_var') }}")
		ctx := analysis.DbtContext{
			VariableDetailMap: map[string]analysis.Variable{},
		}
		diags := CheckVars(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics (var defined via set), got %d: %v", len(diags), diags)
		}
	})

	t.Run("invalid var warning", func(t *testing.T) {
		doc := makeDoc("select {{ var('unknown_var') }}")
		ctx := analysis.DbtContext{
			VariableDetailMap: map[string]analysis.Variable{},
		}
		diags := CheckVars(doc, ctx)
		if len(diags) != 1 {
			t.Fatalf("expected 1 diagnostic, got %d", len(diags))
		}
		if diags[0].Severity != 2 {
			t.Errorf("severity: got %d, want 2 (Warning)", diags[0].Severity)
		}
		if diags[0].Message != `Variable "unknown_var" not found` {
			t.Errorf("message: got %q", diags[0].Message)
		}
	})
}

func TestCheckMacros(t *testing.T) {
	t.Run("valid macro no diagnostic", func(t *testing.T) {
		doc := makeDoc("select {{ my_macro() }}")
		ctx := analysis.DbtContext{
			MacroDetailMap: map[analysis.Package]map[string]analysis.Macro{
				"pkg": {"my_macro": {}},
			},
		}
		diags := CheckMacros(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics, got %d: %v", len(diags), diags)
		}
	})

	t.Run("macro in different package", func(t *testing.T) {
		doc := makeDoc("select {{ shared_macro() }}")
		ctx := analysis.DbtContext{
			MacroDetailMap: map[analysis.Package]map[string]analysis.Macro{
				"pkg1": {"other": {}},
				"pkg2": {"shared_macro": {}},
			},
		}
		diags := CheckMacros(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics, got %d: %v", len(diags), diags)
		}
	})

	t.Run("invalid macro warning", func(t *testing.T) {
		doc := makeDoc("select {{ unknown_macro() }}")
		ctx := analysis.DbtContext{
			MacroDetailMap: map[analysis.Package]map[string]analysis.Macro{},
		}
		diags := CheckMacros(doc, ctx)
		if len(diags) != 1 {
			t.Fatalf("expected 1 diagnostic, got %d", len(diags))
		}
		if diags[0].Severity != 2 {
			t.Errorf("severity: got %d, want 2 (Warning)", diags[0].Severity)
		}
		if diags[0].Message != `Macro "unknown_macro" not found` {
			t.Errorf("message: got %q", diags[0].Message)
		}
	})

	t.Run("jinja builtin skipped", func(t *testing.T) {
		// "if" followed by "(" would get MACRO type, but let's test the skip list
		// In practice, jinja builtins like "if" are parsed as keywords, not MACRO.
		// This test verifies the skip list works if such tokens appear.
		doc := makeDoc("{% if true %}select 1{% endif %}")
		ctx := analysis.DbtContext{
			MacroDetailMap: map[analysis.Package]map[string]analysis.Macro{},
		}
		diags := CheckMacros(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics for jinja builtins, got %d: %v", len(diags), diags)
		}
	})

	t.Run("dbt builtin skipped", func(t *testing.T) {
		// adapter() is not a keyword, so it gets MACRO type — must be in skip list
		doc := makeDoc("{{ adapter() }}")
		ctx := analysis.DbtContext{
			MacroDetailMap: map[analysis.Package]map[string]analysis.Macro{},
		}
		diags := CheckMacros(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics for dbt builtins, got %d: %v", len(diags), diags)
		}
	})
}
