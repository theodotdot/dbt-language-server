package jinja

import (
	"log"
	"os"
	"regexp"
	"strings"
)

var (
	envVarRegex    = regexp.MustCompile(`\{\{\s*env_var\(\s*('|")([^'"]+)('|")\s*(?:,\s*('|")([^'"]*?)('|"))?\s*\)\s*\}\}`)
	docBlockRegex  = regexp.MustCompile(`\{\{\s*doc\(('|")([-\w]+)('|")\)\s*\}\}`)
	docsBlockRegex = regexp.MustCompile(`(?s){%-?\s*docs\s+([\w]+)\s*-?%}(.*?){%-?\s*enddocs\s*-?%}`)

	// MacroDefRegex matches {% macro name(args) %} definitions
	MacroDefRegex = regexp.MustCompile(`(?s){%-?\s*macro\s+(\w+)\s*\((.*?)\)\s*-?%}`)
)

func ResolveEnvVars(input string) string {
	return envVarRegex.ReplaceAllStringFunc(input, func(match string) string {
		submatches := envVarRegex.FindStringSubmatch(match)
		varName := submatches[2]
		defaultValue := submatches[5]

		value, exists := os.LookupEnv(varName)
		if exists {
			return value
		}

		if submatches[4] != "" {
			return defaultValue
		}

		log.Printf("env var %q not set and no default provided", varName)
		return match
	})
}

func ReplaceDocBlocks(description string, docsContent map[string]string) string {
	matches := docBlockRegex.FindAllStringSubmatchIndex(description, -1)
	if len(matches) == 0 {
		return description
	}

	result := description
	for i := 0; i < len(matches); i++ {
		docName := description[matches[i][4]:matches[i][5]]
		if content, ok := docsContent[docName]; ok {
			result = result[:matches[i][0]] + content + result[matches[i][1]:]
		}
	}
	return result
}

func ExtractDocsBlocks(content string) map[string]string {
	result := make(map[string]string)
	matches := docsBlockRegex.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		result[m[1]] = strings.TrimSpace(m[2])
	}
	return result
}
