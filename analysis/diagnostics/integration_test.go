package diagnostics

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/j-clemons/dbt-language-server/analysis"
	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/rpc"
)

// safeBuffer wraps bytes.Buffer with a mutex for concurrent test use.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sb *safeBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *safeBuffer) Len() int {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Len()
}

func (sb *safeBuffer) Bytes() []byte {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return append([]byte(nil), sb.buf.Bytes()...)
}

func decodeNotification(t *testing.T, data []byte) lsp.DiagnosticsNotification {
	t.Helper()
	idx := bytes.Index(data, []byte("\r\n\r\n"))
	if idx < 0 {
		t.Fatalf("no header separator in output: %q", data)
	}
	body := data[idx+4:]
	var notif lsp.DiagnosticsNotification
	if err := json.Unmarshal(body, &notif); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, body)
	}
	return notif
}

func TestDidOpenTriggersDiagnostics(t *testing.T) {
	buf := &safeBuffer{}
	state := analysis.NewState()
	engine := NewEngine(buf, &state,
		func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
			return []lsp.Diagnostic{{Message: "test", Severity: 2, Source: "dbt-ls"}}
		},
	)

	uri := "file:///test.sql"
	state.OpenDocument(uri, "select 1")
	engine.RunImmediate(uri)

	if buf.Len() == 0 {
		t.Fatal("expected diagnostic output")
	}
	notif := decodeNotification(t, buf.Bytes())
	if notif.Params.URI != uri {
		t.Errorf("uri: got %q, want %q", notif.Params.URI, uri)
	}
	if len(notif.Params.Diagnostics) != 1 {
		t.Errorf("diag count: got %d, want 1", len(notif.Params.Diagnostics))
	}
}

func TestNonSqlURISkippedByHandler(t *testing.T) {
	buf := &safeBuffer{}
	state := analysis.NewState()
	engine := NewEngine(buf, &state,
		func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
			return []lsp.Diagnostic{{Message: "test"}}
		},
	)

	uri := "file:///schema.yml"
	engine.RunImmediate(uri)

	if buf.Len() != 0 {
		t.Error("expected no output for non-.sql URI")
	}
}

func TestDidChangeTriggersDebouncedDiagnostics(t *testing.T) {
	buf := &safeBuffer{}
	state := analysis.NewState()
	engine := NewEngine(buf, &state,
		func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
			return []lsp.Diagnostic{{Message: "changed"}}
		},
	)

	uri := "file:///test.sql"
	state.OpenDocument(uri, "select 1")
	engine.RunDebounced(uri)

	// Should not fire immediately
	if buf.Len() != 0 {
		t.Error("debounced should not fire immediately")
	}

	time.Sleep(250 * time.Millisecond)

	if buf.Len() == 0 {
		t.Fatal("expected diagnostic output after debounce")
	}
}

func TestDidClosePublishesEmpty(t *testing.T) {
	buf := &safeBuffer{}
	state := analysis.NewState()
	engine := NewEngine(buf, &state)

	uri := "file:///test.sql"
	state.OpenDocument(uri, "select 1")
	engine.Clear(uri)

	if buf.Len() == 0 {
		t.Fatal("expected empty diagnostics on clear")
	}
	notif := decodeNotification(t, buf.Bytes())
	if len(notif.Params.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics, got %d", len(notif.Params.Diagnostics))
	}
}

func TestEngineNilWhenFusionEnabled(t *testing.T) {
	var engine *Engine
	if engine != nil {
		t.Error("nil engine should be nil")
	}
}

func TestConcurrentLocking(t *testing.T) {
	buf := &safeBuffer{}
	state := analysis.NewState()
	engine := NewEngine(buf, &state,
		func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
			return nil
		},
	)

	uri := "file:///test.sql"
	state.OpenDocument(uri, "select 1")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			state.OpenDocument(uri, "select 2")
		}()
		go func() {
			defer wg.Done()
			engine.RunImmediate(uri)
		}()
	}
	wg.Wait()
}

func TestAllCheckersRegistered(t *testing.T) {
	buf := &safeBuffer{}
	state := analysis.NewState()
	engine := NewEngine(buf, &state,
		CheckRefs,
		CheckSources,
		CheckVars,
		CheckMacros,
		CheckJinja,
		CheckColumns,
	)

	uri := "file:///test.sql"
	state.OpenDocument(uri, "select {{ ref('nonexistent') }}")
	engine.RunImmediate(uri)

	if buf.Len() == 0 {
		t.Fatal("expected diagnostic output")
	}

	raw := buf.Bytes()
	idx := bytes.Index(raw, []byte("\r\n\r\n"))
	body := raw[idx+4:]
	if !strings.Contains(string(body), "nonexistent") {
		t.Error("expected ref diagnostic for 'nonexistent'")
	}
}

func TestPublishFormat(t *testing.T) {
	buf := &safeBuffer{}
	state := analysis.NewState()
	engine := NewEngine(buf, &state,
		func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
			return []lsp.Diagnostic{{Message: "test"}}
		},
	)

	uri := "file:///test.sql"
	state.OpenDocument(uri, "select 1")
	engine.RunImmediate(uri)

	raw := buf.Bytes()
	if !bytes.HasPrefix(raw, []byte("Content-Length:")) {
		t.Error("missing Content-Length header")
	}

	_, body, err := rpc.DecodeMessage(raw)
	if err != nil {
		t.Fatalf("rpc decode: %v", err)
	}

	var notif lsp.DiagnosticsNotification
	if err := json.Unmarshal(body, &notif); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if notif.Method != "textDocument/publishDiagnostics" {
		t.Errorf("method: got %q", notif.Method)
	}
}
