package parser

import (
	"testing"

	"github.com/j-clemons/dbt-language-server/docs"
)

func TestNewQueryScope(t *testing.T) {
	t.Run("initializes maps", func(t *testing.T) {
		scope := NewQueryScope(nil)
		if scope.CTEs == nil {
			t.Fatal("CTEs map should be initialized")
		}
		if scope.ClauseRanges == nil {
			t.Fatal("ClauseRanges map should be initialized")
		}
		if scope.Parent != nil {
			t.Fatal("Parent should be nil when no parent provided")
		}
	})

	t.Run("sets parent", func(t *testing.T) {
		parent := NewQueryScope(nil)
		child := NewQueryScope(parent)
		if child.Parent != parent {
			t.Fatal("Parent should be set")
		}
	})
}

func TestFindSourceByAlias(t *testing.T) {
	tests := []struct {
		name      string
		sources   []*SourceRef
		alias     string
		wantName  string
		wantAlias string
	}{
		{
			name: "alias match",
			sources: []*SourceRef{
				{Kind: SourceKindRef, Name: "orders", Alias: "o"},
			},
			alias:    "o",
			wantName: "orders",
		},
		{
			name: "fallback to name match",
			sources: []*SourceRef{
				{Kind: SourceKindRef, Name: "orders"},
			},
			alias:    "orders",
			wantName: "orders",
		},
		{
			name: "no match returns nil",
			sources: []*SourceRef{
				{Kind: SourceKindRef, Name: "orders", Alias: "o"},
			},
			alias:    "customers",
			wantName: "",
		},
		{
			name: "self-join different aliases",
			sources: []*SourceRef{
				{Kind: SourceKindRef, Name: "events", Alias: "e1"},
				{Kind: SourceKindRef, Name: "events", Alias: "e2"},
			},
			alias:     "e2",
			wantName:  "events",
			wantAlias: "e2",
		},
		{
			name:     "empty sources",
			sources:  nil,
			alias:    "anything",
			wantName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := &QueryScope{Sources: tt.sources}
			got := scope.FindSourceByAlias(tt.alias)
			if tt.wantName == "" {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected source with name %q, got nil", tt.wantName)
			}
			if got.Name != tt.wantName {
				t.Fatalf("expected name %q, got %q", tt.wantName, got.Name)
			}
			if tt.wantAlias != "" && got.Alias != tt.wantAlias {
				t.Fatalf("expected alias %q, got %q", tt.wantAlias, got.Alias)
			}
		})
	}
}

func TestClauseStateMachine(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantClauses []ClauseKind
	}{
		{
			name:        "select sets ClauseSelect",
			input:       "select id from t",
			wantClauses: []ClauseKind{ClauseSelect},
		},
		{
			name:        "from sets ClauseFrom",
			input:       "select id from t",
			wantClauses: []ClauseKind{ClauseFrom},
		},
		{
			name:        "join sets ClauseJoin",
			input:       "select id from t join t2 on t.id = t2.id",
			wantClauses: []ClauseKind{ClauseJoin, ClauseOn},
		},
		{
			name:        "where sets ClauseWhere",
			input:       "select id from t where id = 1",
			wantClauses: []ClauseKind{ClauseWhere},
		},
		{
			name:        "all major clauses present",
			input:       "select id, name from orders where id > 0 group by name order by id having count(*) > 1",
			wantClauses: []ClauseKind{ClauseSelect, ClauseFrom, ClauseWhere, ClauseGroupBy, ClauseOrderBy, ClauseHaving},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Parse(tt.input, docs.Dialect("snowflake"))
			scope := p.CreateQueryScope()
			for _, clause := range tt.wantClauses {
				if _, ok := scope.ClauseRanges[clause]; !ok {
					t.Errorf("expected ClauseRanges to contain %d", clause)
				}
			}
		})
	}
}

