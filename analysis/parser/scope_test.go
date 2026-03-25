package parser

import "testing"

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
