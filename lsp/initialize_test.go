package lsp

import (
	"encoding/json"
	"testing"
)

func TestInitializeResponseIncludesRenameProvider(t *testing.T) {
	resp := NewInitializeResponse(1)

	if resp.Result.Capabilities.RenameProvider == nil {
		t.Fatal("RenameProvider is nil")
	}
	if !resp.Result.Capabilities.RenameProvider.PrepareProvider {
		t.Error("PrepareProvider should be true")
	}

	// Verify JSON serialization
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err = json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	result := raw["result"].(map[string]any)
	caps := result["capabilities"].(map[string]any)
	rp, ok := caps["renameProvider"].(map[string]any)
	if !ok {
		t.Fatal("renameProvider not in JSON output")
	}
	if pp, _ := rp["prepareProvider"].(bool); !pp {
		t.Error("prepareProvider not true in JSON")
	}
}
