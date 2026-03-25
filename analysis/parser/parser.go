package parser

import (
	"errors"
	"sort"

	"github.com/j-clemons/dbt-language-server/docs"
)

type Parser struct {
	l           *Lexer
	curTok      Token
	peekTok     Token
	tokens      []TokenLL
	ctes        CTE
	setVars     map[string]Token
	scopeStack  []*QueryScope
	clauseStack []ClauseKind
	parenDepth  int
	// select item accumulator
	selectTokens   []Token
	selectHasAS    bool
	selectStarted  bool
	// FROM/JOIN source tracking
	lastSourceRef  *SourceRef
}

type CTE struct {
	Ind          bool
	ParenCount   int
	Tokens       []Token
	TokenNameMap map[string]Token
}

type TokenLL struct {
	Token     Token
	PrevToken *TokenLL
}

func NewParser(input string, dialect docs.Dialect) *Parser {
	topScope := NewQueryScope(nil)
	return &Parser{
		l: New(input, dialect),
		ctes: CTE{
			Ind:        false,
			ParenCount: -1,
			Tokens:     []Token{},
		},
		setVars:     make(map[string]Token),
		scopeStack:  []*QueryScope{topScope},
		clauseStack: []ClauseKind{ClauseNone},
	}
}

func Parse(input string, dialect docs.Dialect) *Parser {
	p := NewParser(input, dialect)
	p.parseTokens()
	return p
}

func (p *Parser) NextToken() Token {
	if p.curTok.Type != "" {
		prevToken := (*TokenLL)(nil)
		if len(p.tokens) > 0 {
			prevToken = &p.tokens[len(p.tokens)-1]
		}
		p.tokens = append(p.tokens, TokenLL{
			Token:     p.curTok,
			PrevToken: prevToken,
		})
	}

	p.curTok = p.peekTok
	p.peekTok = p.l.NextToken()
	return p.curTok
}

func (p *Parser) parseWith() {
	p.NextToken()
	if p.curTok.Type == IDENT {
		p.ctes.Ind = true
		p.ctes.Tokens = append(p.ctes.Tokens, p.curTok)
		if p.peekTok.Type == AS {
			p.NextToken()
		}
		if p.peekTok.Type == LPAREN {
			p.ctes.ParenCount = 1
		}
		p.NextToken()
	}
}

func (p *Parser) parseRef() {
	p.NextToken()
	if p.curTok.Type == LPAREN {
		p.incParenCount()
		p.NextToken()
		if p.curTok.Type == SINGLE_QUOTE || p.curTok.Type == DOUBLE_QUOTE {
			p.NextToken()
			if p.curTok.Type == IDENT {
				p.curTok.Type = REF
			}
		}
	}
}

func (p *Parser) parseVar() {
	p.NextToken()
	if p.curTok.Type == LPAREN {
		p.incParenCount()
		p.NextToken()
		if p.curTok.Type == SINGLE_QUOTE || p.curTok.Type == DOUBLE_QUOTE {
			p.NextToken()
			if p.curTok.Type == IDENT {
				p.curTok.Type = VAR
			}
		}
	}
}

func (p *Parser) parseMacro() {
	if p.peekTok.Type == DOT {
		p.curTok.Type = PACKAGE
		p.NextToken()
		p.NextToken()
	}
	if p.curTok.Type == IDENT && p.peekTok.Type == LPAREN {
		p.curTok.Type = MACRO
	}
}

func (p *Parser) parseSource() {
	p.NextToken()
	if p.curTok.Type == LPAREN {
		p.incParenCount()
		p.NextToken()
		if p.curTok.Type == SINGLE_QUOTE || p.curTok.Type == DOUBLE_QUOTE {
			p.NextToken()
			if p.curTok.Type == IDENT {
				p.curTok.Type = SOURCE
				p.NextToken()
				if p.curTok.Type == SINGLE_QUOTE || p.curTok.Type == DOUBLE_QUOTE {
					p.NextToken()
					if p.curTok.Type == COMMA {
						p.NextToken()
						if p.curTok.Type == SINGLE_QUOTE || p.curTok.Type == DOUBLE_QUOTE {
							p.NextToken()
							if p.curTok.Type == IDENT {
								p.curTok.Type = SOURCE_TABLE
							}
						}
					}
				}
			}
		}
	}
}

