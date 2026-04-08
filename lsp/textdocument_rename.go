package lsp

import "encoding/json"

const ErrCodeInvalidParams = -32602

type PrepareRenameRequest struct {
	Request
	Params TextDocumentPositionParams `json:"params"`
}

type PrepareRenameResponse struct {
	Response
	Result *PrepareRenameResult `json:"result"`
}

type PrepareRenameResult struct {
	Range       Range  `json:"range"`
	Placeholder string `json:"placeholder"`
}

type RenameRequest struct {
	Request
	Params RenameParams `json:"params"`
}

type RenameParams struct {
	TextDocumentPositionParams
	NewName string `json:"newName"`
}

type RenameResponse struct {
	Response
	Result *WorkspaceEdit `json:"result,omitempty"`
	Error  *ResponseError `json:"error,omitempty"`
}

type DocumentChange struct {
	TextDocumentEditValue *TextDocumentEdit
	RenameFileValue       *RenameFile
}

func (d DocumentChange) MarshalJSON() ([]byte, error) {
	if d.RenameFileValue != nil {
		return json.Marshal(*d.RenameFileValue)
	}
	if d.TextDocumentEditValue != nil {
		return json.Marshal(*d.TextDocumentEditValue)
	}
	return json.Marshal(nil)
}

type TextDocumentEdit struct {
	TextDocument OptionalVersionedTextDocumentIdentifier `json:"textDocument"`
	Edits        []TextEdit                              `json:"edits"`
}

type OptionalVersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version *int   `json:"version"`
}

type RenameFile struct {
	Kind   string `json:"kind"`
	OldURI string `json:"oldUri"`
	NewURI string `json:"newUri"`
}

type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
