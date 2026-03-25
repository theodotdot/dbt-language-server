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
