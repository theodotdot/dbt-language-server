package docs

import (
	"strings"
	"testing"
)

func TestFunctionCompletionItemsSortText(t *testing.T) {
	dialect := Dialect("snowflake")
	items := dialect.FunctionCompletionItems()
	if len(items) == 0 {
		t.Fatal("expected function completion items")
	}
	for _, item := range items {
		if !strings.HasPrefix(item.SortText, "2") {
			t.Errorf("function SortText %q should start with '2'", item.SortText)
		}
	}
}
