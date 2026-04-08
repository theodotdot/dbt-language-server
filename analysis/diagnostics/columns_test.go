package diagnostics

import (
	"testing"

	"github.com/j-clemons/dbt-language-server/analysis"
)

func TestCheckColumns(t *testing.T) {
	t.Run("column exists no diagnostic", func(t *testing.T) {
		t.Skip("blocked on column lineage engine")
	})

	t.Run("column missing warning", func(t *testing.T) {
		t.Skip("blocked on column lineage engine")
	})

	t.Run("unresolvable CTE skipped", func(t *testing.T) {
		t.Skip("blocked on column lineage engine")
	})

	t.Run("stub returns nil", func(t *testing.T) {
		doc := makeDoc("select id from {{ ref('orders') }}")
		diags := CheckColumns(doc, analysis.DbtContext{})
		if diags != nil {
			t.Errorf("expected nil from stub, got %v", diags)
		}
	})
}
