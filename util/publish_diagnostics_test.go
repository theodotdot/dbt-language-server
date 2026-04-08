package util

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/j-clemons/dbt-language-server/lsp"
)

func TestPublishDiagnosticsWritesValidNotification(t *testing.T) {
	var buf bytes.Buffer
	diags := []lsp.Diagnostic{
		{
			Range: lsp.Range{
				Start: lsp.Position{Line: 1, Character: 5},
				End:   lsp.Position{Line: 1, Character: 10},
			},
			Message:  "undefined ref",
			Severity: 1,
			Code:     "E001",
			Source:   "dbt-ls",
		},
	}

	PublishDiagnostics(&buf, "file:///test.sql", diags)

	body := extractJSONBody(t, buf.String())
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if got["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", got["jsonrpc"])
	}
	if got["method"] != "textDocument/publishDiagnostics" {
		t.Errorf("method = %v, want textDocument/publishDiagnostics", got["method"])
	}

	params := got["params"].(map[string]any)
	if params["uri"] != "file:///test.sql" {
		t.Errorf("uri = %v, want file:///test.sql", params["uri"])
	}

	diagsOut := params["diagnostics"].([]any)
	if len(diagsOut) != 1 {
		t.Fatalf("diagnostics count = %d, want 1", len(diagsOut))
	}
	d := diagsOut[0].(map[string]any)
	if d["message"] != "undefined ref" {
		t.Errorf("message = %v, want 'undefined ref'", d["message"])
	}
}

func TestPublishDiagnosticsPreservesSource(t *testing.T) {
	var buf bytes.Buffer
	diags := []lsp.Diagnostic{
		{Message: "a", Source: "custom-source"},
		{Message: "b", Source: ""},
	}

	PublishDiagnostics(&buf, "file:///x.sql", diags)

	body := extractJSONBody(t, buf.String())
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	params := got["params"].(map[string]any)
	diagsOut := params["diagnostics"].([]any)

	first := diagsOut[0].(map[string]any)
	if first["source"] != "custom-source" {
		t.Errorf("source[0] = %v, want 'custom-source'", first["source"])
	}
	second := diagsOut[1].(map[string]any)
	if second["source"] != "" {
		t.Errorf("source[1] = %v, want empty", second["source"])
	}
}

func TestPublishDiagnosticsEmptySlice(t *testing.T) {
	var buf bytes.Buffer
	PublishDiagnostics(&buf, "file:///empty.sql", []lsp.Diagnostic{})

	body := extractJSONBody(t, buf.String())
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	params := got["params"].(map[string]any)
	diagsOut := params["diagnostics"].([]any)
	if len(diagsOut) != 0 {
		t.Errorf("diagnostics count = %d, want 0", len(diagsOut))
	}
}

func TestPublishDiagnosticsMethod(t *testing.T) {
	var buf bytes.Buffer
	PublishDiagnostics(&buf, "file:///m.sql", nil)

	body := extractJSONBody(t, buf.String())
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if got["method"] != "textDocument/publishDiagnostics" {
		t.Errorf("method = %v, want textDocument/publishDiagnostics", got["method"])
	}
}

// extractJSONBody strips the Content-Length header from an LSP message.
func extractJSONBody(t *testing.T, raw string) string {
	t.Helper()
	idx := strings.Index(raw, "\r\n\r\n")
	if idx == -1 {
		t.Fatalf("no Content-Length header found in output")
	}
	return raw[idx+4:]
}