func (p *Parser) parseConfig() {
	p.NextToken()
	if p.curTok.Type == LPAREN {
		p.incParenCount()
		// Parse config parameters - we'll mark the config function call
		// and let the rest of the parsing handle the parameters normally
	}
}

func (p *Parser) parseJinjaBlock() {
	p.NextToken()
	switch p.curTok.Type {
	case SET:
		p.NextToken()
		if p.curTok.Type == IDENT {
			p.curTok.Type = JINJA_SET
			p.setVars[p.curTok.Literal] = p.curTok
		}
		for p.curTok.Type != JINJA_RBRACE && p.curTok.Type != EOF {
			switch p.curTok.Type {
			case REF:
				p.parseRef()
			case VAR:
				p.parseVar()
			case SOURCE:
				p.parseSource()
			}
			p.NextToken()
		}
	default:
		for p.curTok.Type != JINJA_RBRACE && p.curTok.Type != EOF {
			p.NextToken()
		}
	}
}

func (p *Parser) incParenCount() {
	if p.ctes.Ind {
		p.ctes.ParenCount++
	}
}

func (p *Parser) decParenCount() {
	if p.ctes.Ind {
		p.ctes.ParenCount--
	}
}

func (p *Parser) currentScope() *QueryScope {
	return p.scopeStack[len(p.scopeStack)-1]
}

func (p *Parser) currentClause() ClauseKind {
	return p.clauseStack[len(p.clauseStack)-1]
}

func (p *Parser) setClause(kind ClauseKind) {
	if p.currentClause() == ClauseSelect {
		p.finalizeSelectItem()
	}
	p.lastSourceRef = nil
	p.clauseStack[len(p.clauseStack)-1] = kind
	p.currentScope().ClauseRanges[kind] = p.curTok
}

func (p *Parser) CreateQueryScope() *QueryScope {
	if len(p.scopeStack) == 0 {
		return NewQueryScope(nil)
	}
	return p.scopeStack[0]
}

func (p *Parser) resetSelectAccumulator() {
	p.selectTokens = p.selectTokens[:0]
	p.selectHasAS = false
}

func (p *Parser) finalizeSelectItem() {
	tokens := p.selectTokens
	if len(tokens) == 0 {
		p.resetSelectAccumulator()
		return
	}

	item := SelectItem{Token: tokens[0]}

	if p.selectHasAS {
		// Last token is the alias after AS
		last := tokens[len(tokens)-1]
		if last.Type == IDENT {
			item.Alias = last.Literal
		}
	} else if len(tokens) == 1 && tokens[0].Type == IDENT {
		// Single identifier: both expression and alias
		item.Alias = tokens[0].Literal
	} else if len(tokens) == 3 && tokens[0].Type == IDENT && tokens[1].Type == DOT && tokens[2].Type == IDENT {
		// Qualified column: qualifier.column
		item.Source = tokens[0].Literal
		item.Alias = tokens[2].Literal
	} else if len(tokens) >= 2 {
		// Implicit alias: last IDENT at depth 0
		last := tokens[len(tokens)-1]
		if last.Type == IDENT {
			// Check it's not preceded by DOT (which would make it a qualified ref)
			if tokens[len(tokens)-2].Type != DOT {
				item.Alias = last.Literal
			}
		}
	}

	p.currentScope().SelectItems = append(p.currentScope().SelectItems, item)
	p.resetSelectAccumulator()
}

