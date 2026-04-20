package diagnostics

import (
	"fmt"
	"strings"

	"github.com/j-clemons/dbt-language-server/analysis"
	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/lsp"
)

func CheckColumns(doc analysis.Document, ctx analysis.DbtContext) []lsp.Diagnostic {
	if doc.Scope == nil {
		return nil
	}

	modelCols := buildCheckerModelColumns(ctx)
	allCols := parser.ResolveColumnsAtCursor(doc.Scope, modelCols, "")
	if len(allCols) == 0 {
		return nil
	}

	hasUnknownSource := hasUnresolvableSource(doc.Scope, modelCols)

	colSet := make(map[string]bool, len(allCols))
	for _, c := range allCols {
		colSet[strings.ToLower(c.Name)] = true
	}

	var diags []lsp.Diagnostic
	for _, item := range doc.Scope.SelectItems {
		if item.IsStar || item.Expression == "" {
			continue
		}
		colName := strings.ToLower(item.Expression)

		if item.Source != "" {
			srcCols := parser.ResolveColumnsAtCursor(doc.Scope, modelCols, item.Source)
			if len(srcCols) == 0 {
				continue
			}
			found := false
			for _, c := range srcCols {
				if strings.ToLower(c.Name) == colName {
					found = true
					break
				}
			}
			if !found {
				diags = append(diags, lsp.Diagnostic{
					Range:    tokenRange(item.Token),
					Message:  fmt.Sprintf("Column %q not found in %q", item.Expression, item.Source),
					Severity: 2,
					Source:   "dbt-ls",
				})
			}
		} else {
			if hasUnknownSource {
				continue
			}
			if !colSet[colName] {
				diags = append(diags, lsp.Diagnostic{
					Range:    tokenRange(item.Token),
					Message:  fmt.Sprintf("Column %q not found in any source", item.Expression),
					Severity: 2,
					Source:   "dbt-ls",
				})
			}
		}
	}
	return diags
}

func buildCheckerModelColumns(ctx analysis.DbtContext) parser.ModelColumns {
	mc := make(parser.ModelColumns, len(ctx.ModelDetailMap))
	for name, model := range ctx.ModelDetailMap {
		cols := make([]parser.ColumnInfo, len(model.Columns))
		for i, c := range model.Columns {
			cols[i] = parser.ColumnInfo{Name: c.Name, Description: c.Description}
		}
		mc[name] = cols
	}
	return mc
}

func hasUnresolvableSource(scope *parser.QueryScope, modelCols parser.ModelColumns) bool {
	for _, src := range scope.Sources {
		switch src.Kind {
		case parser.SourceKindTable, parser.SourceKindSource:
			return true
		case parser.SourceKindRef:
			if cols, ok := modelCols[src.Name]; !ok || len(cols) == 0 {
				return true
			}
		}
	}
	return false
}
