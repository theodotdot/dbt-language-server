package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/docs"
	"github.com/j-clemons/dbt-language-server/lsp"
)

func newRenameState(uri, sql string) *State {
	s := NewState()
	p := parser.Parse(sql, "snowflake")
	s.Documents[uri] = Document{
		Text:      sql,
		Tokens:    p.CreateTokenIndex(),
		DefTokens: p.CreateTokenNameMap(),
	}
	return &s
}

func TestPrepareRenameRefToken(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('orders') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {URI: "models/orders.sql"},
	}

	resp := s.PrepareRename(1, uri, lsp.Position{Line: 0, Character: 22})
	if resp.Result == nil {
		t.Fatal("expected non-nil result for REF token")
	}
	if resp.Result.Placeholder != "orders" {
		t.Errorf("placeholder: got %q, want %q", resp.Result.Placeholder, "orders")
	}
	width := resp.Result.Range.End.Character - resp.Result.Range.Start.Character
	if width != len("orders") {
		t.Errorf("range width: got %d, want %d", width, len("orders"))
	}
}

func TestPrepareRenameModelInSchema(t *testing.T) {
	uri := "file:///models/schema.yml"
	s := NewState()
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"customers": {
			SchemaURI: "/models/schema.yml",
			SchemaRange: lsp.Range{
				Start: lsp.Position{Line: 5, Character: 10},
				End:   lsp.Position{Line: 5, Character: 10},
			},
		},
	}

	resp := s.PrepareRename(1, uri, lsp.Position{Line: 5, Character: 12})
	if resp.Result == nil {
		t.Fatal("expected non-nil result for model in schema")
	}
	if resp.Result.Placeholder != "customers" {
		t.Errorf("placeholder: got %q, want %q", resp.Result.Placeholder, "customers")
	}
	if resp.Result.Range.End.Character != 10+len("customers") {
		t.Errorf("end char: got %d, want %d", resp.Result.Range.End.Character, 10+len("customers"))
	}
}

func TestPrepareRenameColumnInSchema(t *testing.T) {
	uri := "file:///models/schema.yml"
	s := NewState()
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {
			SchemaURI: "/models/schema.yml",
			SchemaRange: lsp.Range{
				Start: lsp.Position{Line: 3, Character: 8},
				End:   lsp.Position{Line: 3, Character: 8},
			},
			Columns: []Column{
				{Name: "order_id", Position: lsp.Position{Line: 7, Character: 12}},
			},
		},
	}

	resp := s.PrepareRename(1, uri, lsp.Position{Line: 7, Character: 14})
	if resp.Result == nil {
		t.Fatal("expected non-nil result for column in schema")
	}
	if resp.Result.Placeholder != "order_id" {
		t.Errorf("placeholder: got %q, want %q", resp.Result.Placeholder, "order_id")
	}
}

func TestPrepareRenameCrossProjectRefReturnsNull(t *testing.T) {
	uri := "file:///test.sql"
	// Cross-project ref: ref('pkg', 'orders')
	sql := "select * from {{ ref('pkg', 'orders') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {},
	}

	// Find the position of "orders" in the parsed tokens
	resp := s.PrepareRename(1, uri, lsp.Position{Line: 0, Character: 30})
	if resp.Result != nil {
		t.Error("expected nil result for cross-project ref")
	}
}

func TestPrepareRenameNonRenameableReturnsNull(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ source('src', 'tbl') }}"
	s := newRenameState(uri, sql)

	// Cursor on "select" keyword
	resp := s.PrepareRename(1, uri, lsp.Position{Line: 0, Character: 0})
	if resp.Result != nil {
		t.Error("expected nil result for non-renameable position")
	}
}

func TestPrepareRenameNonExistentModelReturnsNull(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('nonexistent') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{}

	resp := s.PrepareRename(1, uri, lsp.Position{Line: 0, Character: 22})
	if resp.Result != nil {
		t.Error("expected nil result for non-existent model")
	}
}

