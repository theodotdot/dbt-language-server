package parser

import "testing"

func TestResolveColumnsFromRef(t *testing.T) {
	scope := &QueryScope{
		Sources: []*SourceRef{
			{Kind: SourceKindRef, Name: "orders"},
		},
		CTEs: make(map[string]*CTEDef),
	}
	modelCols := ModelColumns{
		"orders": {
			{Name: "id", Description: "primary key"},
			{Name: "status"},
			{Name: "amount"},
		},
	}

	cols := ResolveColumnsAtCursor(scope, modelCols, "")
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}
	if cols[0].Name != "id" || cols[0].Source != "orders" || cols[0].Qualified != "orders.id" {
		t.Errorf("unexpected col[0]: %+v", cols[0])
	}
	if cols[0].Description != "primary key" {
		t.Errorf("expected description 'primary key', got %q", cols[0].Description)
	}
}

func TestResolveColumnsFromCTE(t *testing.T) {
	scope := &QueryScope{
		Sources: []*SourceRef{
			{Kind: SourceKindCTE, Name: "my_cte"},
		},
		CTEs: map[string]*CTEDef{
			"my_cte": {Name: "my_cte", Columns: []string{"id", "total"}},
		},
	}

	cols := ResolveColumnsAtCursor(scope, nil, "")
	if len(cols) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(cols))
	}
	if cols[0].Source != "my_cte" {
		t.Errorf("expected source 'my_cte', got %q", cols[0].Source)
	}
	if cols[1].Name != "total" {
		t.Errorf("expected column 'total', got %q", cols[1].Name)
	}
}

func TestResolveCTEStarExpansion(t *testing.T) {
	cteScope := &QueryScope{
		Sources: []*SourceRef{
			{Kind: SourceKindRef, Name: "orders"},
		},
		CTEs: make(map[string]*CTEDef),
	}
	scope := &QueryScope{
		Sources: []*SourceRef{
			{Kind: SourceKindCTE, Name: "cte"},
		},
		CTEs: map[string]*CTEDef{
			"cte": {Name: "cte", Columns: []string{"*"}, Scope: cteScope},
		},
	}
	modelCols := ModelColumns{
		"orders": {
			{Name: "id"},
			{Name: "status"},
		},
	}

	cols := ResolveColumnsAtCursor(scope, modelCols, "")
	if len(cols) != 2 {
		t.Fatalf("expected 2 columns from star expansion, got %d", len(cols))
	}
	if cols[0].Source != "cte" {
		t.Errorf("expected source 'cte', got %q", cols[0].Source)
	}
}

func TestResolveAliasQualified(t *testing.T) {
	scope := &QueryScope{
		Sources: []*SourceRef{
			{Kind: SourceKindRef, Name: "orders", Alias: "o"},
			{Kind: SourceKindRef, Name: "customers", Alias: "c"},
		},
		CTEs: make(map[string]*CTEDef),
	}
	modelCols := ModelColumns{
		"orders":    {{Name: "id"}, {Name: "amount"}},
		"customers": {{Name: "id"}, {Name: "name"}},
	}

	cols := ResolveColumnsAtCursor(scope, modelCols, "o")
	if len(cols) != 2 {
		t.Fatalf("expected 2 columns for alias 'o', got %d", len(cols))
	}
	if cols[0].Qualified != "o.id" {
		t.Errorf("expected qualified 'o.id', got %q", cols[0].Qualified)
	}
}

func TestResolveMultipleSourcesMerged(t *testing.T) {
	scope := &QueryScope{
		Sources: []*SourceRef{
			{Kind: SourceKindRef, Name: "orders", Alias: "o"},
			{Kind: SourceKindRef, Name: "customers", Alias: "c"},
		},
		CTEs: make(map[string]*CTEDef),
	}
	modelCols := ModelColumns{
		"orders":    {{Name: "id"}, {Name: "amount"}},
		"customers": {{Name: "id"}, {Name: "name"}},
	}

	cols := ResolveColumnsAtCursor(scope, modelCols, "")
	if len(cols) != 4 {
		t.Fatalf("expected 4 columns total, got %d", len(cols))
	}
}

func TestResolveUnknownTableSource(t *testing.T) {
	scope := &QueryScope{
		Sources: []*SourceRef{
			{Kind: SourceKindTable, Name: "some_table"},
		},
		CTEs: make(map[string]*CTEDef),
	}

	cols := ResolveColumnsAtCursor(scope, nil, "")
	if len(cols) != 0 {
		t.Fatalf("expected 0 columns for unknown table, got %d", len(cols))
	}
}

func TestResolveSourceKindSource(t *testing.T) {
	scope := &QueryScope{
		Sources: []*SourceRef{
			{Kind: SourceKindSource, Name: "payments", SourceName: "stripe"},
		},
		CTEs: make(map[string]*CTEDef),
	}

	cols := ResolveColumnsAtCursor(scope, nil, "")
	if len(cols) != 0 {
		t.Fatalf("expected 0 columns for source kind, got %d", len(cols))
	}
}

func TestResolveNilScope(t *testing.T) {
	cols := ResolveColumnsAtCursor(nil, nil, "")
	if cols != nil {
		t.Fatalf("expected nil for nil scope, got %v", cols)
	}
}

func TestResolveQualifiedAndUnqualifiedForms(t *testing.T) {
	scope := &QueryScope{
		Sources: []*SourceRef{
			{Kind: SourceKindRef, Name: "orders", Alias: "o"},
		},
		CTEs: make(map[string]*CTEDef),
	}
	modelCols := ModelColumns{
		"orders": {{Name: "id"}},
	}

	cols := ResolveColumnsAtCursor(scope, modelCols, "")
	if len(cols) != 1 {
		t.Fatalf("expected 1 column, got %d", len(cols))
	}
	if cols[0].Name != "id" || cols[0].Source != "o" || cols[0].Qualified != "o.id" {
		t.Errorf("unexpected column: %+v", cols[0])
	}
}

func TestResolveSelfJoin(t *testing.T) {
	scope := &QueryScope{
		Sources: []*SourceRef{
			{Kind: SourceKindRef, Name: "events", Alias: "e1"},
			{Kind: SourceKindRef, Name: "events", Alias: "e2"},
		},
		CTEs: make(map[string]*CTEDef),
	}
	modelCols := ModelColumns{
		"events": {{Name: "id"}, {Name: "type"}},
	}

	cols := ResolveColumnsAtCursor(scope, modelCols, "")
	if len(cols) != 4 {
		t.Fatalf("expected 4 columns (2 per alias), got %d", len(cols))
	}

	// Check alias differentiation
	e1Cols := ResolveColumnsAtCursor(scope, modelCols, "e1")
	if len(e1Cols) != 2 {
		t.Fatalf("expected 2 columns for e1, got %d", len(e1Cols))
	}
	if e1Cols[0].Source != "e1" {
		t.Errorf("expected source 'e1', got %q", e1Cols[0].Source)
	}
}
