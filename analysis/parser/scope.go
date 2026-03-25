package parser

type ClauseKind int

const (
	ClauseNone ClauseKind = iota
	ClauseWith
	ClauseSelect
	ClauseFrom
	ClauseJoin
	ClauseWhere
	ClauseGroupBy
	ClauseOrderBy
	ClauseHaving
	ClauseOn
)

type SourceKind int

const (
	SourceKindTable SourceKind = iota
	SourceKindRef
	SourceKindSource
	SourceKindCTE
)

type SourceRef struct {
	Kind       SourceKind
	Name       string // model name, source table name, CTE name, or raw table
	SourceName string // only for SourceKindSource: first arg of source('src', 'table')
	Alias      string
	Token      Token
}

type CTEDef struct {
	Name    string
	Scope   *QueryScope
	Columns []string // resolved output column names; "*" for unresolved stars
	Token   Token
}

type SelectItem struct {
	Expression string
	Alias      string
	Source     string // source table/alias if resolvable (e.g., "o" from "o.id")
	IsStar     bool
	StarSource string // table alias/name for table.* (empty for bare *)
	Token      Token
}

type ScopeColumn struct {
	Name        string
	Source      string
	Qualified   string // "alias.column" form
	Description string
}

type QueryScope struct {
	Sources      []*SourceRef
	CTEs         map[string]*CTEDef
	SelectItems  []SelectItem
	ClauseRanges map[ClauseKind]Token
	Parent       *QueryScope
}

func NewQueryScope(parent *QueryScope) *QueryScope {
	return &QueryScope{
		CTEs:         make(map[string]*CTEDef),
		ClauseRanges: make(map[ClauseKind]Token),
		Parent:       parent,
	}
}

func (qs *QueryScope) FindSourceByAlias(alias string) *SourceRef {
	for _, s := range qs.Sources {
		if s.Alias == alias {
			return s
		}
	}
	for _, s := range qs.Sources {
		if s.Name == alias {
			return s
		}
	}
	return nil
}
