package parser

// ColumnInfo represents a column from a model or source schema.
type ColumnInfo struct {
	Name        string
	Description string
}

// ModelColumns maps model name to its column definitions.
type ModelColumns map[string][]ColumnInfo

// ResolveColumnsAtCursor returns all columns available at the given cursor
// position, filtered by aliasPrefix if non-empty.
func ResolveColumnsAtCursor(
	scope *QueryScope,
	modelCols ModelColumns,
	aliasPrefix string,
) []ScopeColumn {
	if scope == nil {
		return nil
	}

	if aliasPrefix != "" {
		src := scope.FindSourceByAlias(aliasPrefix)
		if src == nil {
			return nil
		}
		return resolveSourceColumns(scope, src, modelCols)
	}

	var cols []ScopeColumn
	for _, src := range scope.Sources {
		cols = append(cols, resolveSourceColumns(scope, src, modelCols)...)
	}
	return cols
}

func resolveSourceColumns(scope *QueryScope, src *SourceRef, modelCols ModelColumns) []ScopeColumn {
	sourceLabel := src.Alias
	if sourceLabel == "" {
		sourceLabel = src.Name
	}

	switch src.Kind {
	case SourceKindRef:
		return buildScopeColumns(modelCols[src.Name], sourceLabel)
	case SourceKindCTE:
		return resolveCTEColumns(scope, src.Name, modelCols, make(map[string]bool))
	case SourceKindSource, SourceKindTable:
		return nil
	}
	return nil
}

func resolveCTEColumns(
	scope *QueryScope,
	cteName string,
	modelCols ModelColumns,
	visited map[string]bool,
) []ScopeColumn {
	if visited[cteName] {
		return nil
	}
	visited[cteName] = true

	cteDef, ok := scope.CTEs[cteName]
	if !ok {
		return nil
	}

	sourceLabel := cteName
	var cols []ScopeColumn

	for _, col := range cteDef.Columns {
		if col == "*" {
			// Expand star from CTE's internal scope
			if cteDef.Scope != nil {
				for _, innerSrc := range cteDef.Scope.Sources {
					switch innerSrc.Kind {
					case SourceKindRef:
						for _, ci := range modelCols[innerSrc.Name] {
							cols = append(cols, ScopeColumn{
								Name:        ci.Name,
								Source:      sourceLabel,
								Qualified:   sourceLabel + "." + ci.Name,
								Description: ci.Description,
							})
						}
					case SourceKindCTE:
						expanded := resolveCTEColumns(scope, innerSrc.Name, modelCols, visited)
						for i := range expanded {
							expanded[i].Source = sourceLabel
							expanded[i].Qualified = sourceLabel + "." + expanded[i].Name
						}
						cols = append(cols, expanded...)
					}
				}
			}
		} else {
			cols = append(cols, ScopeColumn{
				Name:      col,
				Source:    sourceLabel,
				Qualified: sourceLabel + "." + col,
			})
		}
	}
	return cols
}

func buildScopeColumns(infos []ColumnInfo, sourceLabel string) []ScopeColumn {
	if len(infos) == 0 {
		return nil
	}
	cols := make([]ScopeColumn, len(infos))
	for i, ci := range infos {
		cols[i] = ScopeColumn{
			Name:        ci.Name,
			Source:      sourceLabel,
			Qualified:   sourceLabel + "." + ci.Name,
			Description: ci.Description,
		}
	}
	return cols
}
