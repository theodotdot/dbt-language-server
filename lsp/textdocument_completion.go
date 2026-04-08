package lsp

type CompletionRequest struct {
	Request
	Params CompletionParams `json:"params"`
}

type CompletionParams struct {
	TextDocumentPositionParams
}

type CompletionResponse struct {
	Response
	Result []CompletionItem `json:"result"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

type CompletionItem struct {
	Label            string    `json:"label"`
	Detail           string    `json:"detail"`
	Documentation    string    `json:"documentation"`
	Kind             int       `json:"kind"`
	InsertText       string    `json:"insertText"`
	InsertTextFormat int       `json:"insertTextFormat,omitempty"`
	SortText         string    `json:"sortText"`
	FilterText       string    `json:"filterText,omitempty"`
	TextEdit         *TextEdit `json:"textEdit,omitempty"`
}

type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters"`
}
