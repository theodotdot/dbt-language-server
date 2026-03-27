diff --git c/analysis/parser/resolve.go i/analysis/parser/resolve.go
new file mode 100644
index 0000000..1f975ca
--- /dev/null
+++ i/analysis/parser/resolve.go
@@ -0,0 +1,124 @@
+package parser
+
+// ColumnInfo represents a column from a model or source schema.
+type ColumnInfo struct {
+	Name        string
+	Description string
+}
+
+// ModelColumns maps model name to its column definitions.
+type ModelColumns map[string][]ColumnInfo
+
+// ResolveColumnsAtCursor returns all columns available at the given cursor
+// position, filtered by aliasPrefix if non-empty.
+func ResolveColumnsAtCursor(
+	scope *QueryScope,
+	modelCols ModelColumns,
+	aliasPrefix string,
+) []ScopeColumn {
+	if scope == nil {
+		return nil
+	}
+
+	if aliasPrefix != "" {
+		src := scope.FindSourceByAlias(aliasPrefix)
+		if src == nil {
+			return nil
+		}
+		return resolveSourceColumns(scope, src, modelCols)
+	}
+
+	var cols []ScopeColumn
+	for _, src := range scope.Sources {
+		cols = append(cols, resolveSourceColumns(scope, src, modelCols)...)
+	}
+	return cols
+}
+
+func resolveSourceColumns(scope *QueryScope, src *SourceRef, modelCols ModelColumns) []ScopeColumn {
+	sourceLabel := src.Alias
+	if sourceLabel == "" {
+		sourceLabel = src.Name
+	}
+
+	switch src.Kind {
+	case SourceKindRef:
+		return buildScopeColumns(modelCols[src.Name], sourceLabel)
+	case SourceKindCTE:
+		return resolveCTEColumns(scope, src.Name, modelCols, make(map[string]bool))
+	case SourceKindSource, SourceKindTable:
+		return nil
+	}
+	return nil
+}
+
+func resolveCTEColumns(
+	scope *QueryScope,
+	cteName string,
+	modelCols ModelColumns,
+	visited map[string]bool,
+) []ScopeColumn {
+	if visited[cteName] {
+		return nil
+	}
+	visited[cteName] = true
+
+	cteDef, ok := scope.CTEs[cteName]
+	if !ok {
+		return nil
+	}
+
+	sourceLabel := cteName
+	var cols []ScopeColumn
+
+	for _, col := range cteDef.Columns {
+		if col == "*" {
+			// Expand star from CTE's internal scope
+			if cteDef.Scope != nil {
+				for _, innerSrc := range cteDef.Scope.Sources {
+					switch innerSrc.Kind {
+					case SourceKindRef:
+						for _, ci := range modelCols[innerSrc.Name] {
+							cols = append(cols, ScopeColumn{
+								Name:        ci.Name,
+								Source:      sourceLabel,
+								Qualified:   sourceLabel + "." + ci.Name,
+								Description: ci.Description,
+							})
+						}
+					case SourceKindCTE:
+						expanded := resolveCTEColumns(scope, innerSrc.Name, modelCols, visited)
+						for i := range expanded {
+							expanded[i].Source = sourceLabel
+							expanded[i].Qualified = sourceLabel + "." + expanded[i].Name
+						}
+						cols = append(cols, expanded...)
+					}
+				}
+			}
+		} else {
+			cols = append(cols, ScopeColumn{
+				Name:      col,
+				Source:    sourceLabel,
+				Qualified: sourceLabel + "." + col,
+			})
+		}
+	}
+	return cols
+}
+
+func buildScopeColumns(infos []ColumnInfo, sourceLabel string) []ScopeColumn {
+	if len(infos) == 0 {
+		return nil
+	}
+	cols := make([]ScopeColumn, len(infos))
+	for i, ci := range infos {
+		cols[i] = ScopeColumn{
+			Name:        ci.Name,
+			Source:      sourceLabel,
+			Qualified:   sourceLabel + "." + ci.Name,
+			Description: ci.Description,
+		}
+	}
+	return cols
+}
diff --git c/analysis/parser/resolve_test.go i/analysis/parser/resolve_test.go
new file mode 100644
index 0000000..3fb9101
--- /dev/null
+++ i/analysis/parser/resolve_test.go
@@ -0,0 +1,206 @@
+package parser
+
+import "testing"
+
+func TestResolveColumnsFromRef(t *testing.T) {
+	scope := &QueryScope{
+		Sources: []*SourceRef{
+			{Kind: SourceKindRef, Name: "orders"},
+		},
+		CTEs: make(map[string]*CTEDef),
+	}
+	modelCols := ModelColumns{
+		"orders": {
+			{Name: "id", Description: "primary key"},
+			{Name: "status"},
+			{Name: "amount"},
+		},
+	}
+
+	cols := ResolveColumnsAtCursor(scope, modelCols, "")
+	if len(cols) != 3 {
+		t.Fatalf("expected 3 columns, got %d", len(cols))
+	}
+	if cols[0].Name != "id" || cols[0].Source != "orders" || cols[0].Qualified != "orders.id" {
+		t.Errorf("unexpected col[0]: %+v", cols[0])
+	}
+	if cols[0].Description != "primary key" {
+		t.Errorf("expected description 'primary key', got %q", cols[0].Description)
+	}
+}
+
+func TestResolveColumnsFromCTE(t *testing.T) {
+	scope := &QueryScope{
+		Sources: []*SourceRef{
+			{Kind: SourceKindCTE, Name: "my_cte"},
+		},
+		CTEs: map[string]*CTEDef{
+			"my_cte": {Name: "my_cte", Columns: []string{"id", "total"}},
+		},
+	}
+
+	cols := ResolveColumnsAtCursor(scope, nil, "")
+	if len(cols) != 2 {
+		t.Fatalf("expected 2 columns, got %d", len(cols))
+	}
+	if cols[0].Source != "my_cte" {
+		t.Errorf("expected source 'my_cte', got %q", cols[0].Source)
+	}
+	if cols[1].Name != "total" {
+		t.Errorf("expected column 'total', got %q", cols[1].Name)
+	}
+}
+
+func TestResolveCTEStarExpansion(t *testing.T) {
+	cteScope := &QueryScope{
+		Sources: []*SourceRef{
+			{Kind: SourceKindRef, Name: "orders"},
+		},
+		CTEs: make(map[string]*CTEDef),
+	}
+	scope := &QueryScope{
+		Sources: []*SourceRef{
+			{Kind: SourceKindCTE, Name: "cte"},
+		},
+		CTEs: map[string]*CTEDef{
+			"cte": {Name: "cte", Columns: []string{"*"}, Scope: cteScope},
+		},
+	}
+	modelCols := ModelColumns{
+		"orders": {
+			{Name: "id"},
+			{Name: "status"},
+		},
+	}
+
+	cols := ResolveColumnsAtCursor(scope, modelCols, "")
+	if len(cols) != 2 {
+		t.Fatalf("expected 2 columns from star expansion, got %d", len(cols))
+	}
+	if cols[0].Source != "cte" {
+		t.Errorf("expected source 'cte', got %q", cols[0].Source)
+	}
+}
+
+func TestResolveAliasQualified(t *testing.T) {
+	scope := &QueryScope{
+		Sources: []*SourceRef{
+			{Kind: SourceKindRef, Name: "orders", Alias: "o"},
+			{Kind: SourceKindRef, Name: "customers", Alias: "c"},
+		},
+		CTEs: make(map[string]*CTEDef),
+	}
+	modelCols := ModelColumns{
+		"orders":    {{Name: "id"}, {Name: "amount"}},
+		"customers": {{Name: "id"}, {Name: "name"}},
+	}
+
+	cols := ResolveColumnsAtCursor(scope, modelCols, "o")
+	if len(cols) != 2 {
+		t.Fatalf("expected 2 columns for alias 'o', got %d", len(cols))
+	}
+	if cols[0].Qualified != "o.id" {
+		t.Errorf("expected qualified 'o.id', got %q", cols[0].Qualified)
+	}
+}
+
+func TestResolveMultipleSourcesMerged(t *testing.T) {
+	scope := &QueryScope{
+		Sources: []*SourceRef{
+			{Kind: SourceKindRef, Name: "orders", Alias: "o"},
+			{Kind: SourceKindRef, Name: "customers", Alias: "c"},
+		},
+		CTEs: make(map[string]*CTEDef),
+	}
+	modelCols := ModelColumns{
+		"orders":    {{Name: "id"}, {Name: "amount"}},
+		"customers": {{Name: "id"}, {Name: "name"}},
+	}
+
+	cols := ResolveColumnsAtCursor(scope, modelCols, "")
+	if len(cols) != 4 {
+		t.Fatalf("expected 4 columns total, got %d", len(cols))
+	}
+}
+
+func TestResolveUnknownTableSource(t *testing.T) {
+	scope := &QueryScope{
+		Sources: []*SourceRef{
+			{Kind: SourceKindTable, Name: "some_table"},
+		},
+		CTEs: make(map[string]*CTEDef),
+	}
+
+	cols := ResolveColumnsAtCursor(scope, nil, "")
+	if len(cols) != 0 {
+		t.Fatalf("expected 0 columns for unknown table, got %d", len(cols))
+	}
+}
+
+func TestResolveSourceKindSource(t *testing.T) {
+	scope := &QueryScope{
+		Sources: []*SourceRef{
+			{Kind: SourceKindSource, Name: "payments", SourceName: "stripe"},
+		},
+		CTEs: make(map[string]*CTEDef),
+	}
+
+	cols := ResolveColumnsAtCursor(scope, nil, "")
+	if len(cols) != 0 {
+		t.Fatalf("expected 0 columns for source kind, got %d", len(cols))
+	}
+}
+
+func TestResolveNilScope(t *testing.T) {
+	cols := ResolveColumnsAtCursor(nil, nil, "")
+	if cols != nil {
+		t.Fatalf("expected nil for nil scope, got %v", cols)
+	}
+}
+
+func TestResolveQualifiedAndUnqualifiedForms(t *testing.T) {
+	scope := &QueryScope{
+		Sources: []*SourceRef{
+			{Kind: SourceKindRef, Name: "orders", Alias: "o"},
+		},
+		CTEs: make(map[string]*CTEDef),
+	}
+	modelCols := ModelColumns{
+		"orders": {{Name: "id"}},
+	}
+
+	cols := ResolveColumnsAtCursor(scope, modelCols, "")
+	if len(cols) != 1 {
+		t.Fatalf("expected 1 column, got %d", len(cols))
+	}
+	if cols[0].Name != "id" || cols[0].Source != "o" || cols[0].Qualified != "o.id" {
+		t.Errorf("unexpected column: %+v", cols[0])
+	}
+}
+
+func TestResolveSelfJoin(t *testing.T) {
+	scope := &QueryScope{
+		Sources: []*SourceRef{
+			{Kind: SourceKindRef, Name: "events", Alias: "e1"},
+			{Kind: SourceKindRef, Name: "events", Alias: "e2"},
+		},
+		CTEs: make(map[string]*CTEDef),
+	}
+	modelCols := ModelColumns{
+		"events": {{Name: "id"}, {Name: "type"}},
+	}
+
+	cols := ResolveColumnsAtCursor(scope, modelCols, "")
+	if len(cols) != 4 {
+		t.Fatalf("expected 4 columns (2 per alias), got %d", len(cols))
+	}
+
+	// Check alias differentiation
+	e1Cols := ResolveColumnsAtCursor(scope, modelCols, "e1")
+	if len(e1Cols) != 2 {
+		t.Fatalf("expected 2 columns for e1, got %d", len(e1Cols))
+	}
+	if e1Cols[0].Source != "e1" {
+		t.Errorf("expected source 'e1', got %q", e1Cols[0].Source)
+	}
+}
