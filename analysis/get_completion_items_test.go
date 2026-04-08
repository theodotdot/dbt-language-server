package analysis

import (
	"reflect"
	"sort"
	"testing"

	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/lsp/completionKind"
)

func TestReverseRefPrefix(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Full Jinja",
			input:    "{{ ('",
			expected: "') }}",
		},
		{
			name:     "Multiple Spaces",
			input:    "{{   ('",
			expected: "')   }}",
		},
		{
			name:     "No Spaces",
			input:    "{{('",
			expected: "')}}",
		},
		{
			name:     "Generic Reversal",
			input:    "reversal",
			expected: "lasrever",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := reverseRefPrefix(tc.input)
			if result != tc.expected {
				t.Errorf("input: %s; got: %s; want: %s",
					tc.input, result, tc.expected)
			}
		})
	}
}

func TestGetReferenceSuffix(t *testing.T) {
	testCases := []struct {
		name     string
		ref      string
		trailing string
		expected string
	}{
		{
			name:     "Full Jinja",
			ref:      "{{ ref('",
			trailing: "",
			expected: "') }}",
		},
		{
			name:     "Multiple Spaces",
			ref:      "{{   ref('",
			trailing: "",
			expected: "')   }}",
		},
		{
			name:     "Trailing Jinja Symbols",
			ref:      "{{ ref('",
			trailing: "') }}",
			expected: "",
		},
		{
			name:     "Trailing Characters",
			ref:      "{{ ref('",
			trailing: "')",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getSuffix(tc.ref, tc.trailing, "ref")
			if result != tc.expected {
				t.Errorf("input: %s, %s; got: %s; want: %s",
					tc.ref, tc.trailing, result, tc.expected)
			}
		})
	}
}

func TestGetVariableSuffix(t *testing.T) {
	testCases := []struct {
		name     string
		vars     string
		trailing string
		expected string
	}{
		{
			name:     "Full Jinja",
			vars:     "{{ var('",
			trailing: "",
			expected: "') }}",
		},
		{
			name:     "Multiple Spaces",
			vars:     "{{   var('",
			trailing: "",
			expected: "')   }}",
		},
		{
			name:     "Trailing Jinja Symbols",
			vars:     "{{ var('",
			trailing: "') }}",
			expected: "",
		},
		{
			name:     "Trailing Characters",
			vars:     "{{ var('",
			trailing: "')",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getSuffix(tc.vars, tc.trailing, "var")
			if result != tc.expected {
				t.Errorf("input: %s, %s; got: %s; want: %s",
					tc.vars, tc.trailing, result, tc.expected)
			}
		})
	}
}

