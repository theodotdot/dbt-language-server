package diagnostics

import (
	"testing"

	"github.com/j-clemons/dbt-language-server/analysis"
)

func emptyCtx() analysis.DbtContext {
	return analysis.DbtContext{}
}

func TestCheckJinja(t *testing.T) {
	t.Run("matched expression no diagnostic", func(t *testing.T) {
		doc := makeDoc("{{ x }}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 0 {
			t.Errorf("expected 0, got %d: %v", len(diags), diags)
		}
	})

	t.Run("matched block no diagnostic", func(t *testing.T) {
		doc := makeDoc("{% if true %}select 1{% endif %}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 0 {
			t.Errorf("expected 0, got %d: %v", len(diags), diags)
		}
	})

	t.Run("matched for block", func(t *testing.T) {
		doc := makeDoc("{% for x in items %}select 1{% endfor %}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 0 {
			t.Errorf("expected 0, got %d: %v", len(diags), diags)
		}
	})

	t.Run("nested blocks", func(t *testing.T) {
		doc := makeDoc("{% if true %}{% for x in y %}a{% endfor %}{% endif %}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 0 {
			t.Errorf("expected 0, got %d: %v", len(diags), diags)
		}
	})

	t.Run("mismatched block tags", func(t *testing.T) {
		doc := makeDoc("{% if true %}{% endfor %}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 1 {
			t.Fatalf("expected 1, got %d: %v", len(diags), diags)
		}
		if diags[0].Severity != 1 {
			t.Errorf("severity: got %d, want 1", diags[0].Severity)
		}
	})

	t.Run("unclosed block at EOF", func(t *testing.T) {
		doc := makeDoc("{% if true %}select 1")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 1 {
			t.Fatalf("expected 1, got %d: %v", len(diags), diags)
		}
		if diags[0].Severity != 1 {
			t.Errorf("severity: got %d, want 1", diags[0].Severity)
		}
	})

	t.Run("unexpected closer no opener", func(t *testing.T) {
		doc := makeDoc("{% endif %}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 1 {
			t.Fatalf("expected 1, got %d: %v", len(diags), diags)
		}
	})

	t.Run("elif inside if ok", func(t *testing.T) {
		doc := makeDoc("{% if a %}x{% elif b %}y{% endif %}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 0 {
			t.Errorf("expected 0, got %d: %v", len(diags), diags)
		}
	})

	t.Run("else inside if ok", func(t *testing.T) {
		doc := makeDoc("{% if a %}x{% else %}y{% endif %}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 0 {
			t.Errorf("expected 0, got %d: %v", len(diags), diags)
		}
	})

	t.Run("elif inside for error", func(t *testing.T) {
		doc := makeDoc("{% for x in y %}{% elif z %}{% endfor %}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) < 1 {
			t.Fatalf("expected at least 1 diagnostic, got %d", len(diags))
		}
	})

	t.Run("else inside for ok", func(t *testing.T) {
		doc := makeDoc("{% for x in y %}a{% else %}b{% endfor %}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 0 {
			t.Errorf("expected 0 (Jinja2 for/else), got %d: %v", len(diags), diags)
		}
	})

	t.Run("empty expression", func(t *testing.T) {
		doc := makeDoc("{{ }}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 1 {
			t.Fatalf("expected 1, got %d: %v", len(diags), diags)
		}
		if diags[0].Severity != 1 {
			t.Errorf("severity: got %d, want 1", diags[0].Severity)
		}
		want := `Empty expression "{{ }}"`
		if diags[0].Message != want {
			t.Errorf("message: got %q, want %q", diags[0].Message, want)
		}
	})

	t.Run("non-empty expression ok", func(t *testing.T) {
		doc := makeDoc("{{ var }}")
		diags := CheckJinja(doc, emptyCtx())
		// Only jinja structural issues, not ref checks
		for _, d := range diags {
			if d.Message == `Empty expression "{{ }}"` {
				t.Errorf("should not flag non-empty expression")
			}
		}
	})

	t.Run("raw block content ignored", func(t *testing.T) {
		doc := makeDoc("{% raw %}{% if %}{% endraw %}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 0 {
			t.Errorf("expected 0 (raw content ignored), got %d: %v", len(diags), diags)
		}
	})

	t.Run("set is not a block opener", func(t *testing.T) {
		doc := makeDoc("{% set x = 1 %}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 0 {
			t.Errorf("expected 0, got %d: %v", len(diags), diags)
		}
	})

	t.Run("source field set", func(t *testing.T) {
		doc := makeDoc("{{ }}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 1 {
			t.Fatalf("expected 1, got %d", len(diags))
		}
		if diags[0].Source != "dbt-ls" {
			t.Errorf("source: got %q, want %q", diags[0].Source, "dbt-ls")
		}
	})

	t.Run("macro block match", func(t *testing.T) {
		doc := makeDoc("{% macro my_func() %}body{% endmacro %}")
		diags := CheckJinja(doc, emptyCtx())
		if len(diags) != 0 {
			t.Errorf("expected 0, got %d: %v", len(diags), diags)
		}
	})
}