func TestClauseRangesMonotonicallyIncreasing(t *testing.T) {
	p := Parse("select id, name from orders where id > 0 group by name order by id having count(*) > 1", docs.Dialect("snowflake"))
	scope := p.CreateQueryScope()

	order := []ClauseKind{ClauseSelect, ClauseFrom, ClauseWhere, ClauseGroupBy, ClauseOrderBy, ClauseHaving}
	prevCol := -1
	for _, clause := range order {
		tok, ok := scope.ClauseRanges[clause]
		if !ok {
			t.Fatalf("missing clause %d", clause)
		}
		if tok.Column <= prevCol {
			t.Errorf("clause %d at column %d is not after previous column %d", clause, tok.Column, prevCol)
		}
		prevCol = tok.Column
	}
}

func TestSubqueryDoesNotCorruptOuterClause(t *testing.T) {
	input := "select id from t where id in (select id from t2)"
	//        0123456789...  "from" starts at column 10
	p := Parse(input, docs.Dialect("snowflake"))
	scope := p.CreateQueryScope()

	fromTok, ok := scope.ClauseRanges[ClauseFrom]
	if !ok {
		t.Fatal("expected ClauseFrom in ClauseRanges")
	}
	// Outer "from" is at column 10, subquery "from" is at column 40
	if fromTok.Column != 10 {
		t.Errorf("expected outer FROM at column 10, got column %d (subquery may have overwritten it)", fromTok.Column)
	}
	whereTok, ok := scope.ClauseRanges[ClauseWhere]
	if !ok {
		t.Fatal("expected ClauseWhere in ClauseRanges")
	}
	if fromTok.Column >= whereTok.Column {
		t.Errorf("FROM (col %d) should be before WHERE (col %d)", fromTok.Column, whereTok.Column)
	}
}

func TestEmptyInputReturnsEmptyScope(t *testing.T) {
	p := Parse("", docs.Dialect("snowflake"))
	scope := p.CreateQueryScope()
	if scope == nil {
		t.Fatal("expected non-nil scope")
	}
	if len(scope.Sources) != 0 {
		t.Errorf("expected empty Sources, got %d", len(scope.Sources))
	}
	if len(scope.SelectItems) != 0 {
		t.Errorf("expected empty SelectItems, got %d", len(scope.SelectItems))
	}
	if len(scope.ClauseRanges) != 0 {
		t.Errorf("expected empty ClauseRanges, got %d", len(scope.ClauseRanges))
	}
}

func TestCreateQueryScopeReturnsNonNil(t *testing.T) {
	p := Parse("select 1", docs.Dialect("snowflake"))
	scope := p.CreateQueryScope()
	if scope == nil {
		t.Fatal("expected non-nil scope from CreateQueryScope")
	}
}

