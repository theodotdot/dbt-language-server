package analysis

import "github.com/j-clemons/dbt-language-server/lsp"

// PrepareRename determines if the cursor is on a renameable symbol.
// Full implementation in section-04.
func (s *State) PrepareRename(id int, uri string, position lsp.Position) lsp.PrepareRenameResponse {
	return lsp.PrepareRenameResponse{
		Response: lsp.Response{RPC: "2.0", ID: &id},
	}
}

// Rename builds a WorkspaceEdit for renaming a symbol.
// Full implementation in section-06.
func (s *State) Rename(id int, uri string, position lsp.Position, newName string) lsp.RenameResponse {
	return lsp.RenameResponse{
		Response: lsp.Response{RPC: "2.0", ID: &id},
	}
}
