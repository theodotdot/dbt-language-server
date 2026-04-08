package diagnostics

import (
	"github.com/j-clemons/dbt-language-server/analysis"
	"github.com/j-clemons/dbt-language-server/lsp"
)

// CheckColumns detects references to columns not found in the schema of
// referenced models. Blocked on the column lineage engine from split 01.
// When unblocked, this will:
//  1. Use the lineage engine to map column references to originating models
//  2. Check each resolved column against ModelDetailMap[model].Columns
//  3. Emit Warning diagnostics for missing columns
func CheckColumns(_ analysis.Document, _ analysis.DbtContext) []lsp.Diagnostic {
	return nil
}
