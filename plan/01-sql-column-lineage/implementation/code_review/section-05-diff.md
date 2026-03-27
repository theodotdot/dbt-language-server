diff --git c/analysis/parser/parser.go i/analysis/parser/parser.go
index 908b7c9..812a068 100644
--- c/analysis/parser/parser.go
+++ i/analysis/parser/parser.go
@@ -78,18 +78,61 @@ func (p *Parser) NextToken() Token {
 func (p *Parser) parseWith() {
 	p.NextToken()
 	if p.curTok.Type == IDENT {
+		cteName := p.curTok.Literal
+		cteToken := p.curTok
 		p.ctes.Ind = true
 		p.ctes.Tokens = append(p.ctes.Tokens, p.curTok)
+		p.registerCTE(cteName, cteToken)
 		if p.peekTok.Type == AS {
 			p.NextToken()
 		}
 		if p.peekTok.Type == LPAREN {
 			p.ctes.ParenCount = 1
+			p.pushCTEScope(cteName)
 		}
 		p.NextToken()
 	}
 }
 
+func (p *Parser) registerCTE(name string, tok Token) {
+	topScope := p.scopeStack[0]
+	if _, exists := topScope.CTEs[name]; !exists {
+		topScope.CTEs[name] = &CTEDef{Name: name, Token: tok}
+	}
+}
+
+func (p *Parser) pushCTEScope(cteName string) {
+	cteScope := NewQueryScope(p.scopeStack[0])
+	if cteDef, ok := p.scopeStack[0].CTEs[cteName]; ok {
+		cteDef.Scope = cteScope
+	}
+	p.scopeStack = append(p.scopeStack, cteScope)
+	p.clauseStack = append(p.clauseStack, ClauseNone)
+}
+
+func (p *Parser) popCTEScope() {
+	if len(p.scopeStack) <= 1 {
+		return
+	}
+	cteScope := p.scopeStack[len(p.scopeStack)-1]
+	p.scopeStack = p.scopeStack[:len(p.scopeStack)-1]
+	p.clauseStack = p.clauseStack[:len(p.clauseStack)-1]
+
+	// Extract columns from CTE's SelectItems
+	for _, cteDef := range p.scopeStack[0].CTEs {
+		if cteDef.Scope == cteScope {
+			for _, item := range cteScope.SelectItems {
+				if item.IsStar {
+					cteDef.Columns = append(cteDef.Columns, "*")
+				} else if item.Alias != "" {
+					cteDef.Columns = append(cteDef.Columns, item.Alias)
+				}
+			}
+			break
+		}
+	}
+}
+
 func (p *Parser) parseRef() {
 	p.NextToken()
 	if p.curTok.Type == LPAREN {
@@ -399,14 +442,33 @@ func (p *Parser) parseTokens() {
 			}
 			p.decParenCount()
 			if p.ctes.Ind && p.ctes.ParenCount == 0 {
+				// Finalize any pending select item in the CTE scope
+				if p.currentClause() == ClauseSelect {
+					p.finalizeSelectItem()
+				}
+				// Pop CTE scope and extract columns
+				p.popCTEScope()
+
 				p.NextToken()
 				if p.curTok.Type == COMMA {
 					p.NextToken()
 					if p.curTok.Type == IDENT {
+						cteName := p.curTok.Literal
+						cteToken := p.curTok
 						p.ctes.Tokens = append(p.ctes.Tokens, p.curTok)
+						p.registerCTE(cteName, cteToken)
+						if p.peekTok.Type == AS {
+							p.NextToken()
+						}
+						if p.peekTok.Type == LPAREN {
+							p.ctes.ParenCount = 1
+							p.pushCTEScope(cteName)
+						}
+						p.NextToken()
 					}
 				} else {
 					p.ctes.Ind = false
+					continue // re-process curTok in main switch
 				}
 			}
 		case SOURCE:
@@ -459,7 +521,7 @@ func (p *Parser) parseTokens() {
 					p.lastSourceRef.Alias = p.curTok.Literal
 				} else {
 					kind := SourceKindTable
-					if _, ok := p.currentScope().CTEs[p.curTok.Literal]; ok {
+					if _, ok := p.scopeStack[0].CTEs[p.curTok.Literal]; ok {
 						kind = SourceKindCTE
 					}
 					p.addSourceRef(kind, p.curTok.Literal, "", p.curTok)
diff --git c/analysis/parser/scope_test.go i/analysis/parser/scope_test.go
index c3cd551..367f5ea 100644
--- c/analysis/parser/scope_test.go
+++ i/analysis/parser/scope_test.go
@@ -475,3 +475,143 @@ func TestParseFromSources(t *testing.T) {
 		})
 	}
 }
+
+func TestCTEParsing(t *testing.T) {
+	t.Run("single CTE", func(t *testing.T) {
+		input := "with cte as (select id, name from {{ ref('orders') }}) select * from cte"
+		p := Parse(input, docs.Dialect("snowflake"))
+		scope := p.CreateQueryScope()
+
+		cteDef, ok := scope.CTEs["cte"]
+		if !ok {
+			t.Fatal("expected CTE 'cte' in scope")
+		}
+		if len(cteDef.Columns) != 2 {
+			t.Fatalf("expected 2 columns, got %d: %v", len(cteDef.Columns), cteDef.Columns)
+		}
+		if cteDef.Columns[0] != "id" || cteDef.Columns[1] != "name" {
+			t.Errorf("expected columns [id, name], got %v", cteDef.Columns)
+		}
+
+		// Main scope should have CTE source ref
+		found := false
+		for _, src := range scope.Sources {
+			if src.Name == "cte" && src.Kind == SourceKindCTE {
+				found = true
+			}
+		}
+		if !found {
+			t.Error("expected SourceRef with Kind=SourceKindCTE for 'cte'")
+		}
+	})
+
+	t.Run("chained CTEs", func(t *testing.T) {
+		input := "with a as (select id from {{ ref('orders') }}), b as (select id from a) select * from b"
+		p := Parse(input, docs.Dialect("snowflake"))
+		scope := p.CreateQueryScope()
+
+		if _, ok := scope.CTEs["a"]; !ok {
+			t.Fatal("expected CTE 'a'")
+		}
+		cteb, ok := scope.CTEs["b"]
+		if !ok {
+			t.Fatal("expected CTE 'b'")
+		}
+		if len(cteb.Columns) != 1 || cteb.Columns[0] != "id" {
+			t.Errorf("expected CTE b columns [id], got %v", cteb.Columns)
+		}
+		// CTE b should reference CTE a
+		if cteb.Scope != nil {
+			found := false
+			for _, src := range cteb.Scope.Sources {
+				if src.Name == "a" && src.Kind == SourceKindCTE {
+					found = true
+				}
+			}
+			if !found {
+				t.Error("expected CTE b to have SourceRef for CTE a")
+			}
+		}
+	})
+
+	t.Run("CTE with select star", func(t *testing.T) {
+		input := "with cte as (select * from {{ ref('orders') }}) select * from cte"
+		p := Parse(input, docs.Dialect("snowflake"))
+		scope := p.CreateQueryScope()
+
+		cteDef, ok := scope.CTEs["cte"]
+		if !ok {
+			t.Fatal("expected CTE 'cte'")
+		}
+		if len(cteDef.Columns) != 1 || cteDef.Columns[0] != "*" {
+			t.Errorf("expected columns [*], got %v", cteDef.Columns)
+		}
+	})
+
+	t.Run("CTE with aliased columns", func(t *testing.T) {
+		input := "with cte as (select id as order_id, name as customer_name from {{ ref('orders') }}) select * from cte"
+		p := Parse(input, docs.Dialect("snowflake"))
+		scope := p.CreateQueryScope()
+
+		cteDef, ok := scope.CTEs["cte"]
+		if !ok {
+			t.Fatal("expected CTE 'cte'")
+		}
+		if len(cteDef.Columns) != 2 {
+			t.Fatalf("expected 2 columns, got %d: %v", len(cteDef.Columns), cteDef.Columns)
+		}
+		if cteDef.Columns[0] != "order_id" || cteDef.Columns[1] != "customer_name" {
+			t.Errorf("expected [order_id, customer_name], got %v", cteDef.Columns)
+		}
+	})
+
+	t.Run("backward compat - old CTE struct", func(t *testing.T) {
+		input := "with cte1 as (select id from {{ ref('orders') }}), cte2 as (select id from cte1) select * from cte2"
+		p := Parse(input, docs.Dialect("snowflake"))
+		tokenMap := p.CreateTokenNameMap()
+
+		if _, ok := tokenMap["cte1"]; !ok {
+			t.Error("expected cte1 in CreateTokenNameMap")
+		}
+		if _, ok := tokenMap["cte2"]; !ok {
+			t.Error("expected cte2 in CreateTokenNameMap")
+		}
+	})
+
+	t.Run("CTE body does not corrupt main scope", func(t *testing.T) {
+		input := "with cte as (select id from t) select name from {{ ref('orders') }}"
+		p := Parse(input, docs.Dialect("snowflake"))
+		scope := p.CreateQueryScope()
+
+		// Main scope should have SelectItems from the outer SELECT
+		if len(scope.SelectItems) != 1 {
+			t.Fatalf("expected 1 SelectItem in main scope, got %d", len(scope.SelectItems))
+		}
+		if scope.SelectItems[0].Alias != "name" {
+			t.Errorf("expected main scope SelectItem alias 'name', got %q", scope.SelectItems[0].Alias)
+		}
+	})
+}
+
+func TestCTEErrorRecovery(t *testing.T) {
+	t.Run("missing closing paren", func(t *testing.T) {
+		input := "with cte as (select id from t"
+		p := Parse(input, docs.Dialect("snowflake"))
+		scope := p.CreateQueryScope()
+		if scope == nil {
+			t.Fatal("expected non-nil scope")
+		}
+	})
+
+	t.Run("jinja only input", func(t *testing.T) {
+		input := "{{ ref('orders') }}"
+		p := Parse(input, docs.Dialect("snowflake"))
+		scope := p.CreateQueryScope()
+		if scope == nil {
+			t.Fatal("expected non-nil scope")
+		}
+		if len(scope.CTEs) != 0 {
+			t.Errorf("expected no CTEs, got %d", len(scope.CTEs))
+		}
+	})
+}