func (p *Parser) collectSelectToken() {
	if p.parenDepth > 0 {
		// Inside function call or subquery — don't collect
		return
	}

	switch p.curTok.Type {
	case ASTERISK:
		// Bare * or qualified table.* — only a star if no prior expression tokens
		// or the previous token is DOT (table.*)
		isQualifiedStar := len(p.selectTokens) >= 2 &&
			p.selectTokens[len(p.selectTokens)-1].Type == DOT &&
			p.selectTokens[len(p.selectTokens)-2].Type == IDENT
		isBareStar := len(p.selectTokens) == 0

		if isBareStar || isQualifiedStar {
			starSource := ""
			if isQualifiedStar {
				starSource = p.selectTokens[len(p.selectTokens)-2].Literal
			}
			item := SelectItem{
				IsStar:     true,
				StarSource: starSource,
				Token:      p.curTok,
			}
			p.currentScope().SelectItems = append(p.currentScope().SelectItems, item)
			p.resetSelectAccumulator()
		} else {
			// Multiplication operator — accumulate as part of expression
			p.selectTokens = append(p.selectTokens, p.curTok)
		}

	case COMMA:
		p.finalizeSelectItem()

	case AS:
		p.selectHasAS = true

	case DISTINCT:
		if !p.selectStarted {
			// Skip DISTINCT right after SELECT
			return
		}
		p.selectTokens = append(p.selectTokens, p.curTok)

	default:
		p.selectTokens = append(p.selectTokens, p.curTok)
	}
	p.selectStarted = true
}

func (p *Parser) inFromJoinContext() bool {
	clause := p.currentClause()
	return clause == ClauseFrom || clause == ClauseJoin
}

func (p *Parser) isSourceAliasKeyword(tt TokenType) bool {
	switch tt {
	case ON, JOIN, WHERE, GROUP, ORDER, HAVING, LIMIT, UNION, SEMICOLON,
		LEFT, RIGHT, INNER, CROSS, FULL, NATURAL, SELECT, EOF,
		COMMA, RPAREN, JINJA_LBRACE, DB_LBRACE:
		return true
	}
	return false
}

func (p *Parser) addSourceRef(kind SourceKind, name, sourceName string, tok Token) {
	ref := &SourceRef{
		Kind:       kind,
		Name:       name,
		SourceName: sourceName,
		Token:      tok,
	}
	p.lastSourceRef = ref
	p.currentScope().Sources = append(p.currentScope().Sources, ref)
}

func (p *Parser) parseTokens() {
	for p.curTok.Type != EOF {
		switch p.curTok.Type {
		case WITH:
			if p.parenDepth == 0 {
				p.setClause(ClauseWith)
			}
			p.parseWith()
		case SELECT:
			if p.parenDepth == 0 {
				p.setClause(ClauseSelect)
				p.selectStarted = false
				p.resetSelectAccumulator()
			}
		case FROM:
			if p.parenDepth == 0 {
				p.setClause(ClauseFrom)
			}
		case JOIN:
			if p.parenDepth == 0 {
				p.setClause(ClauseJoin)
			}
		case WHERE:
			if p.parenDepth == 0 {
				p.setClause(ClauseWhere)
			}
		case GROUP:
			if p.parenDepth == 0 && p.peekTok.Type == BY {
				p.setClause(ClauseGroupBy)
			}
		case ORDER:
			if p.parenDepth == 0 && p.peekTok.Type == BY {
				p.setClause(ClauseOrderBy)
			}
		case HAVING:
			if p.parenDepth == 0 {
				p.setClause(ClauseHaving)
			}
		case ON:
			if p.parenDepth == 0 {
				p.setClause(ClauseOn)
			}
		case LPAREN:
			p.parenDepth++
			p.incParenCount()
		case RPAREN:
			p.parenDepth--
			if p.parenDepth < 0 {
				p.parenDepth = 0
			}
			p.decParenCount()
			if p.ctes.Ind && p.ctes.ParenCount == 0 {
				p.NextToken()
				if p.curTok.Type == COMMA {
					p.NextToken()
					if p.curTok.Type == IDENT {
						p.ctes.Tokens = append(p.ctes.Tokens, p.curTok)
					}
				} else {
					p.ctes.Ind = false
				}
			}
		case SOURCE:
			p.parseSource()
			if p.inFromJoinContext() && p.parenDepth == 0 {
				// After parseSource(), curTok is SOURCE_TABLE, walk tokens for SOURCE name
				if p.curTok.Type == SOURCE_TABLE {
					tblName := p.curTok.Literal
					srcTok := p.curTok
					var srcName string
					for i := len(p.tokens) - 1; i >= 0; i-- {
						if p.tokens[i].Token.Type == SOURCE {
							srcName = p.tokens[i].Token.Literal
							break
						}
					}
					p.addSourceRef(SourceKindSource, tblName, srcName, srcTok)
				}
			}
		case REF:
			p.parseRef()
			if p.inFromJoinContext() && p.parenDepth == 0 {
				if p.curTok.Type == REF {
					p.addSourceRef(SourceKindRef, p.curTok.Literal, "", p.curTok)
				}
			}
		case VAR:
			p.parseVar()
		case DB_LBRACE:
			switch p.peekTok.Type {
			case CONFIG:
				p.NextToken()
				p.parseConfig()
			case IDENT:
				p.NextToken()
				p.parseMacro()
			}
		case JINJA_LBRACE:
			p.parseJinjaBlock()
		case DB_RBRACE:
		case JINJA_RBRACE:
		default:
			if p.inFromJoinContext() && p.parenDepth == 0 && p.curTok.Type == COMMA {
				p.lastSourceRef = nil
			} else if p.currentClause() == ClauseSelect {
				p.collectSelectToken()
			} else if p.inFromJoinContext() && p.parenDepth == 0 && p.curTok.Type == IDENT {
				if p.lastSourceRef != nil && p.lastSourceRef.Alias == "" && !p.isSourceAliasKeyword(p.curTok.Type) {
					// This IDENT is an alias for the last source
					p.lastSourceRef.Alias = p.curTok.Literal
				} else {
					kind := SourceKindTable
					if _, ok := p.currentScope().CTEs[p.curTok.Literal]; ok {
						kind = SourceKindCTE
					}
					p.addSourceRef(kind, p.curTok.Literal, "", p.curTok)
				}
			} else if p.inFromJoinContext() && p.parenDepth == 0 && p.curTok.Type == AS {
				// Explicit AS alias — next IDENT is the alias
				if p.lastSourceRef != nil && p.peekTok.Type == IDENT {
					p.NextToken()
					p.lastSourceRef.Alias = p.curTok.Literal
				}
			}
		}
		p.NextToken()
	}
	// Finalize any pending select item at EOF
	if p.currentClause() == ClauseSelect {
		p.finalizeSelectItem()
	}
}

