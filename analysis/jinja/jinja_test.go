package jinja

import (
	"os"
	"testing"
)

func TestResolveEnvVars(t *testing.T) {
	os.Setenv("TEST_DBT_PROFILE", "my_profile")
	defer os.Unsetenv("TEST_DBT_PROFILE")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "double quoted env var",
			input:    `profile: "{{ env_var("TEST_DBT_PROFILE") }}"`,
			expected: `profile: "my_profile"`,
		},
		{
			name:     "single quoted env var",
			input:    `profile: "{{ env_var('TEST_DBT_PROFILE') }}"`,
			expected: `profile: "my_profile"`,
		},
		{
			name:     "env var with extra spaces",
			input:    `profile: "{{  env_var( "TEST_DBT_PROFILE" )  }}"`,
			expected: `profile: "my_profile"`,
		},
		{
			name:     "env var with default value uses env",
			input:    `profile: "{{ env_var("TEST_DBT_PROFILE", "fallback") }}"`,
			expected: `profile: "my_profile"`,
		},
		{
			name:     "missing env var with default",
			input:    `profile: "{{ env_var("NONEXISTENT_VAR_12345", "fallback") }}"`,
			expected: `profile: "fallback"`,
		},
		{
			name:     "missing env var with empty default",
			input:    `profile: "{{ env_var("NONEXISTENT_VAR_12345", "") }}"`,
			expected: `profile: ""`,
		},
		{
			name:     "missing env var without default unchanged",
			input:    `profile: "{{ env_var("NONEXISTENT_VAR_12345") }}"`,
			expected: `profile: "{{ env_var("NONEXISTENT_VAR_12345") }}"`,
		},
		{
			name:     "no env var expression",
			input:    `profile: "my_profile"`,
			expected: `profile: "my_profile"`,
		},
		{
			name:     "multiple env vars",
			input:    `a: "{{ env_var("TEST_DBT_PROFILE") }}" b: "{{ env_var("TEST_DBT_PROFILE") }}"`,
			expected: `a: "my_profile" b: "my_profile"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ResolveEnvVars(tc.input)
			if result != tc.expected {
				t.Errorf("got %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestReplaceDocBlocks(t *testing.T) {
	docsContent := map[string]string{
		"my-doc": "This is the doc content",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "replace doc block",
			input:    `{{ doc("my-doc") }}`,
			expected: "This is the doc content",
		},
		{
			name:     "no doc blocks",
			input:    "plain description",
			expected: "plain description",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ReplaceDocBlocks(tc.input, docsContent)
			if result != tc.expected {
				t.Errorf("got %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestExtractDocsBlocks(t *testing.T) {
	input := `{% docs my_doc %}
This is documentation content.
{% enddocs %}

{% docs another_doc %}
More content here.
{% enddocs %}`

	result := ExtractDocsBlocks(input)

	if content, ok := result["my_doc"]; !ok {
		t.Error("expected 'my_doc' in result")
	} else if content != "This is documentation content." {
		t.Errorf("my_doc content = %q, want %q", content, "This is documentation content.")
	}

	if _, ok := result["another_doc"]; !ok {
		t.Error("expected 'another_doc' in result")
	}
}