func TestParseSelectItems(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []SelectItem
	}{
		{
			name:  "simple select list",
			input: "select id, name from t",
			expected: []SelectItem{
				{Alias: "id"},
				{Alias: "name"},
			},
		},
		{
			name:  "explicit AS alias",
			input: "select id as order_id from t",
			expected: []SelectItem{
				{Alias: "order_id"},
			},
		},
		{
			name:  "implicit alias",
			input: "select id order_id from t",
			expected: []SelectItem{
				{Alias: "order_id"},
			},
		},
		{
			name:  "select star",
			input: "select * from t",
			expected: []SelectItem{
				{IsStar: true, StarSource: ""},
			},
		},
		{
			name:  "qualified star",
			input: "select o.* from orders o",
			expected: []SelectItem{
				{IsStar: true, StarSource: "o"},
			},
		},
		{
			name:  "qualified column",
			input: "select o.id from orders o",
			expected: []SelectItem{
				{Source: "o", Alias: "id"},
			},
		},
		{
			name:  "function call not treated as alias",
			input: "select coalesce(a, b) as c from t",
			expected: []SelectItem{
				{Alias: "c"},
			},
		},
		{
			name:  "count star",
			input: "select count(*) as total from t",
			expected: []SelectItem{
				{Alias: "total", IsStar: false},
			},
		},
		{
			name:  "case expression",
			input: "select case when x then y end as z from t",
			expected: []SelectItem{
				{Alias: "z"},
			},
		},
		{
			name:  "subquery select skipped",
			input: "select id from t where x in (select id from other)",
			expected: []SelectItem{
				{Alias: "id"},
			},
		},
		{
			name:  "multiple expressions with aliases",
			input: "select a + b as sum, c * d as product from t",
			expected: []SelectItem{
				{Alias: "sum"},
				{Alias: "product"},
			},
		},
		{
			name:  "distinct keyword skipped",
			input: "select distinct id, name from t",
			expected: []SelectItem{
				{Alias: "id"},
				{Alias: "name"},
			},
		},
		{
			name:  "nested parens in function",
			input: "select trim(upper(name)) as clean_name from t",
			expected: []SelectItem{
				{Alias: "clean_name"},
			},
		},
		{
			name:  "window function",
			input: "select row_number() over (partition by x order by y) as rn from t",
			expected: []SelectItem{
				{Alias: "rn"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Parse(tt.input, docs.Dialect("snowflake"))
			scope := p.CreateQueryScope()
			if scope == nil {
				t.Fatal("expected non-nil scope")
			}
			if len(scope.SelectItems) != len(tt.expected) {
				t.Fatalf("expected %d SelectItems, got %d: %+v", len(tt.expected), len(scope.SelectItems), scope.SelectItems)
			}
			for i, exp := range tt.expected {
				got := scope.SelectItems[i]
				if exp.Alias != "" && got.Alias != exp.Alias {
					t.Errorf("item[%d] alias: expected %q, got %q", i, exp.Alias, got.Alias)
				}
				if got.IsStar != exp.IsStar {
					t.Errorf("item[%d] IsStar: expected %v, got %v", i, exp.IsStar, got.IsStar)
				}
				if exp.StarSource != "" && got.StarSource != exp.StarSource {
					t.Errorf("item[%d] StarSource: expected %q, got %q", i, exp.StarSource, got.StarSource)
				}
				if exp.Source != "" && got.Source != exp.Source {
					t.Errorf("item[%d] Source: expected %q, got %q", i, exp.Source, got.Source)
				}
			}
		})
	}
}