func TestPrepareRenameRangeWidth(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('my_model') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"my_model": {},
	}

	resp := s.PrepareRename(1, uri, lsp.Position{Line: 0, Character: 22})
	if resp.Result == nil {
		t.Fatal("expected non-nil result")
	}
	width := resp.Result.Range.End.Character - resp.Result.Range.Start.Character
	if width != len("my_model") {
		t.Errorf("width: got %d, want %d", width, len("my_model"))
	}
}

// --- findModelRefs tests ---

func testdataRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(filepath.Dir(wd), "testdata")
}

func TestFindModelRefs_SingleFile(t *testing.T) {
	root := testdataRoot(t)
	s := NewState()
	s.DbtContext.ProjectRoot = root
	s.DbtContext.Dialect = docs.Dialect("snowflake")
	s.DbtContext.ProjectYaml.ModelPaths = AnnotatedField[[]string]{Value: []string{"models"}}

	edits := s.findModelRefs("orders", "new_orders")
	// single_ref.sql has 1 ref('orders'), multi_ref.sql has 2 = 3 total
	totalEdits := 0
	for _, e := range edits {
		totalEdits += len(e.Edits)
	}
	if totalEdits != 3 {
		t.Fatalf("expected 3 total edits, got %d", totalEdits)
	}
}

func TestFindModelRefs_MultipleFiles(t *testing.T) {
	root := testdataRoot(t)
	s := NewState()
	s.DbtContext.ProjectRoot = root
	s.DbtContext.Dialect = docs.Dialect("snowflake")
	s.DbtContext.ProjectYaml.ModelPaths = AnnotatedField[[]string]{Value: []string{"models"}}

	edits := s.findModelRefs("orders", "new_orders")
	// Should have edits from both single_ref.sql and multi_ref.sql
	if len(edits) != 2 {
		t.Fatalf("expected edits from exactly 2 files, got %d", len(edits))
	}
}

func TestFindModelRefs_SkipsNonMatching(t *testing.T) {
	root := testdataRoot(t)
	s := NewState()
	s.DbtContext.ProjectRoot = root
	s.DbtContext.Dialect = docs.Dialect("snowflake")
	s.DbtContext.ProjectYaml.ModelPaths = AnnotatedField[[]string]{Value: []string{"models"}}

	edits := s.findModelRefs("nonexistent_model", "x")
	if len(edits) != 0 {
		t.Errorf("expected 0 edits for non-matching model, got %d", len(edits))
	}
}

func TestFindModelRefs_SkipsCrossProjectRef(t *testing.T) {
	root := testdataRoot(t)
	s := NewState()
	s.DbtContext.ProjectRoot = root
	s.DbtContext.Dialect = docs.Dialect("snowflake")
	s.DbtContext.ProjectYaml.ModelPaths = AnnotatedField[[]string]{Value: []string{"models"}}

	edits := s.findModelRefs("orders", "new_orders")
	// cross_project_ref.sql has ref('some_pkg', 'orders') — should be excluded
	for _, e := range edits {
		uri := e.TextDocument.URI
		if filepath.Base(uri) == "cross_project_ref.sql" {
			t.Errorf("should not include cross-project ref file, but found URI %s", uri)
		}
	}
}

func TestFindModelRefs_UsesOpenDocuments(t *testing.T) {
	s := NewState()
	s.DbtContext.ProjectRoot = t.TempDir()
	s.DbtContext.Dialect = docs.Dialect("snowflake")
	s.DbtContext.ProjectYaml.ModelPaths = AnnotatedField[[]string]{Value: []string{"models"}}

	// Create model dir with a SQL file on disk
	modelsDir := filepath.Join(s.DbtContext.ProjectRoot, "models")
	os.MkdirAll(modelsDir, 0o755)
	diskPath := filepath.Join(modelsDir, "test.sql")
	os.WriteFile(diskPath, []byte("select 1"), 0o644)

	// Open document with different in-memory content containing ref
	uri := "file://" + diskPath
	inMemSQL := "select * from {{ ref('orders') }}"
	p := parser.Parse(inMemSQL, "snowflake")
	s.Documents[uri] = Document{
		Text:   inMemSQL,
		Tokens: p.CreateTokenIndex(),
	}

	edits := s.findModelRefs("orders", "new_orders")
	if len(edits) != 1 {
		t.Fatalf("expected 1 file with edits from open doc, got %d", len(edits))
	}
	if len(edits[0].Edits) != 1 {
		t.Errorf("expected 1 edit, got %d", len(edits[0].Edits))
	}
}