func (p *Parser) CreateTokenNameMap() map[string]Token {
	tokenMap := make(map[string]Token)
	for _, token := range p.ctes.Tokens {
		tokenMap[token.Literal] = token
	}
	for name, token := range p.setVars {
		tokenMap[name] = token
	}
	return tokenMap
}

type TokenIndex struct {
	lineTokens map[int][]TokenLL
}

func (p *Parser) CreateTokenIndex() *TokenIndex {
	index := &TokenIndex{
		lineTokens: make(map[int][]TokenLL),
	}

	for _, t := range p.tokens {
		index.lineTokens[t.Token.Line] = append(index.lineTokens[t.Token.Line], t)
	}

	return index
}

func (ti *TokenIndex) LineTokens() map[int][]TokenLL {
	return ti.lineTokens
}

func (ti *TokenIndex) FindTokenAtCursor(line, column int) (*TokenLL, error) {
	lineTokens, exists := ti.lineTokens[line]
	if !exists {
		return nil, errors.New("line does not exist")
	}

	// Binary search to find the token
	idx := sort.Search(len(lineTokens), func(i int) bool {
		return lineTokens[i].Token.Column+len(lineTokens[i].Token.Literal) > column
	})

	if idx >= 0 &&
		column >= lineTokens[idx].Token.Column &&
		column < lineTokens[idx].Token.Column+len(lineTokens[idx].Token.Literal) {
		return &lineTokens[idx], nil
	}

	return nil, errors.New("token does not exist")
}

func (tl *TokenLL) TokenLookbackMatch(tokenType TokenType, inc int) (bool, string) {
	if inc <= 0 {
		return false, ""
	}

	prevToken := tl.PrevToken
	for i := 1; i < inc; i++ {
		if prevToken == nil {
			return false, ""
		}
		prevToken = prevToken.PrevToken
	}

	if prevToken != nil && prevToken.Token.Type == tokenType {
		return true, prevToken.Token.Literal
	}
	return false, ""
}