func TestParseFromSources(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []SourceRef
	}{
		{
			name:  "simple table",
			input: "select id from orders",
			expected: []SourceRef{
				{Kind: SourceKindTable, Name: "orders"},
			},
		},
		{
			name:  "aliased table implicit",
			input: "select id from orders o",
			expected: []SourceRef{
				{Kind: SourceKindTable, Name: "orders", Alias: "o"},
			},
		},
		{
			name:  "aliased table explicit AS",
			input: "select id from orders as o",
			expected: []SourceRef{
				{Kind: SourceKindTable, Name: "orders", Alias: "o"},
			},
		},
		{
			name:  "ref in FROM",
			input: "select * from {{ ref('orders') }}",
			expected: []SourceRef{
				{Kind: SourceKindRef, Name: "orders"},
			},
		},
		{
			name:  "ref with alias",
			input: "select * from {{ ref('orders') }} o",
			expected: []SourceRef{
				{Kind: SourceKindRef, Name: "orders", Alias: "o"},
			},
		},
		{
			name:  "source in FROM",
			input: "select * from {{ source('stripe', 'payments') }}",
			expected: []SourceRef{
				{Kind: SourceKindSource, Name: "payments", SourceName: "stripe"},
			},
		},
		{
			name:  "JOIN produces two sources",
			input: "select * from {{ ref('orders') }} o join {{ ref('customers') }} c on o.customer_id = c.id",
			expected: []SourceRef{
				{Kind: SourceKindRef, Name: "orders", Alias: "o"},
				{Kind: SourceKindRef, Name: "customers", Alias: "c"},
			},
		},
		{
			name:  "self-join different aliases",
			input: "select * from {{ ref('events') }} e1 join {{ ref('events') }} e2 on e1.id = e2.id",
			expected: []SourceRef{
				{Kind: SourceKindRef, Name: "events", Alias: "e1"},
				{Kind: SourceKindRef, Name: "events", Alias: "e2"},
			},
		},
		{
			name:  "jinja if/else in FROM",
			input: "select * from\n{% if true %}\n  {{ ref('orders_prod') }}\n{% else %}\n  {{ ref('orders_dev') }}\n{% endif %}",
			expected: []SourceRef{
				{Kind: SourceKindRef, Name: "orders_prod"},
				{Kind: SourceKindRef, Name: "orders_dev"},
			},
		},
		{
			name:  "left join",
			input: "select * from {{ ref('orders') }} o left join {{ ref('items') }} i on o.id = i.order_id right join {{ ref('users') }} u on o.user_id = u.id",
			expected: []SourceRef{
				{Kind: SourceKindRef, Name: "orders", Alias: "o"},
				{Kind: SourceKindRef, Name: "items", Alias: "i"},
				{Kind: SourceKindRef, Name: "users", Alias: "u"},
			},
		},
		{
			name:  "comma-separated FROM",
			input: "select * from orders, customers",
			expected: []SourceRef{
				{Kind: SourceKindTable, Name: "orders"},
				{Kind: SourceKindTable, Name: "customers"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Parse(tt.input, docs.Dialect("snowflake"))
			scope := p.CreateQueryScope()
			if scope == nil {
				t.Fatal("expected non-nil scope")
			}
			if len(scope.Sources) != len(tt.expected) {
				names := make([]string, len(scope.Sources))
				for i, s := range scope.Sources {
					names[i] = s.Name
				}
				t.Fatalf("expected %d Sources, got %d: %v", len(tt.expected), len(scope.Sources), names)
			}
			for i, exp := range tt.expected {
				got := scope.Sources[i]
				if got.Kind != exp.Kind {
					t.Errorf("source[%d] Kind: expected %d, got %d", i, exp.Kind, got.Kind)
				}
				if got.Name != exp.Name {
					t.Errorf("source[%d] Name: expected %q, got %q", i, exp.Name, got.Name)
				}
				if got.Alias != exp.Alias {
					t.Errorf("source[%d] Alias: expected %q, got %q", i, exp.Alias, got.Alias)
				}
				if exp.SourceName != "" && got.SourceName != exp.SourceName {
					t.Errorf("source[%d] SourceName: expected %q, got %q", i, exp.SourceName, got.SourceName)
				}
			}
		})
	}
}

