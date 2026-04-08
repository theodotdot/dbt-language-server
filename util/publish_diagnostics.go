package util

import (
	"io"

	"github.com/j-clemons/dbt-language-server/lsp"
)

func PublishDiagnostics(writer io.Writer, uri string, diagnostics []lsp.Diagnostic) {
	if diagnostics == nil {
		diagnostics = []lsp.Diagnostic{}
	}
	notification := lsp.DiagnosticsNotification{
		Notification: lsp.Notification{
			RPC:    "2.0",
			Method: "textDocument/publishDiagnostics",
		},
		Params: lsp.PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: diagnostics,
		},
	}

	WriteResponse(writer, notification)
}
