diff --git c/analysis/parser/parser.go i/analysis/parser/parser.go
index 812a068..4a36e4a 100644
--- c/analysis/parser/parser.go
+++ i/analysis/parser/parser.go
@@ -108,6 +108,7 @@ func (p *Parser) pushCTEScope(cteName string) {
 	}
 	p.scopeStack = append(p.scopeStack, cteScope)
 	p.clauseStack = append(p.clauseStack, ClauseNone)
+	p.selectStarted = false
 }
 
 func (p *Parser) popCTEScope() {
diff --git c/analysis/parser/scope_test.go i/analysis/parser/scope_test.go
index 367f5ea..894f2ca 100644
--- c/analysis/parser/scope_test.go
+++ i/analysis/parser/scope_test.go
@@ -521,17 +521,18 @@ func TestCTEParsing(t *testing.T) {
 			t.Errorf("expected CTE b columns [id], got %v", cteb.Columns)
 		}
 		// CTE b should reference CTE a
-		if cteb.Scope != nil {
-			found := false
-			for _, src := range cteb.Scope.Sources {
-				if src.Name == "a" && src.Kind == SourceKindCTE {
-					found = true
-				}
-			}
-			if !found {
-				t.Error("expected CTE b to have SourceRef for CTE a")
+		if cteb.Scope == nil {
+			t.Fatal("expected CTE b to have non-nil Scope")
+		}
+		found := false
+		for _, src := range cteb.Scope.Sources {
+			if src.Name == "a" && src.Kind == SourceKindCTE {
+				found = true
 			}
 		}
+		if !found {
+			t.Error("expected CTE b to have SourceRef for CTE a")
+		}
 	})
 
 	t.Run("CTE with select star", func(t *testing.T) {
diff --git c/analysis/state.go i/analysis/state.go
index 5580539..2e05ea1 100644
--- c/analysis/state.go
+++ i/analysis/state.go
@@ -11,6 +11,7 @@ import (
 	"github.com/j-clemons/dbt-language-server/analysis/parser"
 	"github.com/j-clemons/dbt-language-server/docs"
 	"github.com/j-clemons/dbt-language-server/lsp"
+	"github.com/j-clemons/dbt-language-server/lsp/completionKind"
 	"github.com/j-clemons/dbt-language-server/util"
 )
 
@@ -33,6 +34,7 @@ type Document struct {
 	Text      string
 	Tokens    *parser.TokenIndex
 	DefTokens map[string]parser.Token
+	Scope     *parser.QueryScope
 }
 
 type DbtContext struct {
@@ -117,6 +119,7 @@ func (s *State) parseDocument(uri, text string) {
 		Text:      text,
 		Tokens:    parserIns.CreateTokenIndex(),
 		DefTokens: parserIns.CreateTokenNameMap(),
+		Scope:     parserIns.CreateQueryScope(),
 	}
 }
 
@@ -420,6 +423,46 @@ func getModelNameFromURI(uri string) string {
 	return ""
 }
 
+func (s *State) buildModelColumns() parser.ModelColumns {
+	mc := make(parser.ModelColumns, len(s.DbtContext.ModelDetailMap))
+	for name, model := range s.DbtContext.ModelDetailMap {
+		cols := make([]parser.ColumnInfo, len(model.Columns))
+		for i, c := range model.Columns {
+			cols[i] = parser.ColumnInfo{Name: c.Name, Description: c.Description}
+		}
+		mc[name] = cols
+	}
+	return mc
+}
+
+func scopeColumnsToCompletionItems(cols []parser.ScopeColumn) []lsp.CompletionItem {
+	seen := make(map[string]bool)
+	items := make([]lsp.CompletionItem, 0, len(cols))
+	for _, col := range cols {
+		if seen[col.Name] {
+			continue
+		}
+		seen[col.Name] = true
+		items = append(items, lsp.CompletionItem{
+			Label:         col.Name,
+			Detail:        fmt.Sprintf("Column from %s", col.Source),
+			Documentation: col.Description,
+			Kind:          completionKind.Field,
+			InsertText:    col.Name,
+			SortText:      col.Name,
+		})
+	}
+	return items
+}
+
+func (s *State) ResolveColumnsAtPosition(uri, aliasPrefix string) []parser.ScopeColumn {
+	doc, exists := s.Documents[uri]
+	if !exists || doc.Scope == nil {
+		return nil
+	}
+	return parser.ResolveColumnsAtCursor(doc.Scope, s.buildModelColumns(), aliasPrefix)
+}
+
 func getReferencedModels(tokens *parser.TokenIndex) []string {
 	if tokens == nil {
 		return nil
@@ -467,8 +510,13 @@ func (s *State) TextDocumentCompletion(id int, uri string, position lsp.Position
 	} else if jinjaBlockTriggerRegex.MatchString(textBeforeCursor) {
 		items = getMacroCompletionItems(s.DbtContext.MacroDetailMap, s.DbtContext.ProjectYaml)
 	} else {
-		refModels := getReferencedModels(s.Documents[uri].Tokens)
-		items = getColumnCompletionItems(refModels, s.DbtContext.ModelDetailMap)
+		scopeCols := s.ResolveColumnsAtPosition(uri, "")
+		if len(scopeCols) > 0 {
+			items = scopeColumnsToCompletionItems(scopeCols)
+		} else {
+			refModels := getReferencedModels(s.Documents[uri].Tokens)
+			items = getColumnCompletionItems(refModels, s.DbtContext.ModelDetailMap)
+		}
 		items = append(items, s.DbtContext.Dialect.FunctionCompletionItems()...)
 	}
 
diff --git c/analysis/state_lsp_test.go i/analysis/state_lsp_test.go
index db19529..4d8c8ff 100644
--- c/analysis/state_lsp_test.go
+++ i/analysis/state_lsp_test.go
@@ -544,3 +544,80 @@ func TestTextDocumentCompletion(t *testing.T) {
 		})
 	}
 }
+
+func TestDocumentScopePopulated(t *testing.T) {
+	state := newTestState()
+	uri := "test://scope.sql"
+
+	tests := []struct {
+		name       string
+		sql        string
+		minSources int
+	}{
+		{"ref produces source", "select id from {{ ref('customers') }}", 1},
+		{"plain SQL", "select 1", 0},
+		{"empty string", "", 0},
+	}
+
+	for _, tc := range tests {
+		t.Run(tc.name, func(t *testing.T) {
+			state.parseDocument(uri, tc.sql)
+			doc := state.Documents[uri]
+			if doc.Scope == nil {
+				t.Fatal("expected non-nil Scope")
+			}
+			if len(doc.Scope.Sources) < tc.minSources {
+				t.Errorf("expected at least %d sources, got %d", tc.minSources, len(doc.Scope.Sources))
+			}
+		})
+	}
+}
+
+func TestResolveColumnsAtPosition(t *testing.T) {
+	state := newTestState()
+	uri := "test://resolve.sql"
+
+	state.parseDocument(uri, "select  from {{ ref('customers') }}")
+	cols := state.ResolveColumnsAtPosition(uri, "")
+	if len(cols) != 2 {
+		t.Fatalf("expected 2 columns, got %d", len(cols))
+	}
+
+	names := map[string]bool{}
+	for _, c := range cols {
+		names[c.Name] = true
+	}
+	if !names["customer_id"] || !names["first_name"] {
+		t.Errorf("expected customer_id and first_name, got %v", cols)
+	}
+}
+
+func TestResolveColumnsAtPositionCTE(t *testing.T) {
+	state := newTestState()
+	uri := "test://cte.sql"
+
+	state.parseDocument(uri, "with cte as (select customer_id from {{ ref('customers') }}) select  from cte")
+	cols := state.ResolveColumnsAtPosition(uri, "")
+	if len(cols) != 1 {
+		t.Fatalf("expected 1 CTE column, got %d: %+v", len(cols), cols)
+	}
+	if cols[0].Name != "customer_id" {
+		t.Errorf("expected 'customer_id', got %q", cols[0].Name)
+	}
+}
+
+func TestResolveColumnsAtPositionAliasFiltered(t *testing.T) {
+	state := newTestState()
+	uri := "test://alias.sql"
+
+	state.parseDocument(uri, "select c. from {{ ref('customers') }} c")
+	cols := state.ResolveColumnsAtPosition(uri, "c")
+	if len(cols) != 2 {
+		t.Fatalf("expected 2 columns for alias 'c', got %d", len(cols))
+	}
+	for _, c := range cols {
+		if c.Source != "c" {
+			t.Errorf("expected source 'c', got %q", c.Source)
+		}
+	}
+}
