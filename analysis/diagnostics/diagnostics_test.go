package diagnostics

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/j-clemons/dbt-language-server/analysis"
	"github.com/j-clemons/dbt-language-server/lsp"
)

func newTestState(uri string) *analysis.State {
	s := analysis.NewState()
	s.Documents[uri] = analysis.Document{
		Text: "SELECT 1",
	}
	return &s
}

func parseNotifications(t *testing.T, buf *bytes.Buffer) []lsp.DiagnosticsNotification {
	t.Helper()
	raw := buf.String()
	var notifications []lsp.DiagnosticsNotification
	for {
		idx := strings.Index(raw, "\r\n\r\n")
		if idx == -1 {
			break
		}
		body := raw[idx+4:]
		// find end of JSON object
		var notif lsp.DiagnosticsNotification
		dec := json.NewDecoder(strings.NewReader(body))
		if err := dec.Decode(&notif); err != nil {
			break
		}
		notifications = append(notifications, notif)
		// advance past this message
		consumed := idx + 4 + int(dec.InputOffset())
		raw = raw[consumed:]
	}
	return notifications
}

func parseLastNotification(t *testing.T, buf *bytes.Buffer) lsp.DiagnosticsNotification {
	t.Helper()
	notifs := parseNotifications(t, buf)
	if len(notifs) == 0 {
		t.Fatal("no notifications found in buffer")
	}
	return notifs[len(notifs)-1]
}

func TestNewEngineRegistersCheckers(t *testing.T) {
	c1 := Checker(func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic { return nil })
	c2 := Checker(func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic { return nil })
	e := NewEngine(nil, nil, c1, c2)
	if len(e.checkers) != 2 {
		t.Errorf("checkers count = %d, want 2", len(e.checkers))
	}
}

func TestRunImmediatePublishesDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	uri := "file:///test.sql"
	s := newTestState(uri)

	checker := Checker(func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
		return []lsp.Diagnostic{
			{Message: "test error", Severity: 1, Source: "will-be-overwritten"},
		}
	})

	e := NewEngine(&buf, s, checker)
	e.RunImmediate(uri)

	notif := parseLastNotification(t, &buf)
	if notif.Params.URI != uri {
		t.Errorf("uri = %v, want %v", notif.Params.URI, uri)
	}
	if len(notif.Params.Diagnostics) != 1 {
		t.Fatalf("diagnostics count = %d, want 1", len(notif.Params.Diagnostics))
	}
	d := notif.Params.Diagnostics[0]
	if d.Message != "test error" {
		t.Errorf("message = %v, want 'test error'", d.Message)
	}
	if d.Source != "dbt-ls" {
		t.Errorf("source = %v, want 'dbt-ls'", d.Source)
	}
}

func TestRunImmediateNoDiagnosticsPublishesEmptyArray(t *testing.T) {
	var buf bytes.Buffer
	uri := "file:///test.sql"
	s := newTestState(uri)

	checker := Checker(func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
		return nil
	})

	e := NewEngine(&buf, s, checker)
	e.RunImmediate(uri)

	notif := parseLastNotification(t, &buf)
	if len(notif.Params.Diagnostics) != 0 {
		t.Errorf("diagnostics count = %d, want 0", len(notif.Params.Diagnostics))
	}
}

func TestRunDebouncedFiresAfterDelay(t *testing.T) {
	var buf bytes.Buffer
	uri := "file:///test.sql"
	s := newTestState(uri)

	checker := Checker(func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
		return []lsp.Diagnostic{{Message: "delayed"}}
	})

	e := NewEngine(&buf, s, checker)
	e.RunDebounced(uri)

	// should not have fired yet
	if buf.Len() > 0 {
		t.Error("expected no output immediately after RunDebounced")
	}

	time.Sleep(250 * time.Millisecond)

	if buf.Len() == 0 {
		t.Error("expected output after debounce delay")
	}
}

func TestRunDebouncedResetsTimer(t *testing.T) {
	var buf bytes.Buffer
	uri := "file:///test.sql"
	s := newTestState(uri)

	callCount := 0
	checker := Checker(func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
		callCount++
		return []lsp.Diagnostic{{Message: "debounced"}}
	})

	e := NewEngine(&buf, s, checker)
	e.RunDebounced(uri)
	time.Sleep(50 * time.Millisecond)
	e.RunDebounced(uri) // reset timer

	time.Sleep(250 * time.Millisecond)

	notifs := parseNotifications(t, &buf)
	if len(notifs) != 1 {
		t.Errorf("notification count = %d, want 1 (timer should have been reset)", len(notifs))
	}
}

func TestRunImmediateCancelsPendingDebounce(t *testing.T) {
	var buf bytes.Buffer
	uri := "file:///test.sql"
	s := newTestState(uri)

	checker := Checker(func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
		return []lsp.Diagnostic{{Message: "result"}}
	})

	e := NewEngine(&buf, s, checker)
	e.RunDebounced(uri)
	e.RunImmediate(uri)

	time.Sleep(250 * time.Millisecond)

	notifs := parseNotifications(t, &buf)
	if len(notifs) != 1 {
		t.Errorf("notification count = %d, want 1 (debounced should have been cancelled)", len(notifs))
	}
}

func TestClearPublishesEmptyDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	uri := "file:///test.sql"
	s := newTestState(uri)

	e := NewEngine(&buf, s)
	e.Clear(uri)

	notif := parseLastNotification(t, &buf)
	if notif.Params.URI != uri {
		t.Errorf("uri = %v, want %v", notif.Params.URI, uri)
	}
	if len(notif.Params.Diagnostics) != 0 {
		t.Errorf("diagnostics count = %d, want 0", len(notif.Params.Diagnostics))
	}
}

func TestClearCancelsPendingDebounce(t *testing.T) {
	var buf bytes.Buffer
	uri := "file:///test.sql"
	s := newTestState(uri)

	checker := Checker(func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
		return []lsp.Diagnostic{{Message: "should not appear"}}
	})

	e := NewEngine(&buf, s, checker)
	e.RunDebounced(uri)
	e.Clear(uri)

	time.Sleep(250 * time.Millisecond)

	notifs := parseNotifications(t, &buf)
	// only the Clear publish, not the debounced one
	if len(notifs) != 1 {
		t.Errorf("notification count = %d, want 1", len(notifs))
	}
	if len(notifs[0].Params.Diagnostics) != 0 {
		t.Errorf("clear notification should have 0 diagnostics, got %d", len(notifs[0].Params.Diagnostics))
	}
}

func TestMultipleCheckersCombineDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	uri := "file:///test.sql"
	s := newTestState(uri)

	c1 := Checker(func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
		return []lsp.Diagnostic{{Message: "error 1"}}
	})
	c2 := Checker(func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
		return []lsp.Diagnostic{{Message: "error 2"}}
	})

	e := NewEngine(&buf, s, c1, c2)
	e.RunImmediate(uri)

	notif := parseLastNotification(t, &buf)
	if len(notif.Params.Diagnostics) != 2 {
		t.Fatalf("diagnostics count = %d, want 2", len(notif.Params.Diagnostics))
	}
}

func TestNonSqlURISkipped(t *testing.T) {
	var buf bytes.Buffer
	uri := "file:///test.yml"
	s := newTestState(uri)

	checker := Checker(func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
		return []lsp.Diagnostic{{Message: "should not run"}}
	})

	e := NewEngine(&buf, s, checker)
	e.RunImmediate(uri)

	if buf.Len() > 0 {
		t.Error("expected no output for non-.sql URI")
	}
}
