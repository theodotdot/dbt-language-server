package diagnostics

import (
	"io"
	"strings"
	"sync"
	"time"

	"github.com/j-clemons/dbt-language-server/analysis"
	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/util"
)

type Checker func(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic

type Engine struct {
	writer   io.Writer
	state    *analysis.State
	mu       sync.Mutex
	timers   map[string]*time.Timer
	checkers []Checker
}

func NewEngine(writer io.Writer, state *analysis.State, checkers ...Checker) *Engine {
	return &Engine{
		writer:   writer,
		state:    state,
		timers:   make(map[string]*time.Timer),
		checkers: checkers,
	}
}

func (e *Engine) RunDebounced(uri string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if t, ok := e.timers[uri]; ok {
		t.Stop()
	}
	e.timers[uri] = time.AfterFunc(150*time.Millisecond, func() {
		e.mu.Lock()
		delete(e.timers, uri)
		e.mu.Unlock()
		e.runCheckers(uri)
	})
}

func (e *Engine) RunImmediate(uri string) {
	e.mu.Lock()
	if t, ok := e.timers[uri]; ok {
		t.Stop()
		delete(e.timers, uri)
	}
	e.mu.Unlock()
	e.runCheckers(uri)
}

func (e *Engine) Clear(uri string) {
	e.mu.Lock()
	if t, ok := e.timers[uri]; ok {
		t.Stop()
		delete(e.timers, uri)
	}
	e.mu.Unlock()
	util.PublishDiagnostics(e.writer, uri, []lsp.Diagnostic{})
}

func (e *Engine) runCheckers(uri string) {
	if !strings.HasSuffix(uri, ".sql") {
		return
	}

	doc, ctx, ok := e.state.Snapshot(uri)
	if !ok {
		util.PublishDiagnostics(e.writer, uri, []lsp.Diagnostic{})
		return
	}

	var all []lsp.Diagnostic
	for _, checker := range e.checkers {
		all = append(all, checker(doc, ctx)...)
	}

	for i := range all {
		all[i].Source = "dbt-ls"
	}

	if all == nil {
		all = []lsp.Diagnostic{}
	}

	util.PublishDiagnostics(e.writer, uri, all)
}
