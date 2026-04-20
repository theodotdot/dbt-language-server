package diagnostics

import (
	"testing"

	"github.com/j-clemons/dbt-language-server/analysis"
)

func TestCheckColumns(t *testing.T) {
	t.Run("column exists no diagnostic", func(t *testing.T) {
		doc := makeDoc("select id, status from {{ ref('orders') }}")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{
				"orders": {Columns: []analysis.Column{
					{Name: "id"},
					{Name: "status"},
				}},
			},
		}
		diags := CheckColumns(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics, got %d: %v", len(diags), diags)
		}
	})

	t.Run("column missing warning", func(t *testing.T) {
		doc := makeDoc("select id, bogus from {{ ref('orders') }}")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{
				"orders": {Columns: []analysis.Column{
					{Name: "id"},
					{Name: "status"},
				}},
			},
		}
		diags := CheckColumns(doc, ctx)
		if len(diags) != 1 {
			t.Fatalf("expected 1 diagnostic, got %d: %v", len(diags), diags)
		}
		if diags[0].Severity != 2 {
			t.Errorf("severity: got %d, want 2 (Warning)", diags[0].Severity)
		}
		if diags[0].Message != `Column "bogus" not found in any source` {
			t.Errorf("message: got %q", diags[0].Message)
		}
	})

	t.Run("qualified column missing", func(t *testing.T) {
		doc := makeDoc("select o.bogus from {{ ref('orders') }} o")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{
				"orders": {Columns: []analysis.Column{
					{Name: "id"},
				}},
			},
		}
		diags := CheckColumns(doc, ctx)
		if len(diags) != 1 {
			t.Fatalf("expected 1 diagnostic, got %d: %v", len(diags), diags)
		}
		if diags[0].Message != `Column "bogus" not found in "o"` {
			t.Errorf("message: got %q", diags[0].Message)
		}
	})

	t.Run("qualified column exists", func(t *testing.T) {
		doc := makeDoc("select o.id from {{ ref('orders') }} o")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{
				"orders": {Columns: []analysis.Column{
					{Name: "id"},
				}},
			},
		}
		diags := CheckColumns(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics, got %d: %v", len(diags), diags)
		}
	})

	t.Run("unknown source skips unqualified", func(t *testing.T) {
		doc := makeDoc("select bogus from raw_table")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{},
		}
		diags := CheckColumns(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics (unknown source), got %d: %v", len(diags), diags)
		}
	})

	t.Run("star skipped", func(t *testing.T) {
		doc := makeDoc("select * from {{ ref('orders') }}")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{
				"orders": {Columns: []analysis.Column{{Name: "id"}}},
			},
		}
		diags := CheckColumns(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics for star, got %d: %v", len(diags), diags)
		}
	})

	t.Run("expression skipped", func(t *testing.T) {
		doc := makeDoc("select count(*) from {{ ref('orders') }}")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{
				"orders": {Columns: []analysis.Column{{Name: "id"}}},
			},
		}
		diags := CheckColumns(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics for expression, got %d: %v", len(diags), diags)
		}
	})

	t.Run("nil scope returns nil", func(t *testing.T) {
		doc := analysis.Document{Text: "select 1"}
		diags := CheckColumns(doc, analysis.DbtContext{})
		if diags != nil {
			t.Errorf("expected nil from nil scope, got %v", diags)
		}
	})

	t.Run("case insensitive match", func(t *testing.T) {
		doc := makeDoc("select ID from {{ ref('orders') }}")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{
				"orders": {Columns: []analysis.Column{{Name: "id"}}},
			},
		}
		diags := CheckColumns(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics (case insensitive), got %d: %v", len(diags), diags)
		}
	})

	t.Run("ref without schema skips", func(t *testing.T) {
		doc := makeDoc("select bogus from {{ ref('orders') }}")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{
				"orders": {Columns: []analysis.Column{}},
			},
		}
		diags := CheckColumns(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics (no schema), got %d: %v", len(diags), diags)
		}
	})

	t.Run("multiple missing columns", func(t *testing.T) {
		doc := makeDoc("select bad1, bad2 from {{ ref('orders') }}")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{
				"orders": {Columns: []analysis.Column{{Name: "id"}}},
			},
		}
		diags := CheckColumns(doc, ctx)
		if len(diags) != 2 {
			t.Fatalf("expected 2 diagnostics, got %d: %v", len(diags), diags)
		}
	})

	t.Run("explicit alias validates expression not alias", func(t *testing.T) {
		doc := makeDoc("select id as my_id from {{ ref('orders') }}")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{
				"orders": {Columns: []analysis.Column{{Name: "id"}}},
			},
		}
		diags := CheckColumns(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics (id exists), got %d: %v", len(diags), diags)
		}
	})

	t.Run("explicit alias with missing column", func(t *testing.T) {
		doc := makeDoc("select bogus as my_id from {{ ref('orders') }}")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{
				"orders": {Columns: []analysis.Column{{Name: "id"}}},
			},
		}
		diags := CheckColumns(doc, ctx)
		if len(diags) != 1 {
			t.Fatalf("expected 1 diagnostic, got %d: %v", len(diags), diags)
		}
		if diags[0].Message != `Column "bogus" not found in any source` {
			t.Errorf("message: got %q", diags[0].Message)
		}
	})

	t.Run("CTE backed by ref", func(t *testing.T) {
		doc := makeDoc("with cte as (select id from {{ ref('orders') }}) select id from cte")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{
				"orders": {Columns: []analysis.Column{{Name: "id"}, {Name: "status"}}},
			},
		}
		diags := CheckColumns(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics (CTE column valid), got %d: %v", len(diags), diags)
		}
	})

	t.Run("mixed known and unknown source", func(t *testing.T) {
		doc := makeDoc("select bogus from {{ ref('orders') }} join raw_table on true")
		ctx := analysis.DbtContext{
			ModelDetailMap: map[string]analysis.ModelDetails{
				"orders": {Columns: []analysis.Column{{Name: "id"}}},
			},
		}
		diags := CheckColumns(doc, ctx)
		if len(diags) != 0 {
			t.Errorf("expected 0 diagnostics (unknown source present), got %d: %v", len(diags), diags)
		}
	})
}