func TestCTEParsing(t *testing.T) {
	t.Run("single CTE", func(t *testing.T) {
		input := "with cte as (select id, name from {{ ref('orders') }}) select * from cte"
		p := Parse(input, docs.Dialect("snowflake"))
		scope := p.CreateQueryScope()

		cteDef, ok := scope.CTEs["cte"]
		if !ok {
			t.Fatal("expected CTE 'cte' in scope")
		}
		if len(cteDef.Columns) != 2 {
			t.Fatalf("expected 2 columns, got %d: %v", len(cteDef.Columns), cteDef.Columns)
		}
		if cteDef.Columns[0] != "id" || cteDef.Columns[1] != "name" {
			t.Errorf("expected columns [id, name], got %v", cteDef.Columns)
		}

		// Main scope should have CTE source ref
		found := false
		for _, src := range scope.Sources {
			if src.Name == "cte" && src.Kind == SourceKindCTE {
				found = true
			}
		}
		if !found {
			t.Error("expected SourceRef with Kind=SourceKindCTE for 'cte'")
		}
	})

	t.Run("chained CTEs", func(t *testing.T) {
		input := "with a as (select id from {{ ref('orders') }}), b as (select id from a) select * from b"
		p := Parse(input, docs.Dialect("snowflake"))
		scope := p.CreateQueryScope()

		if _, ok := scope.CTEs["a"]; !ok {
			t.Fatal("expected CTE 'a'")
		}
		cteb, ok := scope.CTEs["b"]
		if !ok {
			t.Fatal("expected CTE 'b'")
		}
		if len(cteb.Columns) != 1 || cteb.Columns[0] != "id" {
			t.Errorf("expected CTE b columns [id], got %v", cteb.Columns)
		}
		// CTE b should reference CTE a
		if cteb.Scope == nil {
			t.Fatal("expected CTE b to have non-nil Scope")
		}
		found := false
		for _, src := range cteb.Scope.Sources {
			if src.Name == "a" && src.Kind == SourceKindCTE {
				found = true
			}
		}
		if !found {
			t.Error("expected CTE b to have SourceRef for CTE a")
		}
	})

	t.Run("CTE with select star", func(t *testing.T) {
		input := "with cte as (select * from {{ ref('orders') }}) select * from cte"
		p := Parse(input, docs.Dialect("snowflake"))
		scope := p.CreateQueryScope()

		cteDef, ok := scope.CTEs["cte"]
		if !ok {
			t.Fatal("expected CTE 'cte'")
		}
		if len(cteDef.Columns) != 1 || cteDef.Columns[0] != "*" {
			t.Errorf("expected columns [*], got %v", cteDef.Columns)
		}
	})

	t.Run("CTE with aliased columns", func(t *testing.T) {
		input := "with cte as (select id as order_id, name as customer_name from {{ ref('orders') }}) select * from cte"
		p := Parse(input, docs.Dialect("snowflake"))
		scope := p.CreateQueryScope()

		cteDef, ok := scope.CTEs["cte"]
		if !ok {
			t.Fatal("expected CTE 'cte'")
		}
		if len(cteDef.Columns) != 2 {
			t.Fatalf("expected 2 columns, got %d: %v", len(cteDef.Columns), cteDef.Columns)
		}
		if cteDef.Columns[0] != "order_id" || cteDef.Columns[1] != "customer_name" {
			t.Errorf("expected [order_id, customer_name], got %v", cteDef.Columns)
		}
	})

	t.Run("backward compat - old CTE struct", func(t *testing.T) {
		input := "with cte1 as (select id from {{ ref('orders') }}), cte2 as (select id from cte1) select * from cte2"
		p := Parse(input, docs.Dialect("snowflake"))
		tokenMap := p.CreateTokenNameMap()

		if _, ok := tokenMap["cte1"]; !ok {
			t.Error("expected cte1 in CreateTokenNameMap")
		}
		if _, ok := tokenMap["cte2"]; !ok {
			t.Error("expected cte2 in CreateTokenNameMap")
		}
	})

	t.Run("CTE body does not corrupt main scope", func(t *testing.T) {
		input := "with cte as (select id from t) select name from {{ ref('orders') }}"
		p := Parse(input, docs.Dialect("snowflake"))
		scope := p.CreateQueryScope()

		// Main scope should have SelectItems from the outer SELECT
		if len(scope.SelectItems) != 1 {
			t.Fatalf("expected 1 SelectItem in main scope, got %d", len(scope.SelectItems))
		}
		if scope.SelectItems[0].Alias != "name" {
			t.Errorf("expected main scope SelectItem alias 'name', got %q", scope.SelectItems[0].Alias)
		}
	})
}

func TestCTEErrorRecovery(t *testing.T) {
	t.Run("missing closing paren", func(t *testing.T) {
		input := "with cte as (select id from t"
		p := Parse(input, docs.Dialect("snowflake"))
		scope := p.CreateQueryScope()
		if scope == nil {
			t.Fatal("expected non-nil scope")
		}
	})

	t.Run("jinja only input", func(t *testing.T) {
		input := "{{ ref('orders') }}"
		p := Parse(input, docs.Dialect("snowflake"))
		scope := p.CreateQueryScope()
		if scope == nil {
			t.Fatal("expected non-nil scope")
		}
		if len(scope.CTEs) != 0 {
			t.Errorf("expected no CTEs, got %d", len(scope.CTEs))
		}
	})
}
