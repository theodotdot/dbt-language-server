package lsp

type SemanticTokensRequest struct {
	Request
	Params SemanticTokensParams `json:"params"`
}

type SemanticTokensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type SemanticTokensResponse struct {
	Response
	Result SemanticTokensResult `json:"result"`
}

type SemanticTokensResult struct {
	Data []int `json:"data"`
}

type SemanticTokensLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

type SemanticTokensOptions struct {
	Legend SemanticTokensLegend `json:"legend"`
	Full   bool                 `json:"full"`
}