func TestFindModelRefs_ReadsFromDisk(t *testing.T) {
	root := testdataRoot(t)
	s := NewState()
	s.DbtContext.ProjectRoot = root
	s.DbtContext.Dialect = docs.Dialect("snowflake")
	s.DbtContext.ProjectYaml.ModelPaths = AnnotatedField[[]string]{Value: []string{"models"}}
	// No documents open — should read from disk

	edits := s.findModelRefs("orders", "new_orders")
	if len(edits) == 0 {
		t.Fatal("expected edits from disk files, got 0")
	}
}

func TestFindModelRefs_SkipsMissingFiles(t *testing.T) {
	s := NewState()
	s.DbtContext.ProjectRoot = "/nonexistent/path"
	s.DbtContext.Dialect = docs.Dialect("snowflake")
	s.DbtContext.ProjectYaml.ModelPaths = AnnotatedField[[]string]{Value: []string{"models"}}

	// Should not panic
	edits := s.findModelRefs("orders", "new_orders")
	if len(edits) != 0 {
		t.Errorf("expected 0 edits for missing paths, got %d", len(edits))
	}
}

func TestFindModelRefs_EditRangeCorrect(t *testing.T) {
	root := testdataRoot(t)
	s := NewState()
	s.DbtContext.ProjectRoot = root
	s.DbtContext.Dialect = docs.Dialect("snowflake")
	s.DbtContext.ProjectYaml.ModelPaths = AnnotatedField[[]string]{Value: []string{"models"}}

	edits := s.findModelRefs("orders", "new_orders")
	for _, docEdit := range edits {
		for _, edit := range docEdit.Edits {
			width := edit.Range.End.Character - edit.Range.Start.Character
			if width != len("orders") {
				t.Errorf("edit range width: got %d, want %d (file %s)", width, len("orders"), docEdit.TextDocument.URI)
			}
		}
	}
}

// --- Rename handler tests ---

func TestRenameInvalidIdentifier(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('orders') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {URI: "/project/models/orders.sql"},
	}

	for _, bad := range []string{"my model", "1orders", "orders!", ""} {
		resp := s.Rename(1, uri, lsp.Position{Line: 0, Character: 22}, bad)
		if resp.Error == nil {
			t.Errorf("expected error for newName=%q, got nil", bad)
		}
		if resp.Error != nil && resp.Error.Code != lsp.ErrCodeInvalidParams {
			t.Errorf("expected code %d, got %d", lsp.ErrCodeInvalidParams, resp.Error.Code)
		}
	}
}

func TestRenameModelConflict(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('orders') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders":    {URI: "/project/models/orders.sql"},
		"customers": {URI: "/project/models/customers.sql"},
	}

	resp := s.Rename(1, uri, lsp.Position{Line: 0, Character: 22}, "customers")
	if resp.Error == nil {
		t.Fatal("expected error for conflicting model name")
	}
	if resp.Error.Code != lsp.ErrCodeInvalidParams {
		t.Errorf("expected code %d, got %d", lsp.ErrCodeInvalidParams, resp.Error.Code)
	}
}

func TestRenameColumnConflict(t *testing.T) {
	uri := "file:///models/schema.yml"
	s := NewState()
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {
			SchemaURI: "/models/schema.yml",
			SchemaRange: lsp.Range{
				Start: lsp.Position{Line: 3, Character: 8},
				End:   lsp.Position{Line: 3, Character: 8},
			},
			Columns: []Column{
				{Name: "id", Position: lsp.Position{Line: 5, Character: 10}},
				{Name: "name", Position: lsp.Position{Line: 6, Character: 10}},
			},
		},
	}

	resp := s.Rename(1, uri, lsp.Position{Line: 5, Character: 10}, "name")
	if resp.Error == nil {
		t.Fatal("expected error for conflicting column name")
	}
}

