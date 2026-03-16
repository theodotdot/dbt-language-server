package analysis

import (
	"reflect"
	"testing"

	"github.com/j-clemons/dbt-language-server/lsp"
)

func TestGetMacrosFromFile(t *testing.T) {
	macroFileStr := `
{% macro example_macro(str) %}

{% endmacro %}

{% macro multiline_macro(
    str,
    int
) %}

{% endmacro %}
`
	testCases := []struct {
		name           string
		fileStr        string
		fileUri        string
		dbtProjectYaml DbtProjectYaml
		expected       []Macro
	}{
		{
			name:    "Example File",
			fileStr: macroFileStr,
			fileUri: "file:///path/to/file.sql",
			dbtProjectYaml: DbtProjectYaml{
				ProjectName: AnnotatedField[string]{Value: "example"},
				MacroPaths: AnnotatedField[[]string]{
					Value: []string{
						"macros",
					},
				},
			},
			expected: []Macro{
				{
					Name:        "example_macro",
					ProjectName: "example",
					Description: "example_macro(str)",
					Arguments:   []MacroArg{{Name: "str"}},
					URI:         "file:///path/to/file.sql",
					Range: lsp.Range{
						Start: lsp.Position{
							Line:      1,
							Character: 9,
						},
						End: lsp.Position{
							Line:      1,
							Character: 27,
						},
					},
				},
				{
					Name:        "multiline_macro",
					ProjectName: "example",
					Description: "multiline_macro(str, int)",
					Arguments:   []MacroArg{{Name: "str"}, {Name: "int"}},
					URI:         "file:///path/to/file.sql",
					Range: lsp.Range{
						Start: lsp.Position{
							Line:      5,
							Character: 9,
						},
						End: lsp.Position{
							Line:      8,
							Character: 1,
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getMacrosFromFile(tc.fileStr, tc.fileUri, tc.dbtProjectYaml)
			if !reflect.DeepEqual(result, tc.expected) {
				for i, e := range tc.expected {
					if i < len(result) && !reflect.DeepEqual(e, result[i]) {
						t.Errorf("macro[%d]:\n  got:  %+v\n  want: %+v", i, result[i], e)
					}
				}
				if len(result) != len(tc.expected) {
					t.Errorf("got %d macros, want %d", len(result), len(tc.expected))
				}
			}
		})
	}
}

func TestParseMacroArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []MacroArg
	}{
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
		{
			name:     "single arg",
			input:    "str",
			expected: []MacroArg{{Name: "str"}},
		},
		{
			name:     "multiple args",
			input:    "first_name, last_name",
			expected: []MacroArg{{Name: "first_name"}, {Name: "last_name"}},
		},
		{
			name:     "args with defaults",
			input:    "arg1, arg2='default_value'",
			expected: []MacroArg{{Name: "arg1"}, {Name: "arg2", Default: "'default_value'"}},
		},
		{
			name:     "all defaults",
			input:    "a=1, b='hello', c=True",
			expected: []MacroArg{{Name: "a", Default: "1"}, {Name: "b", Default: "'hello'"}, {Name: "c", Default: "True"}},
		},
		{
			name:  "multiline args",
			input: "\n    str,\n    int\n",
			expected: []MacroArg{{Name: "str"}, {Name: "int"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseMacroArgs(tc.input)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("parseMacroArgs(%q):\n  got:  %+v\n  want: %+v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestFormatMacroSignature(t *testing.T) {
	tests := []struct {
		name     string
		macName  string
		args     []MacroArg
		expected string
	}{
		{
			name:     "no args",
			macName:  "my_macro",
			args:     nil,
			expected: "my_macro()",
		},
		{
			name:     "simple args",
			macName:  "full_name",
			args:     []MacroArg{{Name: "first_name"}, {Name: "last_name"}},
			expected: "full_name(first_name, last_name)",
		},
		{
			name:     "args with defaults",
			macName:  "greet",
			args:     []MacroArg{{Name: "name"}, {Name: "greeting", Default: "'hello'"}},
			expected: "greet(name, greeting='hello')",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatMacroSignature(tc.macName, tc.args)
			if result != tc.expected {
				t.Errorf("formatMacroSignature(%q, %+v) = %q, want %q", tc.macName, tc.args, result, tc.expected)
			}
		})
	}
}
