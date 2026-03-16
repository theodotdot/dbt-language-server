package analysis

import (
	"testing"

	"github.com/j-clemons/dbt-language-server/analysis/jinja"
)

func TestExtractDocsBlocks(t *testing.T) {
	docsFileStr := `
{% docs table_events %}

This table contains clickstream events from the marketing website.

{% enddocs %}
`
	result := jinja.ExtractDocsBlocks(docsFileStr)

	expected := "This table contains clickstream events from the marketing website."
	if content, ok := result["table_events"]; !ok {
		t.Error("expected 'table_events' in result")
	} else if content != expected {
		t.Errorf("got %q, want %q", content, expected)
	}
}