func TestRenameSameNameNoop(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('orders') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {URI: "/project/models/orders.sql"},
	}

	resp := s.Rename(1, uri, lsp.Position{Line: 0, Character: 22}, "orders")
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	if resp.Result != nil && len(resp.Result.DocumentChanges) > 0 {
		t.Error("expected empty workspace edit for noop rename")
	}
}

func TestRenameModelRenameFile(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('orders') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {URI: "/project/models/orders.sql"},
	}

	resp := s.Rename(1, uri, lsp.Position{Line: 0, Character: 22}, "purchases")
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	if resp.Result == nil {
		t.Fatal("expected non-nil result")
	}

	// Find RenameFile entry
	found := false
	for _, dc := range resp.Result.DocumentChanges {
		if dc.RenameFileValue != nil {
			found = true
			if dc.RenameFileValue.Kind != "rename" {
				t.Errorf("kind: got %q, want %q", dc.RenameFileValue.Kind, "rename")
			}
			if dc.RenameFileValue.OldURI != "file:///project/models/orders.sql" {
				t.Errorf("oldURI: got %q", dc.RenameFileValue.OldURI)
			}
			if dc.RenameFileValue.NewURI != "file:///project/models/purchases.sql" {
				t.Errorf("newURI: got %q", dc.RenameFileValue.NewURI)
			}
		}
	}
	if !found {
		t.Error("expected RenameFile in documentChanges")
	}
}

func TestRenameModelSchemaEdit(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('orders') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {
			URI:       "/project/models/orders.sql",
			SchemaURI: "/project/models/schema.yml",
			SchemaRange: lsp.Range{
				Start: lsp.Position{Line: 5, Character: 10},
				End:   lsp.Position{Line: 5, Character: 10},
			},
		},
	}

	resp := s.Rename(1, uri, lsp.Position{Line: 0, Character: 22}, "purchases")
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	// Find schema edit
	found := false
	for _, dc := range resp.Result.DocumentChanges {
		if dc.TextDocumentEditValue != nil && dc.TextDocumentEditValue.TextDocument.URI == "file:///project/models/schema.yml" {
			found = true
			if len(dc.TextDocumentEditValue.Edits) != 1 {
				t.Fatalf("expected 1 edit, got %d", len(dc.TextDocumentEditValue.Edits))
			}
			edit := dc.TextDocumentEditValue.Edits[0]
			if edit.Range.Start.Character != 10 || edit.Range.End.Character != 16 {
				t.Errorf("range: got %d-%d, want 10-16", edit.Range.Start.Character, edit.Range.End.Character)
			}
			if edit.NewText != "purchases" {
				t.Errorf("newText: got %q, want %q", edit.NewText, "purchases")
			}
		}
	}
	if !found {
		t.Error("expected schema TextDocumentEdit")
	}
}

func TestRenameModelNoSchemaWhenEmpty(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('orders') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {URI: "/project/models/orders.sql", SchemaURI: ""},
	}

	resp := s.Rename(1, uri, lsp.Position{Line: 0, Character: 22}, "purchases")
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	for _, dc := range resp.Result.DocumentChanges {
		if dc.TextDocumentEditValue != nil {
			t.Error("expected no TextDocumentEdit when SchemaURI is empty")
		}
	}
}

func TestRenameModelPreservesDir(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('orders') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {URI: "/project/models/staging/orders.sql"},
	}

	resp := s.Rename(1, uri, lsp.Position{Line: 0, Character: 22}, "purchases")
	for _, dc := range resp.Result.DocumentChanges {
		if dc.RenameFileValue != nil {
			if dc.RenameFileValue.NewURI != "file:///project/models/staging/purchases.sql" {
				t.Errorf("newURI: got %q, want file:///project/models/staging/purchases.sql", dc.RenameFileValue.NewURI)
			}
		}
	}
}

