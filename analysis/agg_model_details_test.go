package analysis

import (
	"testing"

	"github.com/j-clemons/dbt-language-server/lsp"
)

func TestColumnPositionPropagation(t *testing.T) {
	state := expectedTestState()

	modelMap, _ := state.getModelDetails()

	customers, ok := modelMap["customers"]
	if !ok {
		t.Fatal("model 'customers' not found")
	}

	if len(customers.Columns) < 2 {
		t.Fatalf("expected at least 2 columns, got %d", len(customers.Columns))
	}

	tests := []struct {
		index    int
		name     string
		wantPos  lsp.Position
	}{
		{0, "customer_id", lsp.Position{Line: 7, Character: 14}},
		{1, "first_name", lsp.Position{Line: 13, Character: 14}},
	}

	for _, tt := range tests {
		col := customers.Columns[tt.index]
		if col.Name != tt.name {
			t.Errorf("column[%d].Name = %q, want %q", tt.index, col.Name, tt.name)
		}
		if col.Position != tt.wantPos {
			t.Errorf("column[%d].Position = %v, want %v", tt.index, col.Position, tt.wantPos)
		}
	}
}