func TestBuildMacroSnippet(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		args     []MacroArg
		expected string
	}{
		{
			name:     "no args",
			prefix:   "my_macro",
			args:     nil,
			expected: "my_macro()",
		},
		{
			name:     "single arg",
			prefix:   "my_macro",
			args:     []MacroArg{{Name: "arg1"}},
			expected: "my_macro(${1:arg1})",
		},
		{
			name:     "multiple args",
			prefix:   "my_macro",
			args:     []MacroArg{{Name: "a"}, {Name: "b"}},
			expected: "my_macro(${1:a}, ${2:b})",
		},
		{
			name:     "args with defaults",
			prefix:   "my_macro",
			args:     []MacroArg{{Name: "a"}, {Name: "b", Default: "42"}},
			expected: "my_macro(${1:a}, ${2:42})",
		},
		{
			name:     "all args with defaults",
			prefix:   "m",
			args:     []MacroArg{{Name: "x", Default: "'hello'"}, {Name: "y", Default: "none"}},
			expected: "m(${1:'hello'}, ${2:none})",
		},
		{
			name:     "package prefix",
			prefix:   "pkg.my_macro",
			args:     []MacroArg{{Name: "val"}},
			expected: "pkg.my_macro(${1:val})",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := buildMacroSnippet(tc.prefix, tc.args)
			if result != tc.expected {
				t.Errorf("got %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestGetMacroCompletionItems(t *testing.T) {
	testState := expectedTestState()

	actualCompletionItems := getMacroCompletionItems(
		testState.DbtContext.MacroDetailMap,
		testState.DbtContext.ProjectYaml,
	)

	expectedCompletionItems := []lsp.CompletionItem{
		{
			Label:            "add_values",
			Detail:           "Project: jaffle_package",
			Documentation:    "add_values(arg1, arg2)",
			Kind:             completionKind.Snippet,
			InsertText:       "jaffle_package.add_values(${1:arg1}, ${2:arg2})",
			InsertTextFormat: 2,
			SortText:         "add_values",
		},
		{
			Label:            "full_name",
			Detail:           "Project: jaffle_shop",
			Documentation:    "full_name(first_name, last_name)",
			Kind:             completionKind.Snippet,
			InsertText:       "full_name(${1:first_name}, ${2:last_name})",
			InsertTextFormat: 2,
			SortText:         "full_name",
		},
		{
			Label:            "times_five",
			Detail:           "Project: jaffle_shop",
			Documentation:    "times_five(int_value)",
			Kind:             completionKind.Snippet,
			InsertText:       "times_five(${1:int_value})",
			InsertTextFormat: 2,
			SortText:         "times_five",
		},
	}

	sort.Slice(actualCompletionItems, func(i, j int) bool {
		return actualCompletionItems[i].Label < actualCompletionItems[j].Label
	})

	sort.Slice(expectedCompletionItems, func(i, j int) bool {
		return expectedCompletionItems[i].Label < expectedCompletionItems[j].Label
	})

	if !reflect.DeepEqual(actualCompletionItems, expectedCompletionItems) {
		t.Fatalf("expected %v,\n\ngot %v", expectedCompletionItems, actualCompletionItems)
	}
}

func TestGetColumnCompletionItems(t *testing.T) {
	modelMap := map[string]ModelDetails{
		"customers": {
			Columns: []Column{
				{Name: "customer_id", Description: "Unique ID"},
				{Name: "first_name", Description: "First name"},
			},
		},
		"orders": {
			Columns: []Column{
				{Name: "order_id", Description: "Order ID"},
				{Name: "customer_id", Description: "FK to customers"},
			},
		},
		"empty_model": {},
	}

	t.Run("single model", func(t *testing.T) {
		items := getColumnCompletionItems([]string{"customers"}, modelMap)
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
		if items[0].Kind != completionKind.Field {
			t.Errorf("expected Field kind, got %d", items[0].Kind)
		}
	})

	t.Run("multiple models with dedup", func(t *testing.T) {
		items := getColumnCompletionItems([]string{"customers", "orders"}, modelMap)
		if len(items) != 3 {
			t.Fatalf("expected 3 items, got %d", len(items))
		}
		names := map[string]bool{}
		for _, item := range items {
			names[item.Label] = true
		}
		for _, expected := range []string{"customer_id", "first_name", "order_id"} {
			if !names[expected] {
				t.Errorf("missing expected column %q", expected)
			}
		}
	})

	t.Run("no columns", func(t *testing.T) {
		items := getColumnCompletionItems([]string{"empty_model"}, modelMap)
		if len(items) != 0 {
			t.Fatalf("expected 0 items, got %d", len(items))
		}
	})

	t.Run("unknown model", func(t *testing.T) {
		items := getColumnCompletionItems([]string{"nonexistent"}, modelMap)
		if len(items) != 0 {
			t.Fatalf("expected 0 items, got %d", len(items))
		}
	})

	t.Run("nil model names", func(t *testing.T) {
		items := getColumnCompletionItems(nil, modelMap)
		if len(items) != 0 {
			t.Fatalf("expected 0 items, got %d", len(items))
		}
	})

	t.Run("dedup preserves first model detail", func(t *testing.T) {
		items := getColumnCompletionItems([]string{"customers", "orders"}, modelMap)
		for _, item := range items {
			if item.Label == "customer_id" {
				if item.Detail != "Column from customers" {
					t.Errorf("expected Detail from first model, got %q", item.Detail)
				}
				if item.Documentation != "Unique ID" {
					t.Errorf("expected Description from first model, got %q", item.Documentation)
				}
				return
			}
		}
		t.Error("customer_id not found")
	})

	t.Run("empty description", func(t *testing.T) {
		m := map[string]ModelDetails{
			"stg": {Columns: []Column{{Name: "id", Description: ""}}},
		}
		items := getColumnCompletionItems([]string{"stg"}, m)
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
		if items[0].Documentation != "" {
			t.Errorf("expected empty documentation, got %q", items[0].Documentation)
		}
	})
}

func TestGetScopeColumnCompletionItems(t *testing.T) {
	t.Run("single source", func(t *testing.T) {
		items := getScopeColumnCompletionItems([]parser.ScopeColumn{
			{Name: "customer_id", Source: "customers"},
		})
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
		if items[0].Label != "customer_id" {
			t.Errorf("Label: got %q, want %q", items[0].Label, "customer_id")
		}
		if items[0].Kind != completionKind.Field {
			t.Errorf("Kind: got %d, want %d", items[0].Kind, completionKind.Field)
		}
		if items[0].Detail != "Column from customers" {
			t.Errorf("Detail: got %q, want %q", items[0].Detail, "Column from customers")
		}
		if items[0].SortText != "0customer_id" {
			t.Errorf("SortText: got %q, want %q", items[0].SortText, "0customer_id")
		}
	})

	t.Run("multiple sources no dedup", func(t *testing.T) {
		items := getScopeColumnCompletionItems([]parser.ScopeColumn{
			{Name: "id", Source: "orders"},
			{Name: "id", Source: "customers"},
		})
		if len(items) != 2 {
			t.Fatalf("expected 2 items (no dedup), got %d", len(items))
		}
		details := map[string]bool{}
		for _, item := range items {
			details[item.Detail] = true
		}
		if !details["Column from orders"] || !details["Column from customers"] {
			t.Errorf("expected distinct details, got %v", details)
		}
	})

	t.Run("with alias shows source in detail", func(t *testing.T) {
		items := getScopeColumnCompletionItems([]parser.ScopeColumn{
			{Name: "total", Source: "orders", Qualified: "o.total"},
		})
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
		if items[0].Detail != "Column from orders" {
			t.Errorf("Detail: got %q, want %q", items[0].Detail, "Column from orders")
		}
	})

	t.Run("sort text 0 prefix", func(t *testing.T) {
		items := getScopeColumnCompletionItems([]parser.ScopeColumn{
			{Name: "amount", Source: "payments"},
			{Name: "id", Source: "payments"},
		})
		for _, item := range items {
			expected := "0" + item.Label
			if item.SortText != expected {
				t.Errorf("SortText: got %q, want %q", item.SortText, expected)
			}
		}
	})

	t.Run("empty input", func(t *testing.T) {
		items := getScopeColumnCompletionItems(nil)
		if items == nil {
			t.Fatal("expected empty slice, got nil")
		}
		if len(items) != 0 {
			t.Fatalf("expected 0 items, got %d", len(items))
		}
	})

	t.Run("description carried through", func(t *testing.T) {
		items := getScopeColumnCompletionItems([]parser.ScopeColumn{
			{Name: "amount", Source: "payments", Description: "Payment amount"},
		})
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
		if items[0].Documentation != "Payment amount" {
			t.Errorf("Documentation: got %q, want %q", items[0].Documentation, "Payment amount")
		}
	})
}