func TestRenameColumnSchemaEdit(t *testing.T) {
	uri := "file:///models/schema.yml"
	s := NewState()
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {
			SchemaURI: "/models/schema.yml",
			SchemaRange: lsp.Range{
				Start: lsp.Position{Line: 3, Character: 8},
				End:   lsp.Position{Line: 3, Character: 8},
			},
			Columns: []Column{
				{Name: "order_id", Position: lsp.Position{Line: 8, Character: 12}},
			},
		},
	}

	resp := s.Rename(1, uri, lsp.Position{Line: 8, Character: 14}, "purchase_id")
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	if resp.Result == nil || len(resp.Result.DocumentChanges) != 1 {
		t.Fatalf("expected exactly 1 documentChange, got %v", resp.Result)
	}
	dc := resp.Result.DocumentChanges[0]
	if dc.TextDocumentEditValue == nil {
		t.Fatal("expected TextDocumentEdit, not RenameFile")
	}
	if dc.RenameFileValue != nil {
		t.Error("expected no RenameFile for column rename")
	}
	edit := dc.TextDocumentEditValue.Edits[0]
	if edit.Range.Start.Character != 12 || edit.Range.End.Character != 20 {
		t.Errorf("range: got %d-%d, want 12-20", edit.Range.Start.Character, edit.Range.End.Character)
	}
	if edit.NewText != "purchase_id" {
		t.Errorf("newText: got %q", edit.NewText)
	}
}

func TestRenameAllURIsHavePrefix(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('orders') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {
			URI:       "/project/models/orders.sql",
			SchemaURI: "/project/models/schema.yml",
			SchemaRange: lsp.Range{
				Start: lsp.Position{Line: 5, Character: 10},
				End:   lsp.Position{Line: 5, Character: 10},
			},
		},
	}

	resp := s.Rename(1, uri, lsp.Position{Line: 0, Character: 22}, "purchases")
	for _, dc := range resp.Result.DocumentChanges {
		if dc.RenameFileValue != nil {
			if !strings.HasPrefix(dc.RenameFileValue.OldURI, "file://") {
				t.Errorf("OldURI missing file:// prefix: %s", dc.RenameFileValue.OldURI)
			}
			if !strings.HasPrefix(dc.RenameFileValue.NewURI, "file://") {
				t.Errorf("NewURI missing file:// prefix: %s", dc.RenameFileValue.NewURI)
			}
		}
		if dc.TextDocumentEditValue != nil {
			if !strings.HasPrefix(dc.TextDocumentEditValue.TextDocument.URI, "file://") {
				t.Errorf("TextDocument.URI missing file:// prefix: %s", dc.TextDocumentEditValue.TextDocument.URI)
			}
		}
	}
}

func TestRenameSerializesSuccess(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('orders') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {URI: "/project/models/orders.sql"},
	}

	resp := s.Rename(1, uri, lsp.Position{Line: 0, Character: 22}, "purchases")
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	str := string(data)
	if !strings.Contains(str, `"result"`) {
		t.Error("expected result in JSON")
	}
	if !strings.Contains(str, `"documentChanges"`) {
		t.Errorf("expected documentChanges in JSON: %s", str)
	}
}

func TestRenameSerializesError(t *testing.T) {
	uri := "file:///test.sql"
	sql := "select * from {{ ref('orders') }}"
	s := newRenameState(uri, sql)
	s.DbtContext.ModelDetailMap = map[string]ModelDetails{
		"orders": {URI: "/project/models/orders.sql"},
	}

	resp := s.Rename(1, uri, lsp.Position{Line: 0, Character: 22}, "bad name")
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	str := string(data)
	if !strings.Contains(str, `"error"`) {
		t.Error("expected error in JSON")
	}
	if !strings.Contains(str, `"code":-32602`) {
		t.Errorf("expected code -32602 in JSON: %s", str)
	}
}
